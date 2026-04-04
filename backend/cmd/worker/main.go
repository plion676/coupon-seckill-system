package main

import (
	"context"
	"coupon-seckill-system/internal/infra/mysql"
	rds "coupon-seckill-system/internal/infra/redis"
	"coupon-seckill-system/internal/model"
	"fmt"
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
	mysql.Connect()
	rds.ConnectRedis()

	rootctx := context.Background()
	initctx, cancel := context.WithTimeout(rootctx, 100*time.Millisecond)
	err := rds.RDB.
		XGroupCreateMkStream(initctx, streamName, groupName, "0").Err()
	cancel()
	if err != nil {
		fmt.Print("消费者初始化提示", err)
	}
	for i := 0; i < 5; i++ {
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

	for {
		streams, err := rds.RDB.XReadGroup(ctx, &redis.XReadGroupArgs{
			Group:    groupName,
			Consumer: fmt.Sprintf("worker-%d", n),
			Streams:  []string{streamName, ">"},
			Count:    50,
			Block:    0,
		}).Result()
		if err != nil {
			continue
		}
		for _, stream := range streams {
			orders := make([]model.Order, 0, 50)
			msgIDs := make([]string, 0, 50)
			for _, xmsg := range stream.Messages {
				couponIDStr, _ := xmsg.Values["coupon_id"].(string)
				userIDStr, _ := xmsg.Values["user_id"].(string)

				couponID, err := strconv.ParseInt(couponIDStr, 10, 64)
				if err != nil {
					continue
				}
				userID, err := strconv.ParseInt(userIDStr, 10, 64)
				if err != nil {
					continue
				}
				msgIDs = append(msgIDs, xmsg.ID)
				orders = append(orders, model.Order{
					CouponID:  couponID,
					UserID:    userID,
					CreatedAt: time.Now(),
				})
			}
			tx := mysql.DB.Clauses(clause.OnConflict{DoNothing: true}).Create(&orders)
			if tx.Error != nil {
				fmt.Printf("数据库批量写入失败: %v,消息暂不确认\n", tx.Error)
				continue
			}
			rds.RDB.XAck(ctx, streamName, groupName, msgIDs...)
		}
	}
}
