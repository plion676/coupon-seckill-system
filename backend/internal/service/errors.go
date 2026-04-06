package service

import "errors"

var (
	ErrNotStarted        = errors.New("not started")
	ErrEnded             = errors.New("ended")
	ErrNoStock           = errors.New("no stock")
	ErrDuplicateOrder    = errors.New("duplicate order")
	ErrCouponDataMissing = errors.New("coupon data missing")
	ErrRedisUnavailable  = errors.New("redis unavailable")
	ErrServerBusy        = errors.New("serve busy")
)
