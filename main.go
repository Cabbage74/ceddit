package main

import (
	"ceddit/logger"
	"ceddit/pkg/cos"
	"ceddit/pkg/deepseek"
	"ceddit/pkg/snowflake"
	"ceddit/repository/mysql"
	"ceddit/repository/redis"
	"ceddit/routes"
	"ceddit/service"
	"ceddit/settings"
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/viper"
	"go.uber.org/zap"
)

func main() {
	if err := settings.Init(); err != nil {
		fmt.Printf("Failed to load configs, err: %v\n", err)
		return
	}

	if err := logger.Init(); err != nil {
		fmt.Printf("Failed to initialize logger, err: %v\n", err)
		return
	}
	defer zap.L().Sync()

	if err := mysql.Init(); err != nil {
		fmt.Printf("Failed to initialize mysql, err: %v\n", err)
		return
	}
	defer mysql.Close()

	if err := redis.Init(); err != nil {
		fmt.Printf("Failed to initialize redis, err: %v\n", err)
		return
	}
	defer redis.Close()

	// Pre-load CountInt Lua scripts for atomic binary-counter operations.
	if err := redis.InitCountScripts(); err != nil {
		fmt.Printf("Failed to load count scripts, err: %v\n", err)
		return
	}

	if err := snowflake.Init(viper.GetString("app.start_time"), viper.GetInt64("app.machine_id")); err != nil {
		fmt.Printf("Failed to initialize snowflake, err: %v\n", err)
		return
	}

	if err := cos.Init(); err != nil {
		fmt.Printf("Failed to initialize COS, err: %v\n", err)
		return
	}

	deepseek.Init()

	// Start Kafka consumer in background (projects follower table from outbox events).
	// Reconnects automatically if Kafka or Canal is not ready yet.
	go service.RunConsumer(context.Background(), mysql.DB().DB)

	r := routes.Setup()

	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", viper.GetInt("app.port")),
		Handler: r,
	}

	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			zap.L().Fatal("", zap.Error(err))
		}
	}()

	quit := make(chan os.Signal, 1)

	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		zap.L().Fatal("Server shundown: ", zap.Error(err))
	}

}
