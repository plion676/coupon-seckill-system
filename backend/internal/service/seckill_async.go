package service

import (
	"context"
	rds "coupon-seckill-system/internal/infra/redis"
	"fmt"
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
		return ErrServerBusy
	}
}
func StartAsyncWriter() {
	for i := 0; i < numChans; i++ {
		ch := seckillChans[i]
		go func(c chan SeckillMessage) {
			batchSize := 100
			var batch []SeckillMessage
			ticker := time.NewTicker(300 * time.Millisecond)
			defer ticker.Stop()
			for {
				select {
				case msg := <-c:
					batch = append(batch, msg)
					if len(batch) >= batchSize {
						sendToRedis(batch)
						batch = batch[:0]
					}
				case <-ticker.C:
					if len(batch) > 0 {
						sendToRedis(batch)
						batch = batch[:0]
					}
				}
			}
		}(ch)
	}
}

func sendToRedis(batch []SeckillMessage) {
	if len(batch) == 0 {
		return
	}

	pipe := rds.RDB.Pipeline()
	ctx := context.Background()

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
		fmt.Printf("Pipeline批量写入失败:", err)
	}
}
