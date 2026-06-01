package service

import (
	"ceddit/models"
	"ceddit/repository/mysql"
	redispkg "ceddit/repository/redis"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/go-redis/redis"
	"go.uber.org/zap"
)

const (
	idempotencyKeyPrefix = "ceddit:idempotency:"
	idempotencyTTL       = 5 * time.Minute
	defaultPageLimit     = 20
	maxPageLimit         = 100
)

// ErrNotFollowing is returned when trying to unfollow someone not being followed.
var ErrNotFollowing = errors.New("not following this user")

// Follow creates a follow relationship and publishes an outbox event.
// idempotencyKey is the client-provided Idempotency-Key header.
func Follow(fromUserID, toUserID int64, idempotencyKey string) (*models.RespRelation, error) {
	if fromUserID == toUserID {
		return nil, errors.New("cannot follow yourself")
	}

	// Check idempotency cache.
	if idempotencyKey != "" {
		if cached, err := getIdempotencyResponse(idempotencyKey); err == nil && cached != nil {
			return cached, nil
		}
	}

	rel, err := mysql.CreateFollow(fromUserID, toUserID)
	if err != nil {
		return nil, fmt.Errorf("create follow: %w", err)
	}

	resp := &models.RespRelation{
		ID:         rel.ID,
		FromUserID: rel.FromUserID,
		ToUserID:   rel.ToUserID,
		CreatedAt:  rel.CreatedAt,
	}

	// Cache for idempotent retries.
	if idempotencyKey != "" {
		setIdempotencyResponse(idempotencyKey, resp)
	}

	return resp, nil
}

// Unfollow deletes a follow relationship and publishes an outbox event.
func Unfollow(fromUserID, toUserID int64, idempotencyKey string) error {
	if fromUserID == toUserID {
		return errors.New("cannot unfollow yourself")
	}

	if idempotencyKey != "" {
		if cached, err := getIdempotencyResponse(idempotencyKey); err == nil && cached != nil {
			return nil
		}
	}

	err := mysql.DeleteFollow(fromUserID, toUserID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) ||
			// Our repo wraps with "not following"
			errors.Is(err, ErrNotFollowing) {
			return ErrNotFollowing
		}
		return fmt.Errorf("delete follow: %w", err)
	}

	// Cache an empty success marker so retries don't re-execute.
	if idempotencyKey != "" {
		setIdempotencyResponse(idempotencyKey, map[string]string{"status": "ok"})
	}

	return nil
}

// GetFollowingList returns the user's following list with cursor pagination.
func GetFollowingList(userID int64, cursor string, limit int) (*models.RespFollowList, error) {
	if limit <= 0 || limit > maxPageLimit {
		limit = defaultPageLimit
	}

	list, nextCursor, err := mysql.GetFollowingList(userID, cursor, limit)
	if err != nil {
		return nil, err
	}

	// Read following count from Redis CountInt (with MySQL fallback).
	following, _, err := redispkg.GetUserCounts(userID)
	if err != nil {
		zap.L().Warn("get user counts from redis failed", zap.Int64("user", userID), zap.Error(err))
		following, err = mysql.GetFollowingCount(userID)
		if err != nil {
			return nil, err
		}
	}

	return &models.RespFollowList{
		List:       list,
		NextCursor: nextCursor,
		Total:      following,
	}, nil
}

// GetFollowerList returns the user's follower list with cursor pagination.
func GetFollowerList(userID int64, cursor string, limit int) (*models.RespFollowerList, error) {
	if limit <= 0 || limit > maxPageLimit {
		limit = defaultPageLimit
	}

	list, nextCursor, err := mysql.GetFollowerList(userID, cursor, limit)
	if err != nil {
		return nil, err
	}

	// Read follower count from Redis CountInt (with MySQL fallback).
	_, follower, err := redispkg.GetUserCounts(userID)
	if err != nil {
		zap.L().Warn("get user counts from redis failed", zap.Int64("user", userID), zap.Error(err))
		follower, err = mysql.GetFollowerCountFromProjection(userID)
		if err != nil {
			return nil, err
		}
	}

	return &models.RespFollowerList{
		List:       list,
		NextCursor: nextCursor,
		Total:      follower,
	}, nil
}

// IsFollowing checks if fromUserID follows toUserID.
func IsFollowing(fromUserID, toUserID int64) (bool, error) {
	return mysql.IsFollowing(fromUserID, toUserID)
}

// ---- idempotency helpers ----

func idempotencyCacheKey(key string) string {
	return idempotencyKeyPrefix + key
}

func getIdempotencyResponse(key string) (*models.RespRelation, error) {
	rdb := redispkg.GetRDB()
	data, err := rdb.Get(idempotencyCacheKey(key)).Result()
	if err == redis.Nil {
		return nil, nil // not cached
	}
	if err != nil {
		zap.L().Warn("redis get idempotency key failed", zap.Error(err))
		return nil, err
	}

	var resp models.RespRelation
	if err := json.Unmarshal([]byte(data), &resp); err != nil {
		return nil, fmt.Errorf("unmarshal cached idempotency: %w", err)
	}
	return &resp, nil
}

func setIdempotencyResponse(key string, resp any) {
	rdb := redispkg.GetRDB()
	data, err := json.Marshal(resp)
	if err != nil {
		zap.L().Warn("marshal idempotency response failed", zap.Error(err))
		return
	}
	if err := rdb.Set(idempotencyCacheKey(key), data, idempotencyTTL).Err(); err != nil {
		zap.L().Warn("redis set idempotency key failed", zap.Error(err))
	}
}
