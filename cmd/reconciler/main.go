package main

import (
	"database/sql"
	"fmt"
	"os"
	"time"

	"github.com/go-redis/redis"
	_ "github.com/go-sql-driver/mysql"
	"github.com/spf13/viper"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// reconciler is a periodic job that:
//  1. Counts follow relationships from the following source-of-truth table.
//  2. Counts followers from the follower projection table.
//  3. Updates Redis hash keys with the correct counts.
//  4. Removes stale rows from follower that have no corresponding following row.
//
// Run via cron: */5 * * * * /usr/local/bin/ceddit-reconciler

func main() {
	logger, _ := zap.NewDevelopment(zap.AddStacktrace(zapcore.FatalLevel))
	defer logger.Sync()

	// ---- config ----
	viper.SetConfigFile("config.yaml")
	viper.AddConfigPath(".")
	if err := viper.ReadInConfig(); err != nil {
		fmt.Fprintf(os.Stderr, "read config: %v\n", err)
		os.Exit(1)
	}

	// ---- MySQL ----
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True",
		viper.GetString("mysql.user"),
		viper.GetString("mysql.password"),
		viper.GetString("mysql.host"),
		viper.GetInt("mysql.port"),
		viper.GetString("mysql.dbname"),
	)
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		logger.Fatal("open mysql", zap.Error(err))
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		logger.Fatal("ping mysql", zap.Error(err))
	}

	// ---- Redis ----
	rdb := redis.NewClient(&redis.Options{
		Addr: fmt.Sprintf("%s:%d",
			viper.GetString("redis.host"),
			viper.GetInt("redis.port"),
		),
		Password: viper.GetString("redis.password"),
		DB:       viper.GetInt("redis.db"),
	})
	defer rdb.Close()

	if _, err := rdb.Ping().Result(); err != nil {
		logger.Fatal("ping redis", zap.Error(err))
	}

	start := time.Now()
	logger.Info("reconciler started")

	// ---- 1. Reconcile following counts from source of truth ----
	logger.Info("reconciling following counts")
	rows, err := db.Query(
		`SELECT from_user_id, COUNT(*) AS cnt
		 FROM following
		 GROUP BY from_user_id`,
	)
	if err != nil {
		logger.Fatal("query following counts", zap.Error(err))
	}
	defer rows.Close()

	for rows.Next() {
		var userID, count int64
		if err := rows.Scan(&userID, &count); err != nil {
			logger.Error("scan following row", zap.Error(err))
			continue
		}
		key := fmt.Sprintf("user:%d:counts", userID)
		if err := rdb.HSet(key, "following_count", count).Err(); err != nil {
			logger.Error("redis hset following_count",
				zap.Int64("user", userID),
				zap.Error(err),
			)
		}
	}

	// ---- 2. Reconcile follower counts from projection ----
	logger.Info("reconciling follower counts")
	rows2, err := db.Query(
		`SELECT to_user_id, COUNT(*) AS cnt
		 FROM follower
		 GROUP BY to_user_id`,
	)
	if err != nil {
		logger.Fatal("query follower counts", zap.Error(err))
	}
	defer rows2.Close()

	for rows2.Next() {
		var userID, count int64
		if err := rows2.Scan(&userID, &count); err != nil {
			logger.Error("scan follower row", zap.Error(err))
			continue
		}
		key := fmt.Sprintf("user:%d:counts", userID)
		if err := rdb.HSet(key, "follower_count", count).Err(); err != nil {
			logger.Error("redis hset follower_count",
				zap.Int64("user", userID),
				zap.Error(err),
			)
		}
	}

	// ---- 3. Clean up stale follower rows ----
	// Remove follower rows where the corresponding following row has been deleted
	// (soft-delete gap: following row is gone but follower row lingers).
	logger.Info("cleaning stale follower rows")
	result, err := db.Exec(
		`DELETE f FROM follower f
		 LEFT JOIN following fw
		   ON fw.from_user_id = f.from_user_id
		  AND fw.to_user_id = f.to_user_id
		 WHERE fw.id IS NULL`,
	)
	if err != nil {
		logger.Error("clean stale followers", zap.Error(err))
	} else {
		n, _ := result.RowsAffected()
		if n > 0 {
			logger.Info("removed stale follower rows", zap.Int64("count", n))
		}
	}

	logger.Info("reconciler finished", zap.Duration("elapsed", time.Since(start)))
}
