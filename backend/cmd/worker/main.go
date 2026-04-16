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
	"os/signal"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/redis/go-redis/v9"
	"gorm.io/gorm/clause"
)

const (
	streamName     = "seckill:stream"
	groupName      = "seckill:group"
	deadLetterName = "seckill:dead-letter"
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

	rootctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

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
	slog.Info("worker pool stopped", "module", "worker_main")
}

func worker(ctx context.Context, n int) {
	consumerName := fmt.Sprintf("worker-%d", n)
	slog.Info("worker started", "module", "worker", "consumer", consumerName, "stream", streamName, "group", groupName)
	startIDs := []string{"0", ">"}
	for {
		if ctx.Err() != nil {
			slog.Info("worker stopping", "module", "worker", "consumer", consumerName)
			return
		}

		for _, lastID := range startIDs {
			if ctx.Err() != nil {
				slog.Info("worker stopping", "module", "worker", "consumer", consumerName)
				return
			}

			readStartedAt := time.Now()
			streams, err := rds.RDB.XReadGroup(ctx, &redis.XReadGroupArgs{
				Group:    groupName,
				Consumer: consumerName,
				Streams:  []string{streamName, lastID},
				Count:    100,
				Block:    100 * time.Millisecond,
			}).Result()

			if err != nil {
				if ctx.Err() != nil {
					slog.Info("worker read loop stopped", "module", "worker", "consumer", consumerName)
					return
				}
				if err != redis.Nil {
					slog.Error("stream read failed", "module", "worker", "consumer", consumerName, "stream", streamName, "start_id", lastID, "err", err)
				}
				continue
			}

			for _, stream := range streams {
				orders := make([]model.Order, 0, 100)
				validMsgIDs := make([]string, 0, 100)
				invalidMsgIDs := make([]string, 0, 100)
				invalidEntries := make([]deadLetterEntry, 0, 100)
				invalidMessages := 0
				for _, xmsg := range stream.Messages {
					couponIDStr := stringifyStreamValue(xmsg.Values["coupon_id"])
					userIDStr := stringifyStreamValue(xmsg.Values["user_id"])

					couponID, err := strconv.ParseInt(couponIDStr, 10, 64)
					if err != nil {
						invalidMessages++
						invalidMsgIDs = append(invalidMsgIDs, xmsg.ID)
						invalidEntries = append(invalidEntries, buildDeadLetterEntry(stream.Stream, xmsg, "invalid_coupon_id"))
						slog.Warn("invalid stream message coupon_id", "module", "worker", "consumer", consumerName, "msg_id", xmsg.ID, "coupon_id", couponIDStr, "user_id", userIDStr)
						continue
					}
					userID, err := strconv.ParseInt(userIDStr, 10, 64)
					if err != nil {
						invalidMessages++
						invalidMsgIDs = append(invalidMsgIDs, xmsg.ID)
						invalidEntries = append(invalidEntries, buildDeadLetterEntry(stream.Stream, xmsg, "invalid_user_id"))
						slog.Warn("invalid stream message user_id", "module", "worker", "consumer", consumerName, "msg_id", xmsg.ID, "coupon_id", couponIDStr, "user_id", userIDStr)
						continue
					}
					validMsgIDs = append(validMsgIDs, xmsg.ID)
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

				if len(invalidEntries) > 0 {
					if err := sendToDeadLetter(ctx, consumerName, invalidEntries); err != nil {
						slog.Error("dead-letter write failed", "module", "worker", "consumer", consumerName, "stream", deadLetterName, "msg_count", len(invalidEntries), "err", err)
					} else if err := ackMessages(ctx, consumerName, invalidMsgIDs); err != nil {
						slog.Warn("invalid message ack failed", "module", "worker", "consumer", consumerName, "stream", streamName, "msg_count", len(invalidMsgIDs), "err", err)
					} else {
						slog.Warn("invalid messages moved to dead-letter", "module", "worker", "consumer", consumerName, "dead_letter_stream", deadLetterName, "msg_count", len(invalidEntries))
					}
				}

				if len(orders) == 0 {
					continue
				}

				writeStartedAt := time.Now()
				tx := mysql.DB.Clauses(clause.OnConflict{DoNothing: true}).Create(&orders)
				if tx.Error != nil {
					slog.Error("order batch insert failed", "module", "worker", "consumer", consumerName, "stream", streamName, "order_count", len(orders), "err", tx.Error)
					continue
				}
				slog.Debug("order batch inserted", "module", "worker", "consumer", consumerName, "stream", streamName, "order_count", len(orders), "duration_ms", time.Since(writeStartedAt).Milliseconds())
				if err := ackMessages(ctx, consumerName, validMsgIDs); err != nil {
					slog.Warn("stream ack failed", "module", "worker", "consumer", consumerName, "stream", streamName, "msg_count", len(validMsgIDs), "err", err)
					continue
				}
			}
		}
	}
}

type deadLetterEntry struct {
	OriginalStream string
	OriginalID     string
	CouponID       string
	UserID         string
	Reason         string
}

func buildDeadLetterEntry(stream string, msg redis.XMessage, reason string) deadLetterEntry {
	return deadLetterEntry{
		OriginalStream: stream,
		OriginalID:     msg.ID,
		CouponID:       stringifyStreamValue(msg.Values["coupon_id"]),
		UserID:         stringifyStreamValue(msg.Values["user_id"]),
		Reason:         reason,
	}
}

func sendToDeadLetter(ctx context.Context, consumerName string, entries []deadLetterEntry) error {
	if len(entries) == 0 {
		return nil
	}

	pipe := rds.RDB.Pipeline()
	for _, entry := range entries {
		pipe.XAdd(ctx, &redis.XAddArgs{
			Stream: deadLetterName,
			Values: map[string]any{
				"original_stream": entry.OriginalStream,
				"original_id":     entry.OriginalID,
				"coupon_id":       entry.CouponID,
				"user_id":         entry.UserID,
				"reason":          entry.Reason,
				"consumer":        consumerName,
			},
		})
	}

	_, err := pipe.Exec(ctx)
	return err
}

func ackMessages(ctx context.Context, consumerName string, msgIDs []string) error {
	if len(msgIDs) == 0 {
		return nil
	}

	if err := rds.RDB.XAck(ctx, streamName, groupName, msgIDs...).Err(); err != nil {
		return err
	}
	slog.Debug("stream batch acked", "module", "worker", "consumer", consumerName, "stream", streamName, "msg_count", len(msgIDs))
	return nil
}

func stringifyStreamValue(v any) string {
	switch value := v.(type) {
	case nil:
		return ""
	case string:
		return value
	default:
		return fmt.Sprint(value)
	}
}
