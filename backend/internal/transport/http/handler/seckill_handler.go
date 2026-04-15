package handler

import (
	"errors"
	"log/slog"
	"strconv"
	"time"

	"coupon-seckill-system/internal/service"

	"github.com/gin-gonic/gin"
)

func SeckillHandler(c *gin.Context) {
	startedAt := time.Now()

	couponID, err := strconv.ParseInt(c.Query("coupon_id"), 10, 64)
	if err != nil || couponID <= 0 {
		slog.Warn("seckill request rejected", "module", "http_seckill", "path", c.FullPath(), "status", 400, "result", "invalid_coupon_id", "coupon_id_raw", c.Query("coupon_id"), "user_id_raw", c.Query("user_id"), "duration_ms", time.Since(startedAt).Milliseconds())
		c.JSON(400, gin.H{
			"msg": "invalid coupon_id"})
		return
	}
	userID, err := strconv.ParseInt(c.Query("user_id"), 10, 64)
	if err != nil || userID <= 0 {
		slog.Warn("seckill request rejected", "module", "http_seckill", "path", c.FullPath(), "status", 400, "result", "invalid_user_id", "coupon_id", couponID, "user_id_raw", c.Query("user_id"), "duration_ms", time.Since(startedAt).Milliseconds())
		c.JSON(400, gin.H{
			"msg": "invalid user_id"})
		return
	}

	err = service.Seckill(couponID, userID)
	switch {
	case err == nil:
		slog.Info("seckill request completed", "module", "http_seckill", "path", c.FullPath(), "coupon_id", couponID, "user_id", userID, "status", 200, "result", "accepted", "duration_ms", time.Since(startedAt).Milliseconds())
		c.JSON(200, gin.H{
			"msg": "抢购中"})
	case errors.Is(err, service.ErrNotStarted):
		slog.Info("seckill request completed", "module", "http_seckill", "path", c.FullPath(), "coupon_id", couponID, "user_id", userID, "status", 400, "result", "not_started", "duration_ms", time.Since(startedAt).Milliseconds())
		c.JSON(400, gin.H{
			"msg": "not started"})
	case errors.Is(err, service.ErrEnded):
		slog.Info("seckill request completed", "module", "http_seckill", "path", c.FullPath(), "coupon_id", couponID, "user_id", userID, "status", 400, "result", "ended", "duration_ms", time.Since(startedAt).Milliseconds())
		c.JSON(400, gin.H{
			"msg": "Ended"})
	case errors.Is(err, service.ErrNoStock):
		slog.Info("seckill request completed", "module", "http_seckill", "path", c.FullPath(), "coupon_id", couponID, "user_id", userID, "status", 409, "result", "no_stock", "duration_ms", time.Since(startedAt).Milliseconds())
		c.JSON(409, gin.H{
			"msg": "no stock"})
	case errors.Is(err, service.ErrDuplicateOrder):
		slog.Info("seckill request completed", "module", "http_seckill", "path", c.FullPath(), "coupon_id", couponID, "user_id", userID, "status", 409, "result", "duplicate", "duration_ms", time.Since(startedAt).Milliseconds())
		c.JSON(409, gin.H{
			"msg": "duplicate"})
	case errors.Is(err, service.ErrRedisUnavailable):
		slog.Error("seckill request failed", "module", "http_seckill", "path", c.FullPath(), "coupon_id", couponID, "user_id", userID, "status", 409, "result", "redis_unavailable", "duration_ms", time.Since(startedAt).Milliseconds(), "err", err)
		c.JSON(409, gin.H{
			"msg": "redis unavailable"})
	case errors.Is(err, service.ErrServerBusy):
		slog.Warn("seckill request failed", "module", "http_seckill", "path", c.FullPath(), "coupon_id", couponID, "user_id", userID, "status", 409, "result", "server_busy", "duration_ms", time.Since(startedAt).Milliseconds(), "err", err)
		c.JSON(409, gin.H{
			"msg": "serve busy"})
	default:
		slog.Error("seckill request failed", "module", "http_seckill", "path", c.FullPath(), "coupon_id", couponID, "user_id", userID, "status", 500, "result", "server_error", "duration_ms", time.Since(startedAt).Milliseconds(), "err", err)
		c.JSON(500, gin.H{
			"msg": "server error"})
	}
}
