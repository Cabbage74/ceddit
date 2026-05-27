package redis

import (
	"time"

	"github.com/go-redis/redis"
)

const (
	oneWeekInSeconds = 7 * 24 * 3600
)

func CanVote(postID string) bool {
	postTime := rdb.ZScore(KeyPostTimeZSet, postID).Val()
	if float64(time.Now().Unix())-postTime > oneWeekInSeconds {
		return false
	}
	return true
}

func GetVoteForPostByUser(postID, userID string) float64 {
	return rdb.ZScore(KeyPostVotedZsetPrefix+postID, userID).Val()
}

func IncrScoreForPost(postID string, incr float64) error {
	_, err := rdb.ZIncrBy(KeyPostScoreZSet, incr, postID).Result()
	return err
}

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
