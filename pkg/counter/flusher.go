package counter

import (
	"ceddit/pkg/countint"
	redisrepo "ceddit/repository/redis"
	"context"
	"fmt"
	"strconv"
	"time"

	"go.uber.org/zap"
)

// RunFlushScheduler periodically scans active aggregation buckets, reads the
// accumulated deltas, atomically applies them to the target CountInt SDS keys,
// and deletes the processed fields.
//
// It runs with a fixed delay of 1 second, giving second-level eventual
// consistency for entity total counts (like count on a post).
//
// Callers should invoke this as a goroutine:
//
//	go counter.RunFlushScheduler(ctx)
func RunFlushScheduler(ctx context.Context) {
	logger := zap.L().With(zap.String("component", "counter-flusher"))
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	logger.Info("started (1 s interval)")

	for {
		select {
		case <-ctx.Done():
			logger.Info("stopped")
			return
		case <-ticker.C:
			flushOnce(logger)
		}
	}
}

// flushOnce performs a single sweep of all active aggregation buckets.
func flushOnce(logger *zap.Logger) {
	rdb := redisrepo.GetRDB()

	activeSet := ActiveAggSet()
	members, err := rdb.SMembers(activeSet).Result()
	if err != nil {
		// Redis nil means the set doesn't exist yet — not an error.
		if err.Error() != "redis: nil" {
			logger.Warn("failed to read active set", zap.Error(err))
		}
		return
	}

	if len(members) == 0 {
		return
	}

	var flushed int
	for _, aggKey := range members {
		entityType, entityID, err := ParseAggKey(aggKey)
		if err != nil {
			logger.Warn("invalid agg key, removing from active set",
				zap.String("key", aggKey), zap.Error(err))
			_ = rdb.SRem(activeSet, aggKey).Err()
			continue
		}

		entries, err := rdb.HGetAll(aggKey).Result()
		if err != nil {
			logger.Warn("failed to read agg bucket",
				zap.String("key", aggKey), zap.Error(err))
			continue
		}

		if len(entries) == 0 {
			// Stale entry — bucket was cleaned up externally.
			_ = rdb.SRem(activeSet, aggKey).Err()
			_ = rdb.Del(aggKey).Err()
			continue
		}

		sdsKey, blobSize := sdsKeyAndSize(entityType, entityID)

		for field, deltaStr := range entries {
			delta, err := strconv.ParseInt(deltaStr, 10, 64)
			if err != nil || delta == 0 {
				// Remove zero / invalid fields to keep the bucket clean.
				_ = rdb.HDel(aggKey, field).Err()
				continue
			}

			idx, err := strconv.Atoi(field)
			if err != nil {
				continue
			}

			// Atomically: read delta → apply to SDS → delete field.
			// If the bucket becomes empty the script also removes it from
			// the active set and deletes the key.
			_, err = FlushScript.Run(rdb,
				[]string{aggKey, sdsKey},
				field, idx, blobSize, activeSet,
			).Result()
			if err != nil {
				// The script returns an error when the field is missing
				// (already flushed by a previous retry or concurrent round).
				// This is safe: we simply skip and keep going. The field
				// will be cleaned up on the next sweep.
				logger.Debug("flush script skipped",
					zap.String("agg_key", aggKey),
					zap.String("field", field),
					zap.Error(err),
				)
				continue
			}
			flushed++
		}
	}

	if flushed > 0 {
		logger.Debug("flush sweep done",
			zap.Int("buckets_scanned", len(members)),
			zap.Int("fields_flushed", flushed),
		)
	}
}

// sdsKeyAndSize returns the CountInt SDS Redis key and blob size for a given
// entity type. This is the only place where the counter package needs to know
// about specific entity SDS layouts — new entity types only need a new case
// here.
func sdsKeyAndSize(entityType string, entityID int64) (key string, size int) {
	switch entityType {
	case "post":
		return fmt.Sprintf("%s%d", redisrepo.KeyPostCountPrefix, entityID), countint.PostBlobSize
	default:
		return "", 0
	}
}
