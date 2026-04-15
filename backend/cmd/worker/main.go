package main

import (
	"context"
	"coupon-seckill-system/internal/infra/mysql"
	rds "coupon-seckill-system/internal/infra/redis"
	"coupon-seckill-system/internal/model"
	"coupon-seckill-system/internal/pkg/logger"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
	"gorm.io/gorm/clause"
)

const (
	streamName = "seckill:stream"
	groupName  = "seckill:group"
)

func main() {
	var wg sync.WaitGroup
	logger.Init("worker")

	mysql.Connect()
	rds.ConnectRedis()
	if mysql.DB == nil {
		slog.Error("mysql init failed", "module", "worker_main")
		os.Exit(1)
	}
	if rds.RDB == nil {
		slog.Error("redis init failed", "module", "worker_main")
		os.Exit(1)
	}

	rootctx := context.Background()
	initctx, cancel := context.WithTimeout(rootctx, 100*time.Millisecond)
	err := rds.RDB.
		XGroupCreateMkStream(initctx, streamName, groupName, "0").Err()
	cancel()
	if err != nil {
		slog.Warn("stream consumer group init result", "module", "worker_main", "stream", streamName, "group", groupName, "err", err)
	} else {
		slog.Info("stream consumer group ready", "module", "worker_main", "stream", streamName, "group", groupName)
	}
	slog.Info("worker pool starting", "module", "worker_main", "workers", 20, "stream", streamName, "group", groupName)
	for i := 0; i < 20; i++ {
		n := i
		wg.Add(1)
		go func() {
			defer wg.Done()
			worker(rootctx, n)
		}()
	}
	wg.Wait()
}

func worker(ctx context.Context, n int) {
	consumerName := fmt.Sprintf("worker-%d", n)
	slog.Info("worker started", "module", "worker", "consumer", consumerName, "stream", streamName, "group", groupName)
	startIDs := []string{"0", ">"}
	for {
		for _, lastID := range startIDs {
			readStartedAt := time.Now()
			streams, err := rds.RDB.XReadGroup(ctx, &redis.XReadGroupArgs{
				Group:    groupName,
				Consumer: consumerName,
				Streams:  []string{streamName, lastID},
				Count:    100,
				Block:    100 * time.Millisecond, // 稍微阻塞一下，避免空转
			}).Result()

			if err != nil {
				if err != redis.Nil {
					slog.Error("stream read failed", "module", "worker", "consumer", consumerName, "stream", streamName, "start_id", lastID, "err", err)
				}
				continue
			}

			for _, stream := range streams {
				orders := make([]model.Order, 0, 100)
				msgIDs := make([]string, 0, 100)
				invalidMessages := 0
				for _, xmsg := range stream.Messages {
					couponIDStr, _ := xmsg.Values["coupon_id"].(string)
					userIDStr, _ := xmsg.Values["user_id"].(string)

					couponID, err := strconv.ParseInt(couponIDStr, 10, 64)
					if err != nil {
						invalidMessages++
						slog.Warn("invalid stream message coupon_id", "module", "worker", "consumer", consumerName, "msg_id", xmsg.ID, "coupon_id", couponIDStr, "user_id", userIDStr)
						continue
					}
					userID, err := strconv.ParseInt(userIDStr, 10, 64)
					if err != nil {
						invalidMessages++
						slog.Warn("invalid stream message user_id", "module", "worker", "consumer", consumerName, "msg_id", xmsg.ID, "coupon_id", couponIDStr, "user_id", userIDStr)
						continue
					}
					msgIDs = append(msgIDs, xmsg.ID)
					orders = append(orders, model.Order{
						CouponID:  couponID,
						UserID:    userID,
						CreatedAt: time.Now(),
					})
				}

				if len(stream.Messages) > 0 {
					slog.Debug("stream batch received",
						"module", "worker",
						"consumer", consumerName,
						"stream", stream.Stream,
						"start_id", lastID,
						"messages", len(stream.Messages),
						"valid_orders", len(orders),
						"invalid_messages", invalidMessages,
						"duration_ms", time.Since(readStartedAt).Milliseconds(),
					)
				}

				if len(orders) == 0 {
					if len(msgIDs) > 0 {
						if err := rds.RDB.XAck(ctx, streamName, groupName, msgIDs...).Err(); err != nil {
							slog.Warn("stream ack failed", "module", "worker", "consumer", consumerName, "stream", streamName, "msg_count", len(msgIDs), "err", err)
						}
					}
					continue
				}

				writeStartedAt := time.Now()
				tx := mysql.DB.Clauses(clause.OnConflict{DoNothing: true}).Create(&orders)
				if tx.Error != nil {
					slog.Error("order batch insert failed", "module", "worker", "consumer", consumerName, "stream", streamName, "order_count", len(orders), "err", tx.Error)
					continue
				}
				slog.Debug("order batch inserted", "module", "worker", "consumer", consumerName, "stream", streamName, "order_count", len(orders), "duration_ms", time.Since(writeStartedAt).Milliseconds())
				if err := rds.RDB.XAck(ctx, streamName, groupName, msgIDs...).Err(); err != nil {
					slog.Warn("stream ack failed", "module", "worker", "consumer", consumerName, "stream", streamName, "msg_count", len(msgIDs), "err", err)
					continue
				}
				slog.Debug("stream batch acked", "module", "worker", "consumer", consumerName, "stream", streamName, "msg_count", len(msgIDs))
			}
		}
	}
}
