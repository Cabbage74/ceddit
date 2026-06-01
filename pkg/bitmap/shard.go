// Package bitmap implements Redis bitmap shards for tracking per-user
// boolean facts (liked, faved, etc.) with fixed-size chunking to avoid
// hot keys and oversized values.
//
// Each shard holds up to ChunkSize bits (4 KB of storage). A user ID is
// mapped to (chunk, bit) via integer division, spreading all users of a
// given entity across multiple Redis keys.
//
// Key layout:
//
//	bm:{metric}:{entityType}:{entityId}:{chunk}
//
// Example: bm:like:post:456:0 holds bits [0, 32767] for users who liked
// post 456.
package bitmap

// ChunkSize is the number of bits per bitmap shard.
// 32 768 bits = 4 096 bytes per shard.
const ChunkSize = 32_768

// ChunkOf returns the shard index for the given user ID.
func ChunkOf(userID int64) int64 {
	return userID / ChunkSize
}

// BitOf returns the bit offset within a shard for the given user ID.
func BitOf(userID int64) int64 {
	return userID % ChunkSize
}
