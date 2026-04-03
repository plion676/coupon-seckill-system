package service

import (
	"errors"
	"time"

	mysql "coupon-seckill-system/internal/infra/mysql"
	"coupon-seckill-system/internal/model"

	mysqldriver "github.com/go-sql-driver/mysql"
)

func Seckill(couponID, userID int64) error {
	code, err := EvalSeckill(couponID, userID)
	if err != nil {
		return err
	}

	switch code {
	case 0:
	case 1:
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

	order := model.Order{
		CouponID:  couponID,
		UserID:    userID,
		CreatedAt: time.Now(),
	}
	if err := mysql.DB.Create(&order).Error; err != nil {
		if isDuplicateKeyErr(err) {
			return ErrDuplicateOrder
		}
		return err
	}
	return nil
}

//判断一个用户是否对一个coupon抢了两次，mysql中user_id和coupon_id有唯一键关联
func isDuplicateKeyErr(err error) bool {
	var me *mysqldriver.MySQLError
	if errors.As(err, &me) {
		return me.Number == 1062
	}
	return false
}
