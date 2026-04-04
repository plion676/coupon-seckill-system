package handler

import (
	"errors"
	"strconv"

	"coupon-seckill-system/internal/service"

	"github.com/gin-gonic/gin"
)

func SeckillHandler(c *gin.Context) {
	couponID, err := strconv.ParseInt(c.Query("coupon_id"), 10, 64)
	if err != nil || couponID <= 0 {
		c.JSON(400, gin.H{
			"msg": "invalid coupon_id"})
		return
	}
	userID, err := strconv.ParseInt(c.Query("user_id"), 10, 64)
	if err != nil || userID <= 0 {
		c.JSON(400, gin.H{
			"msg": "invalid user_id"})
		return
	}

	err = service.Seckill(couponID, userID)
	switch {
	case err == nil:
		c.JSON(200, gin.H{
			"msg": "抢购中"})
	case errors.Is(err, service.ErrNotStarted):
		c.JSON(400, gin.H{
			"msg": "not started"})
	case errors.Is(err, service.ErrEnded):
		c.JSON(400, gin.H{
			"msg": "Ended"})
	case errors.Is(err, service.ErrNoStock):
		c.JSON(409, gin.H{
			"msg": "no stock"})
	case errors.Is(err, service.ErrDuplicateOrder):
		c.JSON(409, gin.H{
			"msg": "duplicate"})
	case errors.Is(err, service.ErrRedisUnavailable):
		c.JSON(409, gin.H{
			"msg": "redis unavailable"})
	default:
		c.JSON(500, gin.H{
			"msg": "server error"})
	}
}
