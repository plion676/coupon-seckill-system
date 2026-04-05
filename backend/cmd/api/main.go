package main

import (
	mysql "coupon-seckill-system/internal/infra/mysql"
	rds "coupon-seckill-system/internal/infra/redis"
	"coupon-seckill-system/internal/transport/http/handler"

	"github.com/gin-gonic/gin"
)

func main() {
	mysql.Connect()
	rds.ConnectRedis()
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()

	r.POST("/coupon/create", handler.CreateCoupon)
	r.POST("/seckill", handler.SeckillHandler)
	r.Run(":8080")
}
