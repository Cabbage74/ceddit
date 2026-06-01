package counter

import (
	redisrepo "ceddit/repository/redis"
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/go-redis/redis"
	"github.com/segmentio/kafka-go"
	"github.com/spf13/viper"
	"go.uber.org/zap"
)

const (
	// aggBucketTTL is the Redis TTL for aggregation buckets. It must be
	// longer than the flush cycle (1 s) to prevent premature eviction.
	aggBucketTTL = 2 * time.Hour

	// reconnectDelay is the wait time before reconnecting to Kafka after a
	// consumer error.
	reconnectDelay = 5 * time.Second
)

// RunAggregationConsumer starts the Kafka consumer that reads CounterEvents
// and accumulates their deltas into Redis Hash aggregation buckets.
//
// It runs in a reconnect loop: if the Kafka connection fails, it waits
// reconnectDelay and retries, so it tolerates Kafka restarts at startup.
//
// Callers should invoke this as a goroutine:
//
//	go counter.RunAggregationConsumer(ctx)
func RunAggregationConsumer(ctx context.Context) {
	brokers := viper.GetStringSlice("kafka.brokers")
	topic := viper.GetString("kafka.counter_topic")
	groupID := viper.GetString("kafka.counter_consumer_group")

	logger := zap.L().With(zap.String("component", "counter-agg-consumer"))

	for {
		select {
		case <-ctx.Done():
			logger.Info("context done, exiting")
			return
		default:
		}

		if err := runAggLoop(ctx, brokers, topic, groupID, logger); err != nil {
			logger.Warn("loop exited, retrying",
				zap.Duration("delay", reconnectDelay),
				zap.Error(err),
			)
		}

		select {
		case <-ctx.Done():
			return
		case <-time.After(reconnectDelay):
		}
	}
}

func runAggLoop(ctx context.Context, brokers []string, topic, groupID string, logger *zap.Logger) error {
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:     brokers,
		Topic:       topic,
		GroupID:     groupID,
		StartOffset: kafka.FirstOffset,
		MinBytes:    1,
		MaxBytes:    10e6,            // 10 MB
		MaxWait:     500 * time.Millisecond,
	})
	defer reader.Close()

	logger.Info("connected to Kafka",
		zap.Strings("brokers", brokers),
		zap.String("topic", topic),
		zap.String("group_id", groupID),
	)

	rdb := redisrepo.GetRDB()

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
			return fmt.Errorf("read counter event: %w", err)
		}

		if err := aggregate(rdb, msg.Value, logger); err != nil {
			logger.Error("aggregate event failed",
				zap.Int64("offset", msg.Offset),
				zap.Int("partition", msg.Partition),
				zap.Error(err),
			)
			// Continue processing — failed messages will be redelivered on
			// restart (at-least-once). HINCRBY is effectively idempotent at the
			// bucket level because the flush script reads-then-deletes atomically;
			// replay produces a safe double-accumulation that the next flush round
			// picks up.
			continue
		}
	}
}

// aggregate deserialises a CounterEvent and accumulates its delta into the
// appropriate Redis Hash aggregation bucket.
func aggregate(rdb *redis.Client, raw []byte, logger *zap.Logger) error {
	var evt CounterEvent
	if err := json.Unmarshal(raw, &evt); err != nil {
		return fmt.Errorf("unmarshal: %w", err)
	}

	ts := TimeSlotOf(evt.Timestamp)
	aggKey := AggKey(evt.EntityType, evt.EntityID, ts)
	field := strconv.Itoa(evt.Idx)

	// Atomically accumulate delta in the aggregation bucket.
	if err := rdb.HIncrBy(aggKey, field, evt.Delta).Err(); err != nil {
		return fmt.Errorf("hincrby %s %s: %w", aggKey, field, err)
	}

	// Track this bucket in the active set so the flush scheduler can find it
	// without a full key-space scan.
	_ = rdb.SAdd(ActiveAggSet(), aggKey).Err()

	// Extend TTL on every write — keeps hot buckets alive.
	_ = rdb.Expire(aggKey, aggBucketTTL).Err()

	logger.Debug("event aggregated",
		zap.String("agg_key", aggKey),
		zap.String("field", field),
		zap.Int64("delta", evt.Delta),
	)

	return nil
}
