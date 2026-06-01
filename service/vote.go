package service

import (
	"ceddit/models"
	"ceddit/pkg/counter"
	"ceddit/pkg/countint"
	"ceddit/repository/redis"
	"errors"
	"strconv"

	"go.uber.org/zap"
)

const (
	scorePerVote = 432
)

// VoteResult describes the outcome of a vote operation.
type VoteResult struct {
	Changed bool `json:"changed"` // true if the vote state actually changed
	Liked   bool `json:"liked"`   // the new like state after the operation
}

// VoteForPost processes a vote (like / unlike) on a post.
//
// Synchronous path (instant):
//  1. Check voting window (1 week).
//  2. Atomically toggle the like bit in the bitmap shard layer.
//     This is the fact store — determines "did user X like post Y".
//  3. If the bit changed: update the ranking ZSet score (±432).
//
// Asynchronous path (~1s eventual consistency):
//  4. If the bit changed: publish a CounterEvent to Kafka.
//     → Aggregation consumer writes to Redis Hash bucket.
//     → Flush scheduler (every 1s) folds delta into SDS via Lua.
func VoteForPost(userID int64, p *models.ParamVote) (*VoteResult, error) {
	postIDStr := strconv.FormatInt(p.PostID, 10)

	if !redis.CanVote(postIDStr) {
		return nil, errors.New("too late to vote for this post")
	}

	// direction == 1 means the user wants to like; anything else means unlike.
	like := p.Direction == 1

	// Atomically toggle the bitmap. Toggle is idempotent: repeated calls
	// with the same (user, post, like) produce no change.
	changed, liked, err := redis.ToggleLike(p.PostID, userID, like)
	if err != nil {
		return nil, err
	}

	if changed {
		// Update ranking score synchronously.
		var scoreDelta float64
		if like {
			scoreDelta = scorePerVote
		} else {
			scoreDelta = -scorePerVote
		}
		if err := redis.IncrScoreForPost(postIDStr, scoreDelta); err != nil {
			return nil, err
		}

		// Produce counter event for async aggregation → flush to SDS.
		var delta int64
		if like {
			delta = 1
		} else {
			delta = -1
		}
		if err := counter.PublishPostLikeEvent(p.PostID, userID, delta, countint.PostLikeOffset); err != nil {
			zap.L().Warn("publish post like event failed",
				zap.Int64("post_id", p.PostID),
				zap.Int64("user_id", userID),
				zap.Error(err),
			)
			// Do not fail the request — the bitmap fact is already recorded;
			// the reconciler will eventually fix the SDS drift.
		}
	}

	return &VoteResult{Changed: changed, Liked: liked}, nil
}
