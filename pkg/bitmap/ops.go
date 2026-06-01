package bitmap

import "github.com/go-redis/redis"

// Toggle atomically sets or clears a user's bit in the given metric bitmap.
//
// rdb must be a connected Redis client.
//
// Returns true if the bit was actually changed (first toggle), false if it
// was already in the desired state (idempotent replay).
//
// The caller should only produce a CounterEvent when changed == true.
func Toggle(rdb *redis.Client, metric, entityType string, entityID, userID int64, set bool) (ToggleResult, error) {
	chunk := ChunkOf(userID)
	bit := BitOf(userID)
	key := Key(metric, entityType, entityID, chunk)

	action := "clear"
	if set {
		action = "set"
	}

	res, err := ToggleScript.Run(rdb, []string{key}, bit, action).Result()
	if err != nil {
		return ToggleResult{}, err
	}

	// Lua returns a table {changed, old_value}, decoded as []interface{}.
	arr, ok := res.([]interface{})
	if !ok || len(arr) < 2 {
		return ToggleResult{}, nil
	}

	changed, _ := arr[0].(int64)
	oldVal, _ := arr[1].(int64)

	return ToggleResult{
		Changed:  changed == 1,
		OldValue: oldVal,
	}, nil
}

// IsSet returns whether the given user's bit is set (1) in the metric bitmap.
//
// This is the synchronous read path for "has the user liked / faved this
// entity?" — a single GETBIT, suitable for millisecond-level UI checks.
func IsSet(rdb *redis.Client, metric, entityType string, entityID, userID int64) (bool, error) {
	chunk := ChunkOf(userID)
	bit := BitOf(userID)
	key := Key(metric, entityType, entityID, chunk)

	val, err := rdb.GetBit(key, bit).Result()
	if err != nil {
		return false, err
	}
	return val == 1, nil
}

// CountShards counts the total number of set bits across all bitmap shards
// for a given (metric, entityType, entityID).
//
// Used during rebuild to derive the true like/fav count from the fact layer.
//
// NOTE: uses KEYS to enumerate shards. For high-throughput production use,
// maintain an index set (e.g. "bm:index:{metric}:{etype}:{eid}") instead.
func CountShards(rdb *redis.Client, metric, entityType string, entityID int64) (int64, error) {
	pattern := KeyPattern(metric, entityType, entityID)
	keys, err := rdb.Keys(pattern).Result()
	if err != nil {
		return 0, err
	}
	if len(keys) == 0 {
		return 0, nil
	}

	// Pipeline BITCOUNT for all shards.
	pipe := rdb.Pipeline()
	cmds := make([]*redis.IntCmd, len(keys))
	for i, k := range keys {
		cmds[i] = pipe.BitCount(k, nil)
	}
	_, _ = pipe.Exec()

	var total int64
	for _, cmd := range cmds {
		count, err := cmd.Result()
		if err != nil {
			continue
		}
		total += count
	}
	return total, nil
}
