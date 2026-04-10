package middleware

import (
	"coupon-seckill-system/internal/pkg/ratelimit"
	"sync"

	"github.com/gin-gonic/gin"
)

type RateLimitManager struct {
	globalBucket *ratelimit.TokenBucket
	userBuckets  sync.Map
}

func NewRateLimitManager() *RateLimitManager {
	return &RateLimitManager{
		globalBucket: ratelimit.NewTokenBucket(10000, 8000),
	}
}

func (m *RateLimitManager) GlobalLimit() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !m.globalBucket.TryConsume(1) {
			c.JSON(429, gin.H{
				"error": "系统繁忙,请稍后重试"})
			c.Abort()
			return
		}
		c.Next()
	}
}

func (m *RateLimitManager) UserLimit() gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := c.Query("user_id")
		if userID == "" {
			c.JSON(400, gin.H{
				"error": "缺少user_id参数"})
			c.Abort()
			return
		}
		bucket := m.getUserBucket(userID)

		if !bucket.TryConsume(1) {
			c.JSON(429, gin.H{
				"error": "请求过于频繁,请稍后再试"})
			c.Abort()
			return
		}
		c.Next()
	}
}

func (m *RateLimitManager) getUserBucket(userID string) *ratelimit.TokenBucket {
	if bucket, ok := m.userBuckets.
		Load(userID); ok {
		return bucket.(*ratelimit.TokenBucket)
	}

	newBucket := ratelimit.
		NewTokenBucket(1, 1)
	actual, _ := m.userBuckets.
		LoadOrStore(userID, newBucket)
	return actual.(*ratelimit.TokenBucket)
}
