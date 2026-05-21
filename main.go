package main

import (
	"ceddit/logger"
	"ceddit/repository/mysql"
	"ceddit/repository/redis"
	"ceddit/routes"
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
	// 1. Load configs
	if err := settings.Init(); err != nil {
		fmt.Printf("Failed to load configs, err: %v\n", err)
		return
	}

	// 2. Initialize Logger
	if err := logger.Init(); err != nil {
		fmt.Printf("Failed to initialize logger, err: %v\n", err)
		return
	}
	defer zap.L().Sync()

	// 3. Initialize MySQL
	if err := mysql.Init(); err != nil {
		fmt.Printf("Failed to initialize mysql, err: %v\n", err)
		return
	}
	defer mysql.Close()

	// 4. Initialize Redis
	if err := redis.Init(); err != nil {
		fmt.Printf("Failed to initialize redis, err: %v\n", err)
		return
	}
	defer redis.Close()

	// 5. Register Routes
	r := routes.Setup()

	// 6. Start server and exit gracefully
	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", viper.GetInt("app.port")),
		Handler: r,
	}

	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			zap.L().Fatal("Listen: %s\n", zap.Error(err))
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
