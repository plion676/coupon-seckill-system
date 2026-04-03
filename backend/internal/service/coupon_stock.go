package service

import (
	"context"
	"fmt"
	"time"

	mysql "coupon-seckill-system/internal/infra/mysql"
	rds "coupon-seckill-system/internal/infra/redis"
	"coupon-seckill-system/internal/model"
)

func GetCouponStock(couponID int64) error {
	if rds.RDB == nil {
		return ErrRedisUnavailable
	}

	var coupon model.Coupon
	if err := mysql.DB.Select("stock").First(&coupon, couponID).Error; err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	key := fmt.Sprintf("coupon:stock:%d", couponID)
	defer cancel()

	err := rds.RDB.Set(ctx, key, coupon.Stock, 0).Err()
	if err != nil {
		return err
	}
	return nil
}
