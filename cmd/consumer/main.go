package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"ceddit/pkg/countint"
	redispkg "ceddit/repository/redis"

	_ "github.com/go-sql-driver/mysql"
	"github.com/segmentio/kafka-go"
	"github.com/spf13/viper"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// ---- Canal FlatMessage structures ----

type canalFlatMessage struct {
	Data     []map[string]string `json:"data"`
	Database string              `json:"database"`
	Table    string              `json:"table"`
	Type     string              `json:"type"` // INSERT, UPDATE, DELETE
	IsDDL    bool                `json:"isDdl"`
}

// outboxPayload mirrors models.OutboxPayload (duplicated to keep consumer standalone).
type outboxPayload struct {
	EventID    string `json:"event_id"`
	FromUserID int64  `json:"from_user_id,string"`
	ToUserID   int64  `json:"to_user_id,string"`
	OccurredAt string `json:"occurred_at"`
}

func main() {
	// ---- config ----
	viper.SetConfigFile("config.yaml")
	viper.AddConfigPath(".")
	if err := viper.ReadInConfig(); err != nil {
		fmt.Fprintf(os.Stderr, "read config: %v\n", err)
		os.Exit(1)
	}

	// ---- logger ----
	logger, _ := zap.NewDevelopment(zap.AddStacktrace(zapcore.FatalLevel))
	defer logger.Sync()

	// ---- MySQL ----
	mysqlDSN := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True",
		viper.GetString("mysql.user"),
		viper.GetString("mysql.password"),
		viper.GetString("mysql.host"),
		viper.GetInt("mysql.port"),
		viper.GetString("mysql.dbname"),
	)
	db, err := sql.Open("mysql", mysqlDSN)
	if err != nil {
		logger.Fatal("open mysql", zap.Error(err))
	}
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	defer db.Close()

	if err := db.Ping(); err != nil {
		logger.Fatal("ping mysql", zap.Error(err))
	}
	logger.Info("connected to MySQL")

	// ---- Redis (for CountInt updates) ----
	if err := redispkg.Init(); err != nil {
		logger.Fatal("init redis", zap.Error(err))
	}
	defer redispkg.Close()

	if err := redispkg.InitCountScripts(); err != nil {
		logger.Fatal("init count scripts", zap.Error(err))
	}
	logger.Info("connected to Redis")

	// ---- Kafka reader ----
	brokers := viper.GetStringSlice("kafka.brokers")
	topic := viper.GetString("kafka.topic")
	groupID := viper.GetString("kafka.consumer_group")

	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:     brokers,
		Topic:       topic,
		GroupID:     groupID,
		StartOffset: kafka.FirstOffset,
		MinBytes:    1,
		MaxBytes:    10e6,
		MaxWait:     500 * time.Millisecond,
	})
	defer reader.Close()
	logger.Info("connected to Kafka", zap.Strings("brokers", brokers), zap.String("topic", topic))

	// ---- graceful shutdown ----
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		logger.Info("shutting down...")
		cancel()
	}()

	// ---- consume loop ----
	for {
		select {
		case <-ctx.Done():
			logger.Info("consumer stopped")
			return
		default:
		}

		msg, err := reader.ReadMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				continue
			}
			logger.Error("read message", zap.Error(err))
			time.Sleep(time.Second)
			continue
		}

		logger.Debug("received message",
			zap.Int64("offset", msg.Offset),
			zap.Int("partition", msg.Partition),
		)

		if err := handleMessage(db, msg.Value, logger); err != nil {
			logger.Error("handle message failed",
				zap.Int64("offset", msg.Offset),
				zap.Error(err),
			)
			// Continue — do not commit offset for failed messages.
			// The message will be redelivered on restart.
			continue
		}
	}
}

func handleMessage(db *sql.DB, raw []byte, logger *zap.Logger) error {
	var flat canalFlatMessage
	if err := json.Unmarshal(raw, &flat); err != nil {
		return fmt.Errorf("unmarshal flat message: %w", err)
	}

	// Ignore DDL, non-outbox tables, and non-INSERT operations.
	// (We only have INSERT into outbox — Canal picks up the INSERT.)
	if flat.IsDDL || flat.Table != "outbox" {
		return nil
	}
	if len(flat.Data) == 0 {
		return nil
	}

	for _, row := range flat.Data {
		aggregateType := row["aggregate_type"]
		aggregateID := row["aggregate_id"]
		payloadRaw := row["payload"]

		var payload outboxPayload
		if err := json.Unmarshal([]byte(payloadRaw), &payload); err != nil {
			return fmt.Errorf("unmarshal payload: %w", err)
		}

		switch aggregateType {
		case "FOLLOW":
			if err := handleFollow(db, aggregateID, &payload, logger); err != nil {
				return fmt.Errorf("handle follow: %w", err)
			}
		case "UNFOLLOW":
			if err := handleUnfollow(db, aggregateID, &payload, logger); err != nil {
				return fmt.Errorf("handle unfollow: %w", err)
			}
		default:
			logger.Warn("unknown aggregate type", zap.String("type", aggregateType))
		}
	}

	return nil
}

func handleFollow(db *sql.DB, aggregateID string, p *outboxPayload, logger *zap.Logger) error {
	occurredAt, err := time.Parse("2006-01-02T15:04:05.000Z", p.OccurredAt)
	if err != nil {
		occurredAt = time.Now()
	}

	// INSERT ... ON DUPLICATE KEY UPDATE with idempotent last_event_id check.
	// Only update if the incoming event is newer (by event_id comparison).
	// Since event_id is a snowflake, lexicographic order ≈ temporal order.
	result, err := db.Exec(
		`INSERT INTO follower (id, to_user_id, from_user_id, created_at, updated_at, last_event_id)
		 VALUES (?, ?, ?, ?, ?, ?)
		 ON DUPLICATE KEY UPDATE
		     updated_at = IF(last_event_id IS NULL OR last_event_id < VALUES(last_event_id), VALUES(updated_at), updated_at),
		     last_event_id = IF(last_event_id IS NULL OR last_event_id < VALUES(last_event_id), VALUES(last_event_id), last_event_id)`,
		aggregateID, p.ToUserID, p.FromUserID, occurredAt, occurredAt, p.EventID,
	)
	if err != nil {
		return fmt.Errorf("insert follower: %w", err)
	}

	rows, _ := result.RowsAffected()
	logger.Debug("follow applied",
		zap.String("aggregate_id", aggregateID),
		zap.Int64("from", p.FromUserID),
		zap.Int64("to", p.ToUserID),
		zap.Int64("rows_affected", rows),
	)

	// Update Redis CountInt: from_user's following_count +1, to_user's follower_count +1.
	if _, err := redispkg.IncrUserCount(p.FromUserID, countint.UserFollowingOffset, 1); err != nil {
		logger.Warn("incr following_count failed",
			zap.Int64("user", p.FromUserID), zap.Error(err))
	}
	if _, err := redispkg.IncrUserCount(p.ToUserID, countint.UserFollowerOffset, 1); err != nil {
		logger.Warn("incr follower_count failed",
			zap.Int64("user", p.ToUserID), zap.Error(err))
	}

	return nil
}

func handleUnfollow(db *sql.DB, aggregateID string, p *outboxPayload, logger *zap.Logger) error {
	result, err := db.Exec(
		`DELETE FROM follower WHERE id = ? AND (last_event_id IS NULL OR last_event_id < ?)`,
		aggregateID, p.EventID,
	)
	if err != nil {
		return fmt.Errorf("delete follower: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows > 0 {
		logger.Debug("unfollow applied",
			zap.String("aggregate_id", aggregateID),
			zap.Int64("from", p.FromUserID),
			zap.Int64("to", p.ToUserID),
		)
	} else {
		// Row not found or already processed — idempotent skip.
		logger.Debug("unfollow skipped: no matching follower row",
			zap.String("aggregate_id", aggregateID),
		)
	}

	// Update Redis CountInt: from_user's following_count -1, to_user's follower_count -1.
	if _, err := redispkg.IncrUserCount(p.FromUserID, countint.UserFollowingOffset, -1); err != nil {
		logger.Warn("decr following_count failed",
			zap.Int64("user", p.FromUserID), zap.Error(err))
	}
	if _, err := redispkg.IncrUserCount(p.ToUserID, countint.UserFollowerOffset, -1); err != nil {
		logger.Warn("decr follower_count failed",
			zap.Int64("user", p.ToUserID), zap.Error(err))
	}

	return nil
}
