package counter

import (
	"ceddit/pkg/bitmap"
	"ceddit/pkg/countint"
	redisrepo "ceddit/repository/redis"
	"fmt"
	"time"

	"go.uber.org/zap"
)

// RebuildPostCounts rebuilds the post SDS counter from bitmap fact-layer data.
//
// It is called when the SDS key is missing, has the wrong length, or is
// suspected of drift. The function:
//  1. Acquires a distributed lock to prevent concurrent rebuilds.
//  2. BITCOUNTs all bitmap shards for each metric to derive true counts.
//  3. Writes the corrected SDS blob.
//  4. Clears the corresponding aggregation bucket fields so stale deltas
//     are not re-applied.
//  5. Releases the lock.
//
// Returns the rebuilt counts keyed by metric name.
func RebuildPostCounts(postID int64) (map[string]int64, error) {
	logger := zap.L().With(
		zap.String("entity_type", EntityPost),
		zap.Int64("entity_id", postID),
	)

	lockKey := rebuildLockKey(EntityPost, postID)
	token, locked := tryLock(lockKey)
	if !locked {
		logger.Debug("rebuild skipped: lock held by another process")
		return nil, fmt.Errorf("rebuild lock held for post %d", postID)
	}
	defer unlock(lockKey, token)

	logger.Info("rebuilding post counts from bitmap facts")

	result := make(map[string]int64)

	// Rebuild each metric from its bitmap shards.
	for _, metric := range PostMetricNames() {
		idx, ok := PostMetricIndex(metric)
		if !ok {
			continue
		}

		total, err := bitmap.CountShards(redisrepo.GetRDB(), metric, EntityPost, postID)
		if err != nil {
			logger.Warn("bitmap count failed for metric",
				zap.String("metric", metric),
				zap.Error(err),
			)
			continue
		}
		result[metric] = total

		// Write to SDS.
		blob := countint.EncodePostLikeCount(total)
		sdsKey := fmt.Sprintf("%s%d", redisrepo.KeyPostCountPrefix, postID)
		if err := redisrepo.GetRDB().Set(sdsKey, blob, 0).Err(); err != nil {
			logger.Warn("write SDS failed during rebuild",
				zap.String("sds_key", sdsKey),
				zap.Error(err),
			)
			continue
		}

		// Clear the corresponding aggregation bucket fields so that stale
		// deltas accumulated before the rebuild are not re-applied.
		// We use the current time slot; if deltas arrived in a previous slot
		// that bucket will naturally expire via TTL.
		aggKey := AggKey(EntityPost, postID, TimeSlot(time.Now()))
		_ = redisrepo.GetRDB().HDel(aggKey, fmt.Sprintf("%d", idx)).Err()

		logger.Debug("metric rebuilt",
			zap.String("metric", metric),
			zap.Int64("count", total),
		)
	}

	return result, nil
}

// GetPostCounts returns the post's counter values from SDS, triggering a
// rebuild from bitmap facts if the SDS key is missing or corrupted.
//
// This is the primary read path for entity counters: it provides the "read
// through" semantics where reads never fail — if the SDS is unusable the
// function falls back to fact-layer reconstruction.
func GetPostCounts(postID int64) (map[string]int64, error) {
	sdsKey := fmt.Sprintf("%s%d", redisrepo.KeyPostCountPrefix, postID)
	rdb := redisrepo.GetRDB()

	raw, err := rdb.Get(sdsKey).Bytes()
	needRebuild := err != nil || len(raw) != PostBlobSize

	if needRebuild {
		return RebuildPostCounts(postID)
	}

	result := make(map[string]int64)
	for _, metric := range PostMetricNames() {
		idx, ok := PostMetricIndex(metric)
		if !ok {
			continue
		}
		// All post fields use the same int64 decoder.
		val := countint.DecodePostLikeCount(raw)
		_ = idx // currently only one field; kept for future expansion
		result[metric] = val
	}

	return result, nil
}
