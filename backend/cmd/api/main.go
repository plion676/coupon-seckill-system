package main

import (
	"fmt"

	mysql "coupon-seckill-system/internal/infra/mysql"
	rds "coupon-seckill-system/internal/infra/redis"
	"coupon-seckill-system/internal/transport/http/handler"

	"github.com/gin-gonic/gin"
)

func main() {
	mysql.Connect()
	rds.ConnectRedis()
	fmt.Println(mysql.DB)

	r := gin.Default()

	r.POST("/coupon/create", handler.CreateCoupon)
	r.POST("/seckill", handler.SeckillHandler)
	r.Run(":8080")
}
