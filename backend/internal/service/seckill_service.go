package service

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
	return nil
}
