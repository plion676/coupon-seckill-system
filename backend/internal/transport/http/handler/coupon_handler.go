package handler

import (
	"time"

	mysql "coupon-seckill-system/internal/infra/mysql"
	"coupon-seckill-system/internal/model"
	"coupon-seckill-system/internal/service"

	"github.com/gin-gonic/gin"
)

// 创建活动的请求，包含库存，开始时间，结束时间
type CreateCouponReq struct {
	Stock     int    `json:"stock" binding:"required,min=1"`
	StartTime string `json:"start_time" binding:"required"`
	EndTime   string `json:"end_time" binding:"required"`
}

func CreateCoupon(c *gin.Context) {
	var req CreateCouponReq
	//c.ShouldBindJSON将接口传的json反序列化后写进req
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{
			"msg": "bad request",
			"err": err.Error()})
		return
	}
	//将时间按格式，按指定时区输出
	st, err := time.ParseInLocation("2006-01-02 15:04:05", req.StartTime, time.Local)
	if err != nil {
		c.JSON(400, gin.H{
			"msg": "invalid start_time"})
		return
	}
	et, err := time.ParseInLocation("2006-01-02 15:04:05", req.EndTime, time.Local)
	if err != nil {
		c.JSON(400, gin.H{
			"msg": "invalid end_time"})
		return
	}

	if !et.After(st) {
		c.JSON(400, gin.H{
			"msg": "end_time must be after start_time"})
		return
	}

	coupon := model.Coupon{
		Stock:     req.Stock,
		StartTime: st,
		EndTime:   et,
	}

	if err := mysql.DB.Create(&coupon).Error; err != nil {
		c.JSON(500, gin.H{"msg": "db error"})
		return
	}

	err = service.GetCouponStock(coupon.ID)
	if err != nil {
		c.JSON(500, gin.H{"msg": "stockpreheat failed"})
		return
	}
	_, _, err = service.GetCouponMeta(coupon.ID)
	if err != nil {
		c.JSON(500, gin.H{"msg": "metapreheat failed"})
		return
	}

	c.JSON(200, gin.H{"coupon_id": coupon.ID})

}
