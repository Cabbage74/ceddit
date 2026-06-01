package counter

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// ---- Redis key builders for the aggregation layer ----
//
// Key patterns:
//
//	Aggregation bucket:  agg:{schema}:{entityType}:{entityId}:{timeSlot}
//	Active bucket index: active:agg:{schema}

// AggKey builds the Redis Hash key for a counter aggregation bucket.
//
// Example: agg:v1:post:456:2024100114
func AggKey(entityType string, entityID int64, timeSlot string) string {
	return fmt.Sprintf("agg:%s:%s:%d:%s", SchemaID, entityType, entityID, timeSlot)
}

// ActiveAggSet returns the Redis Set key that tracks all aggregation buckets
// with pending (non-zero) deltas. The flush scheduler scans this set instead
// of using KEYS, avoiding a full key-space scan.
func ActiveAggSet() string {
	return fmt.Sprintf("active:agg:%s", SchemaID)
}

// TimeSlot formats a time as an hourly slot string.
//
// Example: 2024-10-01 14:35:00 → "2024100114"
func TimeSlot(t time.Time) string {
	return t.Format("2006010215")
}

// TimeSlotOf converts a unix timestamp to an hourly slot string.
func TimeSlotOf(ts int64) string {
	return TimeSlot(time.Unix(ts, 0))
}

// ParseAggKey extracts entityType and entityID from an aggregation bucket key.
//
// Expects: agg:{schema}:{entityType}:{entityId}:{timeSlot}
func ParseAggKey(aggKey string) (entityType string, entityID int64, err error) {
	parts := strings.Split(aggKey, ":")
	if len(parts) != 5 || parts[0] != "agg" {
		return "", 0, fmt.Errorf("invalid agg key: %s", aggKey)
	}
	// parts: ["agg", schema, entityType, entityID, timeSlot]
	entityType = parts[2]
	entityID, err = strconv.ParseInt(parts[3], 10, 64)
	if err != nil {
		return "", 0, fmt.Errorf("invalid agg key (bad entityID): %s", aggKey)
	}
	return
}
