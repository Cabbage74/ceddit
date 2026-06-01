package redis

import (
	"ceddit/pkg/bitmap"
	"time"

	"github.com/go-redis/redis"
)

const (
	oneWeekInSeconds = 7 * 24 * 3600
)

// CanVote returns true if the post is still within its 1-week voting window.
func CanVote(postID string) bool {
	postTime := rdb.ZScore(KeyPostTimeZSet, postID).Val()
	if float64(time.Now().Unix())-postTime > oneWeekInSeconds {
		return false
	}
	return true
}

// ToggleLike atomically sets or clears the like bit for a user on a post.
//
// It uses the bitmap shard layer as the fact store: each user gets one bit
// in a fixed-size shard (32K bits), spreading hot entities across multiple
// Redis keys to avoid hot-key pressure.
//
// Returns:
//   - changed: true if the bit was actually flipped (first toggle)
//   - liked:   the new state after the operation
func ToggleLike(postID, userID int64, like bool) (changed bool, liked bool, err error) {
	metric := "like"
	entityType := "post"

	result, err := bitmap.Toggle(rdb, metric, entityType, postID, userID, like)
	if err != nil {
		return false, false, err
	}

	return result.Changed, like, nil
}

// IsLiked returns whether a user has liked a post.
//
// Uses a single GETBIT on the appropriate bitmap shard — fast enough for
// synchronous UI checks at millisecond scale.
func IsLiked(postID, userID int64) (bool, error) {
	return bitmap.IsSet(rdb, "like", "post", postID, userID)
}

// IncrScoreForPost adjusts the post's ranking score in the score ZSet.
// This is separate from the like count — it controls the post ordering
// in feeds and search results.
func IncrScoreForPost(postID string, incr float64) error {
	_, err := rdb.ZIncrBy(KeyPostScoreZSet, incr, postID).Result()
	return err
}

// ---- deprecated: ZSet-based vote tracking, kept for migration / reference ----
//
// These functions operated on ZSets (ceddit:post:voted:{postID}) with
// userID as member and direction (1/-1/0) as score.
// They are superseded by the bitmap shard layer (bm:like:post:{id}:{chunk})
// which provides better memory efficiency, sharding, and idempotent toggle.

// GetVoteForPostByUser returns the vote direction (1, -1, or 0) from the
// legacy ZSet. Kept for reference; prefer bitmap.IsLiked.
func GetVoteForPostByUser(postID, userID string) float64 {
	return rdb.ZScore(KeyPostVotedZsetPrefix+postID, userID).Val()
}

// RecordVoteForPostByUser writes vote direction to the legacy ZSet.
// Kept for reference; prefer bitmap.Toggle.
func RecordVoteForPostByUser(postID, userID string, direction float64) error {
	if direction == 0 {
		_, err := rdb.ZRem(KeyPostVotedZsetPrefix+postID, userID).Result()
		return err
	}

	_, err := rdb.ZAdd(KeyPostVotedZsetPrefix+postID, redis.Z{
		Score:  direction,
		Member: userID,
	}).Result()

	return err
}
