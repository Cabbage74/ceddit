package main

import (
	"database/sql"
	"fmt"
	"os"
	"time"

	redispkg "ceddit/repository/redis"

	_ "github.com/go-sql-driver/mysql"
	"github.com/spf13/viper"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// reconciler is a periodic job that:
//  1. Counts follow relationships from the following source-of-truth table.
//  2. Counts followers from the follower projection table.
//  3. Writes correct counts to Redis CountInt binary blobs (SDS).
//  4. Removes stale rows from follower that have no corresponding following row.
//
// Uses the CountInt binary encoding: each user gets one Redis String key
// ("ucnt:{user_id}") with a 16-byte value: [following_count:8][follower_count:8].
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

	// ---- Redis (package-level client for CountInt operations) ----
	if err := redispkg.Init(); err != nil {
		logger.Fatal("init redis", zap.Error(err))
	}
	defer redispkg.Close()

	start := time.Now()
	logger.Info("reconciler started")

	// ---- 1. Reconcile following counts from source of truth ----
	logger.Info("reconciling following counts")
	followingRows, err := db.Query(
		`SELECT from_user_id, COUNT(*) AS cnt
		 FROM following
		 GROUP BY from_user_id`,
	)
	if err != nil {
		logger.Fatal("query following counts", zap.Error(err))
	}
	defer followingRows.Close()

	// Collect all user IDs and their following counts.
	userFollowing := make(map[int64]int64)
	for followingRows.Next() {
		var userID, count int64
		if err := followingRows.Scan(&userID, &count); err != nil {
			logger.Error("scan following row", zap.Error(err))
			continue
		}
		userFollowing[userID] = count
	}

	// ---- 2. Reconcile follower counts from projection ----
	logger.Info("reconciling follower counts")
	followerRows, err := db.Query(
		`SELECT to_user_id, COUNT(*) AS cnt
		 FROM follower
		 GROUP BY to_user_id`,
	)
	if err != nil {
		logger.Fatal("query follower counts", zap.Error(err))
	}
	defer followerRows.Close()

	userFollower := make(map[int64]int64)
	for followerRows.Next() {
		var userID, count int64
		if err := followerRows.Scan(&userID, &count); err != nil {
			logger.Error("scan follower row", zap.Error(err))
			continue
		}
		userFollower[userID] = count
	}

	// ---- 3. Write CountInt blobs to Redis ----
	// Merge both maps: every user that has at least one following or follower.
	allUsers := make(map[int64]struct{})
	for uid := range userFollowing {
		allUsers[uid] = struct{}{}
	}
	for uid := range userFollower {
		allUsers[uid] = struct{}{}
	}

	logger.Info("writing CountInt blobs", zap.Int("users", len(allUsers)))
	for uid := range allUsers {
		if err := redispkg.SetUserCounts(uid, userFollowing[uid], userFollower[uid]); err != nil {
			logger.Error("set user counts",
				zap.Int64("user", uid),
				zap.Error(err),
			)
		}
	}

	// ---- 4. Reconcile post like counts from vote ZSets ----
	// Query all published posts, then count upvotes from Redis ZSets.
	logger.Info("reconciling post like counts")
	postRows, err := db.Query(
		`SELECT id FROM post WHERE status = 'published'`,
	)
	if err != nil {
		logger.Error("query posts", zap.Error(err))
	} else {
		defer postRows.Close()
		var postCount int
		for postRows.Next() {
			var postID int64
			if err := postRows.Scan(&postID); err != nil {
				logger.Error("scan post row", zap.Error(err))
				continue
			}
			// Count upvotes (score == 1) from the vote ZSet.
			key := fmt.Sprintf("%s%d", redispkg.KeyPostVotedZsetPrefix, postID)
			likeCount := redispkg.GetRDB().ZCount(key, "1", "1").Val()
			if err := redispkg.SetPostLikeCount(postID, likeCount); err != nil {
				logger.Error("set post like count",
					zap.Int64("post", postID),
					zap.Error(err),
				)
			}
			postCount++
		}
		logger.Info("reconciled post like counts", zap.Int("posts", postCount))
	}

	// ---- 5. Clean up stale follower rows ----
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
