package redis

import (
	"ceddit/models"
	"time"

	"github.com/go-redis/redis"
)

func AddPostToTimeline(postID int64) error {
	pipeline := rdb.TxPipeline()

	pipeline.ZAdd(KeyPostTimeZSet, redis.Z{
		Score:  float64(time.Now().Unix()),
		Member: postID,
	}).Result()

	pipeline.ZAdd(KeyPostScoreZSet, redis.Z{
		Score:  float64(time.Now().Unix()),
		Member: postID,
	}).Result()

	_, err := pipeline.Exec()

	return err
}

func GetPostIDInOrder(p *models.ParamPostList) ([]string, error) {
	key := KeyPostTimeZSet
	if p.Order == models.OrderScore {
		key = KeyPostScoreZSet
	}

	start := (p.Page - 1) * p.Size
	end := start + p.Size - 1
	return rdb.ZRevRange(key, start, end).Result()
}

func GetPostVote(ids []string) []int64 {
	var data []int64
	for _, id := range ids {
		key := KeyPostVotedZsetPrefix + id
		vUp := rdb.ZCount(key, "1", "1").Val()
		vDown := rdb.ZCount(key, "-1", "-1").Val()
		data = append(data, vUp-vDown)
	}
	return data
}
