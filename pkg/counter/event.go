// Package counter implements the Kafka-backed async write aggregation
// pipeline for high-concurrency counting (likes, favs, etc.).
//
// Architecture:
//
//	Producer (sync)  →  Kafka  →  Aggregation Consumer  →  Redis Hash buckets
//	                                                    →  Flush Scheduler  →  Redis SDS (CountInt)
//
// The synchronous path records user fact-state immediately (e.g. vote ZSet).
// The asynchronous path produces a CounterEvent to Kafka, which is aggregated
// into per-entity-per-hour Redis Hash buckets. A periodic flush scheduler
// reads the accumulated deltas and atomically applies them to the SDS
// (Simple Dynamic String) binary counters, then deletes the processed fields.
//
// This reduces write amplification 10-1000x by folding many fine-grained
// increments into batch writes while keeping user-visible state instantaneous.
package counter

import (
	"fmt"
	"time"
)

// SchemaID is the current aggregation schema version, embedded in Redis keys
// so that future schema changes can coexist without migration.
const SchemaID = "v1"

// CounterEvent represents a single counting operation (like, unlike, fav, etc.).
// It is serialised to JSON and published to Kafka for asynchronous aggregation.
type CounterEvent struct {
	EntityType    string `json:"entity_type"`    // e.g. "post"
	EntityID      int64  `json:"entity_id"`      // the post / user / etc. ID
	Metric        string `json:"metric"`          // e.g. "like"
	Idx           int    `json:"idx"`             // byte offset inside the SDS blob
	UID           int64  `json:"uid"`             // user who performed the action
	Delta         int64  `json:"delta"`           // +1 or -1
	Timestamp     int64  `json:"timestamp"`       // unix seconds
	IdempotentKey string `json:"idempotent_key"`  // dedup key: "{uid}_{etype}_{eid}_{metric}_{ts}"
	Version       int64  `json:"version"`         // schema version number
}

// NewPostLikeEvent creates a CounterEvent for a post like / unlike operation.
//
// idx should be countint.PostLikeOffset for the like_count field inside the
// post SDS blob.
func NewPostLikeEvent(postID, uid int64, delta int64, idx int) CounterEvent {
	ts := time.Now().Unix()
	return CounterEvent{
		EntityType:    "post",
		EntityID:      postID,
		Metric:        "like",
		Idx:           idx,
		UID:           uid,
		Delta:         delta,
		Timestamp:     ts,
		IdempotentKey: fmt.Sprintf("%d_post_%d_like_%d", uid, postID, ts),
		Version:       1,
	}
}
