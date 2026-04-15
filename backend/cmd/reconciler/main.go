package main

import (
	"context"
	mysql "coupon-seckill-system/internal/infra/mysql"
	rds "coupon-seckill-system/internal/infra/redis"
	"coupon-seckill-system/internal/service"
	"log"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"
)

func main() {
	mysql.Connect()
	rds.ConnectRedis()

	if mysql.DB == nil {
		log.Fatal("mysql init failed")
	}
	if rds.RDB == nil {
		log.Fatal("redis init failed")
	}

	interval := loadInterval()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	log.Printf("reconciler started, interval=%s", interval)
	service.StartReconciler(ctx, interval)
	log.Print("reconciler stopped")
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
