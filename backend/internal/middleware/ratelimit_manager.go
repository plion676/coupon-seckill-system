package middleware

import (
	"context"
	"fmt"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

type RateLimitManager struct {
	redisClient  *redis.Client
	globalConfig RateLimitConfig
	userConfig   RateLimitConfig
	scriptSHA    string
}

type RateLimitConfig struct {
	Capacity int64
	Rate     int64
}

const luaScript = `
	local key = KEYS[1]
	local numTokens = tonumber(ARGV[1])
	local now = tonumber(ARGV[2])
	local capacity = tonumber(ARGV[3])
	local rate = tonumber(ARGV[4])
	local expireSeconds = tonumber(ARGV[5])

	if not numTokens or numTokens <= 0 then return {-1, "invalid numTokens"} end
	if not now or now < 0 then return {-1, "invalid timestamp"} end

	local exists = redis.call('EXISTS', key)
	local current, lastRefill, bucketCapacity, bucketRate

	if exists == 0 then
		if not capacity or not rate then return {-2, "uninitialized"} end
		current = capacity
		lastRefill = now
		bucketCapacity = capacity
		bucketRate = rate
	else
		local data = redis.call('HMGET', key, 'current', 'last_refill', 'capacity', 'rate')
		current = tonumber(data[1])
		lastRefill = tonumber(data[2])
		bucketCapacity = tonumber(data[3])
		bucketRate = tonumber(data[4])

		if not current or not lastRefill or not bucketCapacity or not bucketRate then
			return {-1, "invalid bucket data"}
		end
	end

	local timeDiff = now - lastRefill
	if timeDiff < 0 then timeDiff = 0 end

	local newTokens = (timeDiff * bucketRate) / 1000
	local updatedCurrent = math.min(bucketCapacity, current + newTokens)

	if updatedCurrent >= numTokens then
		local resultCurrent = updatedCurrent - numTokens
		redis.call('HSET', key, 'current', resultCurrent, 'last_refill', now, 'capacity', bucketCapacity, 'rate', bucketRate)
		redis.call('EXPIRE', key, expireSeconds)
		return {1, resultCurrent}
	else
		redis.call('HSET', key, 'current', updatedCurrent, 'last_refill', now, 'capacity', bucketCapacity, 'rate', bucketRate)
		redis.call('EXPIRE', key, expireSeconds)
		return {0, updatedCurrent}
	end`

func NewRateLimitManager(redisClient *redis.Client) *RateLimitManager {
	sha, err := redisClient.ScriptLoad(context.Background(), luaScript).Result()
	if err != nil {
		panic(fmt.Sprintf("Failed to load Lua script: %v", err))
	}
	return &RateLimitManager{
		redisClient:  redisClient,
		globalConfig: RateLimitConfig{10000, 8000},
		userConfig:   RateLimitConfig{1, 1},
		scriptSHA:    sha,
	}
}

func (m *RateLimitManager) GlobalLimit() gin.HandlerFunc {
	return func(c *gin.Context) {
		couponID := c.Query("coupon_id")
		if couponID == "" {
			c.JSON(400, gin.H{
				"error": "缺少coupon_id参数"})
			c.Abort()
			return
		}

		key := fmt.Sprintf("rate_limit:coupon:%s:global", couponID)

		ok, _, err := m.tryConsume(key, 1, m.globalConfig.Capacity, m.globalConfig.Rate, 24*time.Hour)
		if err != nil {
			fmt.Printf("GlobalLimit Redis error: %v\n", err)
			c.Next()
			return
		}

		if !ok {
			c.JSON(429, gin.H{
				"error": "当前活动太火爆了，请稍后重试"})
			c.Abort()
			return
		}
		c.Next()
	}
}

func (m *RateLimitManager) UserLimit() gin.HandlerFunc {
	return func(c *gin.Context) {
		couponID := c.Query("coupon_id")
		userID := c.Query("user_id")
		if couponID == "" || userID == "" {
			c.JSON(400, gin.H{
				"error": "缺少coupon_id/user_id参数"})
			c.Abort()
			return
		}

		key := fmt.Sprintf("rate_limit:coupon:%s:user:%s", couponID, userID)
		ok, _, err := m.tryConsume(key, 1, m.userConfig.Capacity, m.userConfig.Rate, 1*time.Hour)
		if err != nil {
			// Redis 故障时采取 Fail-Open 策略，记录日志并放行
			fmt.Printf("UserLimit Redis error: %v\n", err)
			c.Next()
			return
		}

		if !ok {
			c.JSON(429, gin.H{
				"error": "操作太频繁了，请歇一会吧"})
			c.Abort()
			return
		}
		c.Next()
	}
}

func (m *RateLimitManager) tryConsume(key string, numTokens int64, capacity, rate int64, expiration time.Duration) (bool, int64, error) {
	now := time.Now().UnixMilli()
	expireSeconds := int64(expiration.Seconds())

	result, err := m.redisClient.EvalSha(
		context.Background(),
		m.scriptSHA,
		[]string{key},
		numTokens,
		now,
		capacity,
		rate,
		expireSeconds,
	).Result()
	if err != nil {
		return false, 0, fmt.Errorf("redis eval error: %v", err)
	}

	resultSlice, ok := result.([]interface{})
	if !ok {
		return false, 0, fmt.Errorf("invalid result type: expected []interface{}")
	}

	// 验证数组长度
	if len(resultSlice) != 2 {
		return false, 0, fmt.Errorf("invalid result length: expected 2, got %d", len(resultSlice))
	}

	// 解析状态码（第一个元素）
	statusCode, ok := resultSlice[0].(int64)
	if !ok {
		// 有时候 Redis 返回的数字可能是其他类型，尝试转换
		if floatVal, ok := resultSlice[0].(float64); ok {
			statusCode = int64(floatVal)
		} else {
			return false, 0, fmt.Errorf("invalid status code type")
		}
	}

	// 根据状态码处理
	switch statusCode {
	case 1:
		// 成功：解析剩余令牌数
		remaining, ok := resultSlice[1].(int64)
		if !ok {
			if floatVal, ok := resultSlice[1].(float64); ok {
				remaining = int64(floatVal)
			} else {
				return false, 0, fmt.Errorf("invalid remaining tokens type")
			}
		}
		return true, remaining, nil

	case 0:
		// 失败：解析当前令牌数
		current, ok := resultSlice[1].(int64)
		if !ok {
			if floatVal, ok := resultSlice[1].(float64); ok {
				current = int64(floatVal)
			} else {
				return false, 0, fmt.Errorf("invalid current tokens type")
			}
		}
		return false, current, nil

	case -1:
		// 错误：解析错误信息
		errorMsg, ok := resultSlice[1].(string)
		if !ok {
			return false, 0, fmt.Errorf("rate limit error: unknown error")
		}
		return false, 0, fmt.Errorf("rate limit error: %s", errorMsg)

	case -2:
		// 未初始化：解析错误信息
		errorMsg, ok := resultSlice[1].(string)
		if !ok {
			return false, 0, fmt.Errorf("bucket not initialized")
		}
		return false, 0, fmt.Errorf("bucket not initialized: %s", errorMsg)

	default:
		return false, 0, fmt.Errorf("unknown status code: %d", statusCode)
	}
}
