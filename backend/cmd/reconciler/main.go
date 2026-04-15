package main

import (
	"context"
	mysql "coupon-seckill-system/internal/infra/mysql"
	rds "coupon-seckill-system/internal/infra/redis"
	"coupon-seckill-system/internal/pkg/logger"
	"coupon-seckill-system/internal/service"
	"log/slog"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"
)

func main() {
	logger.Init("reconciler")

	mysql.Connect()
	rds.ConnectRedis()

	if mysql.DB == nil {
		slog.Error("mysql init failed", "module", "reconciler_main")
		os.Exit(1)
	}
	if rds.RDB == nil {
		slog.Error("redis init failed", "module", "reconciler_main")
		os.Exit(1)
	}

	interval := loadInterval()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	slog.Info("reconciler started", "module", "reconciler_main", "interval", interval.String())
	service.StartReconciler(ctx, interval)
	slog.Info("reconciler stopped", "module", "reconciler_main")
}

func loadInterval() time.Duration {
	raw := os.Getenv("RECONCILE_INTERVAL_SECONDS")
	if raw == "" {
		return time.Minute
	}

	seconds, err := strconv.Atoi(raw)
	if err != nil || seconds <= 0 {
		return time.Minute
	}

	return time.Duration(seconds) * time.Second
}
