package service

import (
	"context"
	rds "coupon-seckill-system/internal/infra/redis"
	"log/slog"
	"time"

	"github.com/redis/go-redis/v9"
)

type SeckillMessage struct {
	CouponID int64
	UserID   int64
}

const numChans = 32

var seckillChans [numChans]chan SeckillMessage

func init() {
	for i := 0; i < numChans; i++ {
		seckillChans[i] = make(chan SeckillMessage, 2000)
	}
}
func DispatchSeckillMessage(msg SeckillMessage) error {
	idx := msg.UserID % int64(numChans)
	select {
	case seckillChans[idx] <- msg:
		return nil
	default:
		slog.Warn("seckill dispatch queue full", "module", "seckill_async", "coupon_id", msg.CouponID, "user_id", msg.UserID, "queue_index", idx, "queue_len", len(seckillChans[idx]), "queue_cap", cap(seckillChans[idx]))
		return ErrServerBusy
	}
}
func StartAsyncWriter() {
	for i := 0; i < numChans; i++ {
		ch := seckillChans[i]
		go func(queueIndex int, c chan SeckillMessage) {
			batchSize := 100
			var batch []SeckillMessage
			ticker := time.NewTicker(300 * time.Millisecond)
			defer ticker.Stop()
			for {
				select {
				case msg := <-c:
					batch = append(batch, msg)
					if len(batch) >= batchSize {
						sendToRedis(batch, queueIndex, "batch_full")
						batch = batch[:0]
					}
				case <-ticker.C:
					if len(batch) > 0 {
						sendToRedis(batch, queueIndex, "ticker")
						batch = batch[:0]
					}
				}
			}
		}(i, ch)
	}
}

func sendToRedis(batch []SeckillMessage, queueIndex int, trigger string) {
	if len(batch) == 0 {
		return
	}

	pipe := rds.RDB.Pipeline()
	ctx := context.Background()
	startedAt := time.Now()

	for _, msg := range batch {
		pipe.XAdd(ctx, &redis.XAddArgs{
			Stream: "seckill:stream",
			Values: map[string]any{
				"coupon_id": msg.CouponID,
				"user_id":   msg.UserID,
			},
		})
	}

	_, err := pipe.Exec(ctx)
	if err != nil {
		slog.Error("stream batch write failed", "module", "seckill_async", "stream", "seckill:stream", "queue_index", queueIndex, "trigger", trigger, "batch_size", len(batch), "duration_ms", time.Since(startedAt).Milliseconds(), "err", err)
		return
	}
	slog.Debug("stream batch written", "module", "seckill_async", "stream", "seckill:stream", "queue_index", queueIndex, "trigger", trigger, "batch_size", len(batch), "duration_ms", time.Since(startedAt).Milliseconds())
}
