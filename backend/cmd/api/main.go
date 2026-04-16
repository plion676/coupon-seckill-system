package main

import (
	"context"
	mysql "coupon-seckill-system/internal/infra/mysql"
	rds "coupon-seckill-system/internal/infra/redis"
	"coupon-seckill-system/internal/middleware"
	"coupon-seckill-system/internal/pkg/logger"
	"coupon-seckill-system/internal/transport/http/handler"
	"log/slog"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
)

func main() {
	logger.Init("api")

	mysql.Connect()
	rds.ConnectRedis()
	if mysql.DB == nil {
		slog.Error("mysql init failed", "module", "api_main")
		os.Exit(1)
	}
	if rds.RDB == nil {
		slog.Error("redis init failed", "module", "api_main")
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pprofServer := &http.Server{
		Addr:              "0.0.0.0:6060",
		ReadHeaderTimeout: time.Second,
	}

	// 启动 pprof 监听端口 6060
	go func() {
		if err := pprofServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("pprof server exited", "module", "api_main", "addr", "0.0.0.0:6060", "err", err)
		}
	}()

	gin.SetMode(gin.ReleaseMode)
	r := gin.New()

	rateLimitManager := middleware.NewRateLimitManager(rds.RDB)

	seckillGroup := r.Group("/seckill")
	seckillGroup.Use(rateLimitManager.GlobalLimit())
	seckillGroup.Use(rateLimitManager.UserLimit())

	r.POST("/coupon/create", handler.CreateCoupon)
	seckillGroup.POST("", handler.SeckillHandler)

	apiServer := &http.Server{
		Addr:              ":8080",
		Handler:           r,
		ReadHeaderTimeout: 2 * time.Second,
	}

	go func() {
		<-ctx.Done()

		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		slog.Info("api server shutting down", "module", "api_main")
		if err := apiServer.Shutdown(shutdownCtx); err != nil {
			slog.Error("api server shutdown failed", "module", "api_main", "err", err)
		}
		if err := pprofServer.Shutdown(shutdownCtx); err != nil {
			slog.Error("pprof server shutdown failed", "module", "api_main", "addr", "0.0.0.0:6060", "err", err)
		}
	}()

	slog.Info("api server started", "module", "api_main", "addr", ":8080")
	if err := apiServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		slog.Error("api server exited", "module", "api_main", "addr", ":8080", "err", err)
		os.Exit(1)
	}
	slog.Info("api server stopped", "module", "api_main")
}
