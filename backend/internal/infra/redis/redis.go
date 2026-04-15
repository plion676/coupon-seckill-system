package rds

import (
	"context"
	"log/slog"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

var RDB *goredis.Client

func ConnectRedis() {
	opts := &goredis.Options{
		Addr:         "127.0.0.1:6379",
		Password:     "",
		DB:           0,
		PoolSize:     2000, // ✅ 继续调大，应对 API+Worker 的总和
		MinIdleConns: 500,
		DialTimeout:  1 * time.Second, // ✅ 给握手留一点空间
		ReadTimeout:  2 * time.Second,
		WriteTimeout: 2 * time.Second,
		PoolTimeout:  2 * time.Second,
	}
	rdb := goredis.NewClient(opts)

	ctx := context.Background()

	err := rdb.Ping(ctx).Err()
	if err != nil {
		slog.Error("redis connect failed", "module", "redis", "addr", opts.Addr, "db", opts.DB, "pool_size", opts.PoolSize, "min_idle_conns", opts.MinIdleConns, "err", err)
		_ = rdb.Close()
		rdb = nil
	} else {
		slog.Info("redis connected", "module", "redis", "addr", opts.Addr, "db", opts.DB, "pool_size", opts.PoolSize, "min_idle_conns", opts.MinIdleConns)
	}
	RDB = rdb
}
