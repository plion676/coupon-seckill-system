package main

import (
	mysql "coupon-seckill-system/internal/infra/mysql"
	rds "coupon-seckill-system/internal/infra/redis"
	"coupon-seckill-system/internal/middleware"
	"coupon-seckill-system/internal/service"
	"coupon-seckill-system/internal/transport/http/handler"
	"net/http"
	_ "net/http/pprof"

	"github.com/gin-gonic/gin"
)

func main() {
	mysql.Connect()
	rds.ConnectRedis()
	service.StartAsyncWriter()

	// 启动 pprof 监听端口 6060
	go func() {
		http.ListenAndServe("0.0.0.0:6060", nil)
	}()

	gin.SetMode(gin.ReleaseMode)
	r := gin.New()

	rateLimitManager := middleware.NewRateLimitManager(rds.RDB)

	seckillGroup := r.Group("/seckill")
	seckillGroup.Use(rateLimitManager.GlobalLimit())
	seckillGroup.Use(rateLimitManager.UserLimit())

	r.POST("/coupon/create", handler.CreateCoupon)
	seckillGroup.POST("", handler.SeckillHandler)
	r.Run(":8080")
}
