package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	rds "coupon-seckill-system/internal/infra/redis"

	"github.com/redis/go-redis/v9"
)

var seckillScript = redis.NewScript(`
local metaKey = KEYS[1]
local stockKey = KEYS[2]
local usersKey = KEYS[3]

local nowUnix = tonumber(ARGV[1])
local userId = ARGV[2]

local startUnix = redis.call("HGET", metaKey, "start_unix")
local endUnix = redis.call("HGET", metaKey, "end_unix")
if (not startUnix) or (not endUnix) then
	return 5
end

startUnix = tonumber(startUnix)
endUnix = tonumber(endUnix)
if (not startUnix) or (not endUnix) then
	return 5
end

if nowUnix < startUnix then
	return 3
end
if nowUnix > endUnix then
	return 4
end

if redis.call("SISMEMBER", usersKey, userId) == 1 then
	return 2
end

local stock = redis.call("GET", stockKey)
if not stock then
	return 5
end

stock = tonumber(stock)
if (not stock) or stock <= 0 then
	return 1
end

local newStock = redis.call("DECR", stockKey)
if newStock < 0 then
	redis.call("INCR", stockKey)
	return 1
end

redis.call("SADD", usersKey, userId)
return 0
`)

func EvalSeckill(couponID, userID int64) (int, error) {
	if rds.RDB == nil {
		return 5, ErrRedisUnavailable
	}

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	metaKey := fmt.Sprintf("coupon:meta:%d", couponID)
	stockKey := fmt.Sprintf("coupon:stock:%d", couponID)
	usersKey := fmt.Sprintf("coupon:users:%d", couponID)

	res, err := seckillScript.Run(ctx, rds.RDB, []string{metaKey, stockKey, usersKey}, time.Now().Unix(), userID).Result()
	if err != nil {
		return 5, err
	}

	switch v := res.(type) {
	case int64:
		return int(v), nil
	case string:
		return 5, errors.New("unexpected lua result type")
	default:
		return 5, errors.New("unexpected lua result type")
	}
}

