package service

import (
	"ceddit/models"
	"ceddit/repository/redis"
	"errors"
	"math"
	"strconv"
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
	return nil
}
