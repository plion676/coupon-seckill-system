package service

import (
	"context"
	mysql "coupon-seckill-system/internal/infra/mysql"
	rds "coupon-seckill-system/internal/infra/redis"
	"coupon-seckill-system/internal/model"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"gorm.io/gorm/clause"
)

const (
	reconcileCouponBatchSize = 200
	reconcileUserBatchSize   = 500
)

type ReconcileSummary struct {
	CouponsScanned     int
	OrdersCompensated  int
	RedisUsersRepaired int
	StocksAdjusted     int
}

func StartReconciler(ctx context.Context, interval time.Duration) {
	if interval <= 0 {
		interval = time.Minute
	}

	runOnce := func() {
		startedAt := time.Now()
		runCtx, cancel := context.WithTimeout(ctx, interval)
		defer cancel()

		slog.Info("reconcile run started", "module", "reconcile", "interval", interval.String())
		summary, err := ReconcileOnce(runCtx)
		if err != nil {
			slog.Error("reconcile run failed", "module", "reconcile", "interval", interval.String(), "duration_ms", time.Since(startedAt).Milliseconds(), "err", err)
			return
		}

		slog.Info("reconcile run finished",
			"module", "reconcile",
			"coupons_scanned", summary.CouponsScanned,
			"orders_compensated", summary.OrdersCompensated,
			"redis_users_repaired", summary.RedisUsersRepaired,
			"stocks_adjusted", summary.StocksAdjusted,
			"duration_ms", time.Since(startedAt).Milliseconds(),
		)
	}

	runOnce()

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			runOnce()
		}
	}
}

func ReconcileOnce(ctx context.Context) (ReconcileSummary, error) {
	if mysql.DB == nil {
		return ReconcileSummary{}, fmt.Errorf("mysql is not initialized")
	}
	if rds.RDB == nil {
		return ReconcileSummary{}, fmt.Errorf("redis is not initialized")
	}

	var summary ReconcileSummary
	var lastCouponID int64

	for {
		coupons, err := loadCouponBatch(ctx, lastCouponID, reconcileCouponBatchSize)
		if err != nil {
			return summary, err
		}
		if len(coupons) == 0 {
			return summary, nil
		}

		for _, coupon := range coupons {
			lastCouponID = coupon.ID
			summary.CouponsScanned++

			couponSummary, err := reconcileCoupon(ctx, coupon)
			if err != nil {
				slog.Error("reconcile coupon failed", "module", "reconcile", "coupon_id", coupon.ID, "err", err)
				continue
			}
			summary.OrdersCompensated += couponSummary.OrdersCompensated
			summary.RedisUsersRepaired += couponSummary.RedisUsersRepaired
			summary.StocksAdjusted += couponSummary.StocksAdjusted
		}
	}
}

func loadCouponBatch(ctx context.Context, lastCouponID int64, limit int) ([]model.Coupon, error) {
	var coupons []model.Coupon
	err := mysql.DB.WithContext(ctx).
		Select("id", "stock").
		Where("id > ?", lastCouponID).
		Order("id ASC").
		Limit(limit).
		Find(&coupons).Error
	return coupons, err
}

type couponReconcileSummary struct {
	OrdersCompensated  int
	RedisUsersRepaired int
	StocksAdjusted     int
}

func reconcileCoupon(ctx context.Context, coupon model.Coupon) (couponReconcileSummary, error) {
	orderUsers, err := loadOrderUsers(ctx, coupon.ID)
	if err != nil {
		return couponReconcileSummary{}, err
	}

	redisUsers, err := loadRedisUsers(ctx, coupon.ID)
	if err != nil {
		return couponReconcileSummary{}, err
	}

	now := time.Now()
	missingOrders := buildMissingOrders(coupon.ID, redisUsers, orderUsers, now)
	missingRedisUsers := buildMissingRedisUsers(redisUsers, orderUsers)

	if err := compensateOrders(ctx, missingOrders); err != nil {
		return couponReconcileSummary{}, err
	}
	if err := repairRedisUsers(ctx, coupon.ID, missingRedisUsers); err != nil {
		return couponReconcileSummary{}, err
	}

	finalReservedUsers := len(redisUsers) + len(missingRedisUsers)
	expectedStock := coupon.Stock - finalReservedUsers
	if expectedStock < 0 {
		expectedStock = 0
	}

	stockAdjusted, err := repairRedisStock(ctx, coupon.ID, expectedStock)
	if err != nil {
		return couponReconcileSummary{}, err
	}

	if len(missingOrders) > 0 || len(missingRedisUsers) > 0 || stockAdjusted {
		slog.Warn("coupon reconciled with inconsistencies",
			"module", "reconcile",
			"coupon_id", coupon.ID,
			"redis_users", len(redisUsers),
			"mysql_orders", len(orderUsers),
			"missing_orders", len(missingOrders),
			"missing_redis_users", len(missingRedisUsers),
			"expected_stock", expectedStock,
			"stock_adjusted", stockAdjusted,
		)
	}

	return couponReconcileSummary{
		OrdersCompensated:  len(missingOrders),
		RedisUsersRepaired: len(missingRedisUsers),
		StocksAdjusted:     boolToInt(stockAdjusted),
	}, nil
}

func loadOrderUsers(ctx context.Context, couponID int64) (map[int64]struct{}, error) {
	var userIDs []int64
	err := mysql.DB.WithContext(ctx).
		Model(&model.Order{}).
		Where("coupon_id = ?", couponID).
		Pluck("user_id", &userIDs).Error
	if err != nil {
		return nil, err
	}

	users := make(map[int64]struct{}, len(userIDs))
	for _, userID := range userIDs {
		users[userID] = struct{}{}
	}
	return users, nil
}

func loadRedisUsers(ctx context.Context, couponID int64) (map[int64]struct{}, error) {
	usersKey := fmt.Sprintf("coupon:users:%d", couponID)
	users := make(map[int64]struct{})

	var cursor uint64
	for {
		members, nextCursor, err := rds.RDB.SScan(ctx, usersKey, cursor, "", int64(reconcileUserBatchSize)).Result()
		if err != nil {
			return nil, err
		}

		for _, member := range members {
			userID, err := strconv.ParseInt(member, 10, 64)
			if err != nil {
				slog.Warn("invalid redis user member", "module", "reconcile", "coupon_id", couponID, "member", member)
				continue
			}
			users[userID] = struct{}{}
		}

		cursor = nextCursor
		if cursor == 0 {
			return users, nil
		}
	}
}

func buildMissingOrders(couponID int64, redisUsers, orderUsers map[int64]struct{}, createdAt time.Time) []model.Order {
	missingOrders := make([]model.Order, 0)
	for userID := range redisUsers {
		if _, ok := orderUsers[userID]; ok {
			continue
		}
		missingOrders = append(missingOrders, model.Order{
			CouponID:  couponID,
			UserID:    userID,
			CreatedAt: createdAt,
		})
	}
	return missingOrders
}

func buildMissingRedisUsers(redisUsers, orderUsers map[int64]struct{}) []string {
	missingUsers := make([]string, 0)
	for userID := range orderUsers {
		if _, ok := redisUsers[userID]; ok {
			continue
		}
		missingUsers = append(missingUsers, strconv.FormatInt(userID, 10))
	}
	return missingUsers
}

func compensateOrders(ctx context.Context, orders []model.Order) error {
	if len(orders) == 0 {
		return nil
	}

	return mysql.DB.WithContext(ctx).
		Clauses(clause.OnConflict{DoNothing: true}).
		CreateInBatches(orders, reconcileUserBatchSize).Error
}

func repairRedisUsers(ctx context.Context, couponID int64, userIDs []string) error {
	if len(userIDs) == 0 {
		return nil
	}

	usersKey := fmt.Sprintf("coupon:users:%d", couponID)
	values := make([]any, 0, len(userIDs))
	for _, userID := range userIDs {
		values = append(values, userID)
	}

	return rds.RDB.SAdd(ctx, usersKey, values...).Err()
}

func repairRedisStock(ctx context.Context, couponID int64, expectedStock int) (bool, error) {
	stockKey := fmt.Sprintf("coupon:stock:%d", couponID)
	currentStock, err := rds.RDB.Get(ctx, stockKey).Int()
	if err == nil && currentStock == expectedStock {
		return false, nil
	}
	if err != nil {
		slog.Warn("redis stock read fallback to overwrite", "module", "reconcile", "coupon_id", couponID, "expected_stock", expectedStock, "err", err)
	}

	if err := rds.RDB.Set(ctx, stockKey, expectedStock, 0).Err(); err != nil {
		return false, err
	}
	return true, nil
}

func boolToInt(v bool) int {
	if v {
		return 1
	}
	return 0
}
