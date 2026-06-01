package service

import (
	"ceddit/models"
	"ceddit/repository/redis"
	"errors"
	"math"
	"strconv"

	"go.uber.org/zap"
)

const (
	scorePerVote = 432
)

func VoteForPost(userID int64, p *models.ParamVote) error {
	userIDStr := strconv.FormatInt(userID, 10)
	postIDStr := strconv.FormatInt(p.PostID, 10)
	direction := float64(p.Direction)

	if !redis.CanVote(postIDStr) {
		return errors.New("too late to vote for this post")
	}

	oldDirection := redis.GetVoteForPostByUser(postIDStr, userIDStr)
	if direction == oldDirection {
		return errors.New("same direction")
	}

	var op float64
	if direction > oldDirection {
		op = 1
	} else {
		op = -1
	}
	diff := math.Abs(direction - oldDirection)
	if err := redis.IncrScoreForPost(postIDStr, op*diff*scorePerVote); err != nil {
		return err
	}

	if err := redis.RecordVoteForPostByUser(postIDStr, userIDStr, direction); err != nil {
		return err
	}

	// Update post like_count in CountInt (only tracks upvotes, direction == 1).
	if oldDirection != 1 && direction == 1 {
		if _, err := redis.IncrPostLikeCount(p.PostID, 1); err != nil {
			zap.L().Warn("incr post like count failed",
				zap.Int64("post_id", p.PostID), zap.Error(err))
		}
	} else if oldDirection == 1 && direction != 1 {
		if _, err := redis.IncrPostLikeCount(p.PostID, -1); err != nil {
			zap.L().Warn("decr post like count failed",
				zap.Int64("post_id", p.PostID), zap.Error(err))
		}
	}

	return nil
}
