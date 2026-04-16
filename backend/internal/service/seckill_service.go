package service

import "log/slog"

func Seckill(couponID, userID int64) error {
	if isSoldOut, ok := soldOutCache.Load(couponID); ok && isSoldOut.(bool) {
		slog.Debug("sold out cache hit", "module", "seckill_service", "coupon_id", couponID, "user_id", userID, "result", "no_stock")
		return ErrNoStock
	}
	code, err := EvalSeckill(couponID, userID)
	if err != nil {
		slog.Error("lua seckill execution failed", "module", "seckill_service", "coupon_id", couponID, "user_id", userID, "err", err)
		return err
	}

	switch code {
	case 0:
		slog.Debug("seckill accepted", "module", "seckill_service", "coupon_id", couponID, "user_id", userID, "lua_code", code, "result", "accepted")
		return nil
	case 1:
		soldOutCache.Store(couponID, true)
		slog.Debug("seckill rejected", "module", "seckill_service", "coupon_id", couponID, "user_id", userID, "lua_code", code, "result", "no_stock")
		return ErrNoStock
	case 2:
		slog.Debug("seckill rejected", "module", "seckill_service", "coupon_id", couponID, "user_id", userID, "lua_code", code, "result", "duplicate")
		return ErrDuplicateOrder
	case 3:
		slog.Debug("seckill rejected", "module", "seckill_service", "coupon_id", couponID, "user_id", userID, "lua_code", code, "result", "not_started")
		return ErrNotStarted
	case 4:
		slog.Debug("seckill rejected", "module", "seckill_service", "coupon_id", couponID, "user_id", userID, "lua_code", code, "result", "ended")
		return ErrEnded
	default:
		slog.Warn("unexpected lua seckill result", "module", "seckill_service", "coupon_id", couponID, "user_id", userID, "lua_code", code, "result", "coupon_data_missing")
		return ErrCouponDataMissing
	}
}
