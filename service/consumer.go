package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"ceddit/pkg/countint"
	redispkg "ceddit/repository/redis"

	"github.com/segmentio/kafka-go"
	"github.com/spf13/viper"
	"go.uber.org/zap"
)

// canalFlatMessage is the JSON format produced by Canal's FlatMessage adapter.
type canalFlatMessage struct {
	Data     []map[string]string `json:"data"`
	Database string              `json:"database"`
	Table    string              `json:"table"`
	Type     string              `json:"type"`
	IsDDL    bool                `json:"isDdl"`
}

// outboxPayload mirrors models.OutboxPayload (kept internal to the consumer).
type outboxPayload struct {
	EventID    string `json:"event_id"`
	FromUserID int64  `json:"from_user_id,string"`
	ToUserID   int64  `json:"to_user_id,string"`
	OccurredAt string `json:"occurred_at"`
}

// RunConsumer starts the Kafka consumer in a blocking loop.
// It connects to Kafka, reads Canal FlatMessages from the outbox topic,
// and projects FOLLOW/UNFOLLOW events into the follower table.
//
// The caller should invoke this as a goroutine:
//
//	go service.RunConsumer(ctx, db)
func RunConsumer(ctx context.Context, db *sql.DB) {
	brokers := viper.GetStringSlice("kafka.brokers")
	topic := viper.GetString("kafka.topic")
	groupID := viper.GetString("kafka.consumer_group")

	logger := zap.L()

	for {
		select {
		case <-ctx.Done():
			logger.Info("consumer: context done, exiting")
			return
		default:
		}

		if err := consumeLoop(ctx, db, brokers, topic, groupID, logger); err != nil {
			logger.Warn("consumer: loop exited, retrying in 5s", zap.Error(err))
		}

		// Wait before reconnect — Kafka or Canal may not be ready yet.
		select {
		case <-ctx.Done():
			return
		case <-time.After(5 * time.Second):
		}
	}
}

func consumeLoop(ctx context.Context, db *sql.DB, brokers []string, topic, groupID string, logger *zap.Logger) error {
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

	logger.Info("consumer: connected to Kafka",
		zap.Strings("brokers", brokers),
		zap.String("topic", topic),
	)

	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}

		msg, err := reader.ReadMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return fmt.Errorf("read message: %w", err)
		}

		if err := handleCanalMessage(db, msg.Value, logger); err != nil {
			logger.Error("consumer: handle message failed",
				zap.Int64("offset", msg.Offset),
				zap.Int("partition", msg.Partition),
				zap.Error(err),
			)
			// Do NOT return — continue processing.
			// Failed messages will be redelivered on restart.
		}
	}
}

func handleCanalMessage(db *sql.DB, raw []byte, logger *zap.Logger) error {
	var flat canalFlatMessage
	if err := json.Unmarshal(raw, &flat); err != nil {
		return fmt.Errorf("unmarshal flat message: %w", err)
	}

	if flat.IsDDL || flat.Table != "outbox" || len(flat.Data) == 0 {
		return nil
	}

	for _, row := range flat.Data {
		aggregateType := row["aggregate_type"]
		aggregateID := row["aggregate_id"]

		var payload outboxPayload
		if err := json.Unmarshal([]byte(row["payload"]), &payload); err != nil {
			return fmt.Errorf("unmarshal payload: %w", err)
		}

		switch aggregateType {
		case "FOLLOW":
			if err := projectFollow(db, aggregateID, &payload, logger); err != nil {
				return fmt.Errorf("project follow: %w", err)
			}
		case "UNFOLLOW":
			if err := projectUnfollow(db, aggregateID, &payload, logger); err != nil {
				return fmt.Errorf("project unfollow: %w", err)
			}
		default:
			logger.Warn("consumer: unknown aggregate type", zap.String("type", aggregateType))
		}
	}

	return nil
}

func projectFollow(db *sql.DB, aggregateID string, p *outboxPayload, logger *zap.Logger) error {
	occurredAt, err := time.Parse("2006-01-02T15:04:05.000Z", p.OccurredAt)
	if err != nil {
		occurredAt = time.Now()
	}

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
	logger.Debug("consumer: follow projected",
		zap.String("aggregate_id", aggregateID),
		zap.Int64("rows", rows),
	)

	// Update Redis CountInt: from_user's following_count +1, to_user's follower_count +1.
	// Best-effort — reconciler will fix any drift.
	if _, err := redispkg.IncrUserCount(p.FromUserID, countint.UserFollowingOffset, 1); err != nil {
		logger.Warn("consumer: incr following_count failed",
			zap.Int64("user", p.FromUserID), zap.Error(err))
	}
	if _, err := redispkg.IncrUserCount(p.ToUserID, countint.UserFollowerOffset, 1); err != nil {
		logger.Warn("consumer: incr follower_count failed",
			zap.Int64("user", p.ToUserID), zap.Error(err))
	}

	return nil
}

func projectUnfollow(db *sql.DB, aggregateID string, p *outboxPayload, logger *zap.Logger) error {
	result, err := db.Exec(
		`DELETE FROM follower WHERE id = ? AND (last_event_id IS NULL OR last_event_id < ?)`,
		aggregateID, p.EventID,
	)
	if err != nil {
		return fmt.Errorf("delete follower: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows > 0 {
		logger.Debug("consumer: unfollow projected", zap.String("aggregate_id", aggregateID))
	} else {
		logger.Debug("consumer: unfollow skipped (idempotent)", zap.String("aggregate_id", aggregateID))
	}

	// Update Redis CountInt: from_user's following_count -1, to_user's follower_count -1.
	// Best-effort — reconciler will fix any drift.
	if _, err := redispkg.IncrUserCount(p.FromUserID, countint.UserFollowingOffset, -1); err != nil {
		logger.Warn("consumer: decr following_count failed",
			zap.Int64("user", p.FromUserID), zap.Error(err))
	}
	if _, err := redispkg.IncrUserCount(p.ToUserID, countint.UserFollowerOffset, -1); err != nil {
		logger.Warn("consumer: decr follower_count failed",
			zap.Int64("user", p.ToUserID), zap.Error(err))
	}

	return nil
}
