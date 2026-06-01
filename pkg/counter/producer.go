package counter

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/segmentio/kafka-go"
	"github.com/spf13/viper"
	"go.uber.org/zap"
)

// Producer sends CounterEvents to Kafka for asynchronous aggregation.
//
// Configuration (config.yaml):
//
//	kafka:
//	  counter_topic: "counter_events"
type Producer struct {
	writer *kafka.Writer
}

// producer is the package-level singleton, initialised by InitProducer.
var producer *Producer

// InitProducer creates the package-level Kafka producer for counter events.
// Must be called once during application startup (after config is loaded).
func InitProducer() {
	brokers := viper.GetStringSlice("kafka.brokers")
	topic := viper.GetString("kafka.counter_topic")

	writer := &kafka.Writer{
		Addr:     kafka.TCP(brokers...),
		Topic:    topic,
		Balancer: &kafka.Hash{}, // hash by key (entityType:entityID) for per-entity ordering

		// Reliability: wait for all ISR replicas before acknowledging
		RequiredAcks: kafka.RequireAll,

		// Throughput: batch small messages together
		BatchSize:    16 * 1024,         // 16 KB
		BatchTimeout: 5 * time.Millisecond,
		Compression:  kafka.Lz4,
	}

	producer = &Producer{writer: writer}

	zap.L().Info("counter producer: initialised",
		zap.Strings("brokers", brokers),
		zap.String("topic", topic),
	)
}

// CloseProducer shuts down the package-level producer gracefully.
func CloseProducer() {
	if producer != nil {
		if err := producer.writer.Close(); err != nil {
			zap.L().Warn("counter producer: close error", zap.Error(err))
		}
	}
}

// Publish sends a counter event to Kafka.
//
// Uses "entityType:entityID" as the partition key so that all events for the
// same entity land in the same partition, preserving order for aggregation.
func (p *Producer) Publish(ctx context.Context, evt CounterEvent) error {
	data, err := json.Marshal(evt)
	if err != nil {
		return fmt.Errorf("marshal counter event: %w", err)
	}

	key := fmt.Sprintf("%s:%d", evt.EntityType, evt.EntityID)

	err = p.writer.WriteMessages(ctx, kafka.Message{
		Key:   []byte(key),
		Value: data,
	})
	if err != nil {
		return fmt.Errorf("write counter event: %w", err)
	}

	zap.L().Debug("counter event published",
		zap.String("entity_type", evt.EntityType),
		zap.Int64("entity_id", evt.EntityID),
		zap.String("metric", evt.Metric),
		zap.Int64("delta", evt.Delta),
	)

	return nil
}

// PublishPostLikeEvent is a convenience helper that builds a post-like event
// and publishes it through the package-level producer.
//
// Returns an error if the producer is not initialised or Kafka is unreachable;
// callers should log the error but NOT fail the user request (counts will be
// reconciled eventually).
func PublishPostLikeEvent(postID, uid int64, delta int64, idx int) error {
	if producer == nil {
		return fmt.Errorf("counter producer not initialised")
	}
	evt := NewPostLikeEvent(postID, uid, delta, idx)
	return producer.Publish(context.Background(), evt)
}
