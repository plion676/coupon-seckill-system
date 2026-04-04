package rds

import (
	"context"

	goredis "github.com/redis/go-redis/v9"
)

var RDB *goredis.Client

func ConnectRedis() {
	rdb := goredis.NewClient(&goredis.Options{
		Addr:     "127.0.0.1:6379",
		Password: "",
		DB:       0,
		PoolSize: 500,
	})

	ctx := context.Background()

	err := rdb.Ping(ctx).Err()
	if err != nil {
		_ = rdb.Close()
		rdb = nil
	}
	RDB = rdb
}
