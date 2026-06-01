package bitmap

import "fmt"

// Key builds a Redis bitmap shard key.
//
// Pattern: bm:{metric}:{entityType}:{entityId}:{chunk}
func Key(metric, entityType string, entityID, chunk int64) string {
	return fmt.Sprintf("bm:%s:%s:%d:%d", metric, entityType, entityID, chunk)
}

// KeyPattern returns a glob pattern that matches all bitmap shards for a
// given (metric, entityType, entityID) combination.
//
// Used during rebuild to enumerate shards for BITCOUNT.
func KeyPattern(metric, entityType string, entityID int64) string {
	return fmt.Sprintf("bm:%s:%s:%d:*", metric, entityType, entityID)
}
