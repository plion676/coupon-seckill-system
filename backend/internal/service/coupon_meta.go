package service

import (
	"context"
	"fmt"
	"strconv"
	"time"

	mysql "coupon-seckill-system/internal/infra/mysql"
	rds "coupon-seckill-system/internal/infra/redis"
	"coupon-seckill-system/internal/model"
)

func GetCouponMeta(couponID int64) (startTime time.Time, endTime time.Time, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	key := fmt.Sprintf("coupon:meta:%d", couponID)
	if rds.RDB != nil {
		m, err := rds.RDB.HGetAll(ctx, key).Result()
		if err == nil && len(m) > 0 {
			startUnix, errStart := strconv.ParseInt(m["start_unix"], 10, 64)
			endUnix, errEnd := strconv.ParseInt(m["end_unix"], 10, 64)

			if errStart == nil && errEnd == nil {
				return time.Unix(startUnix, 0), time.Unix(endUnix, 0), nil
			}
		}
	}

	var coupon model.Coupon

	if err := mysql.DB.Select("id", "start_time", "end_time").
		First(&coupon, couponID).Error; err != nil {
		return time.Time{}, time.Time{}, err
	}

	startTime = coupon.StartTime
	endTime = coupon.EndTime

	cacheCouponMeta(ctx, couponID, startTime, endTime)
	return startTime, endTime, nil

}

func cacheCouponMeta(ctx context.Context, couponID int64, starttime, endtime time.Time) {
	if rds.RDB == nil {
		return
	}

	key := fmt.Sprintf("coupon:meta:%d", couponID)

	ctx, cancel := context.WithTimeout(ctx, 50*time.Millisecond)
	defer cancel()

	_ = rds.RDB.HSet(ctx, key, map[string]any{
		"start_unix": starttime.Unix(),
		"end_unix":   endtime.Unix()}).Err()

	ttl := time.Until(endtime.Add(time.Hour))
	if ttl <= 0 {
		ttl = time.Hour
	}

	_ = rds.RDB.Expire(ctx, key, ttl).Err()
}
