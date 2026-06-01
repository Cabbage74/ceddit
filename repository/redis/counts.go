package redis

import (
	"ceddit/pkg/countint"
	"fmt"
)

// ---- key builders ----

func userCountKey(userID int64) string {
	return fmt.Sprintf("%s%d", KeyUserCountPrefix, userID)
}

func postCountKey(postID int64) string {
	return fmt.Sprintf("%s%d", KeyPostCountPrefix, postID)
}

// ---- initialisation ----

// InitCountScripts pre-loads the CountInt Lua scripts into Redis so that
// subsequent calls can use EVALSHA.
func InitCountScripts() error {
	if err := countint.CountIncrByScript.Load(rdb).Err(); err != nil {
		return fmt.Errorf("load count_incrby script: %w", err)
	}
	return nil
}

// ---- atomic increment ----

// IncrUserCount atomically increments a field inside a user CountInt blob.
// offset should be one of countint.UserFollowingOffset or UserFollowerOffset.
func IncrUserCount(userID int64, offset int, delta int64) (int64, error) {
	key := userCountKey(userID)
	n, err := countint.CountIncrByScript.Run(rdb, []string{key},
		offset, delta, countint.UserBlobSize,
	).Int64()
	if err != nil {
		return 0, fmt.Errorf("incr user count %d offset %d: %w", userID, offset, err)
	}
	return n, nil
}

// IncrPostLikeCount atomically increments the like count inside a post CountInt blob.
func IncrPostLikeCount(postID int64, delta int64) (int64, error) {
	key := postCountKey(postID)
	n, err := countint.CountIncrByScript.Run(rdb, []string{key},
		countint.PostLikeOffset, delta, countint.PostBlobSize,
	).Int64()
	if err != nil {
		return 0, fmt.Errorf("incr post like count %d: %w", postID, err)
	}
	return n, nil
}

// ---- readers ----

// GetUserCounts returns the following and follower counts for a user from Redis.
// Returns (0, 0) if the key does not exist.
func GetUserCounts(userID int64) (following, follower int64, err error) {
	key := userCountKey(userID)
	data, err := rdb.Get(key).Bytes()
	if err != nil {
		if err.Error() == "redis: nil" {
			return 0, 0, nil
		}
		return 0, 0, fmt.Errorf("get user counts %d: %w", userID, err)
	}
	following, follower = countint.DecodeUserCounts(data)
	return
}

// GetPostLikeCount returns the like count for a post from Redis.
// Returns 0 if the key does not exist.
func GetPostLikeCount(postID int64) (int64, error) {
	key := postCountKey(postID)
	data, err := rdb.Get(key).Bytes()
	if err != nil {
		if err.Error() == "redis: nil" {
			return 0, nil
		}
		return 0, fmt.Errorf("get post like count %d: %w", postID, err)
	}
	return countint.DecodePostLikeCount(data), nil
}

// ---- writers (for reconciler) ----

// SetUserCounts overwrites the user CountInt blob. Used by the reconciler.
func SetUserCounts(userID int64, following, follower int64) error {
	key := userCountKey(userID)
	data := countint.EncodeUserCounts(following, follower)
	return rdb.Set(key, data, 0).Err()
}

// SetPostLikeCount overwrites the post CountInt blob. Used by the reconciler.
func SetPostLikeCount(postID int64, like int64) error {
	key := postCountKey(postID)
	data := countint.EncodePostLikeCount(like)
	return rdb.Set(key, data, 0).Err()
}
