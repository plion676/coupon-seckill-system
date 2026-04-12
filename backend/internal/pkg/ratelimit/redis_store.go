package ratelimit

import (
	"context"

	"github.com/redis/go-redis/v9"
)

// SetBucketConfig 修改 Redis 中已存在桶的配置
func SetBucketConfig(ctx context.Context, rdb *redis.Client, key string, capacity, rate int64) error {
	// 直接修改 Hash 中的 capacity 和 rate 字段
	return rdb.HMSet(ctx, key, map[string]interface{}{
		"capacity": capacity,
		"rate":     rate,
	}).Err()
}

// SetCapacity 仅修改容量
func SetCapacity(ctx context.Context, rdb *redis.Client, key string, capacity int64) error {
	return rdb.HSet(ctx, key, "capacity", capacity).Err()
}

// SetRate 仅修改速率
func SetRate(ctx context.Context, rdb *redis.Client, key string, rate int64) error {
	return rdb.HSet(ctx, key, "rate", rate).Err()
}
