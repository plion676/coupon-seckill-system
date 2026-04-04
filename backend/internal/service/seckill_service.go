package service

import (
	"context"
	"errors"
	"time"

	rds "coupon-seckill-system/internal/infra/redis"

	mysqldriver "github.com/go-sql-driver/mysql"
	"github.com/redis/go-redis/v9"
)

func Seckill(couponID, userID int64) error {
	if isSoldOut, ok := soldOutCache.Load(couponID); ok && isSoldOut.(bool) {
		return ErrNoStock
	}
	code, err := EvalSeckill(couponID, userID)
	if err != nil {
		return err
	}

	switch code {
	case 0:
	case 1:
		soldOutCache.Store(couponID, true)
		return ErrNoStock
	case 2:
		return ErrDuplicateOrder
	case 3:
		return ErrNotStarted
	case 4:
		return ErrEnded
	default:
		return ErrCouponDataMissing
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)

	defer cancel()
	err = rds.RDB.XAdd(ctx, &redis.XAddArgs{
		Stream: "seckill:stream",
		ID:     "*",
		Values: map[string]any{
			"coupon_id": couponID,
			"user_id":   userID,
		},
	}).Err()
	if err != nil {
		return ErrRedisUnavailable
	}
	return nil
}

// 判断一个用户是否对一个coupon抢了两次，mysql中user_id和coupon_id有唯一键关联
func IsDuplicateKeyErr(err error) bool {
	var me *mysqldriver.MySQLError
	if errors.As(err, &me) {
		return me.Number == 1062
	}
	return false
}
