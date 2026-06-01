package redis

const (
	KeyPostTimeZSet        = "ceddit:post:time"
	KeyPostScoreZSet       = "ceddit:post:score"
	KeyPostVotedZsetPrefix = "ceddit:post:voted:"

	// CountInt SDS keys (binary-packed counters).
	KeyUserCountPrefix = "ucnt:"
	KeyPostCountPrefix = "pcnt:"
)
