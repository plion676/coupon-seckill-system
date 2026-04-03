package model

import (
	"time"
)

type Coupon struct {
	ID        int64     `gorm:"primaryKey"`
	Stock     int       `gorm:"not null"`
	StartTime time.Time `gorm:"column:start_time;not null"`
	EndTime   time.Time `gorm:"column:end_time;not null"`
}

type Order struct {
	ID        int64     `gorm:"primaryKey"`
	CouponID  int64     `gorm:"column:coupon_id;not null"`
	UserID    int64     `gorm:"column:user_id;not null"`
	CreatedAt time.Time `gorm:"column:created_at;not null"`
}
