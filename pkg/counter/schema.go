package counter

// CounterSchema defines the layout of the SDS (Simple Dynamic String) binary
// counter blob. Each field is an 8-byte little-endian int64 at a fixed byte
// offset. The schema is versioned so that future field additions can coexist.
//
// Post SDS layout (8 bytes):
//
//	Offset 0: like_count  (int64)
//
// User SDS layout (16 bytes) — defined in countint package:
//
//	Offset 0: following_count (int64)
//	Offset 8: follower_count  (int64)

const (
	// Post field indices (matching countint.PostLikeOffset).
	PostFieldLike = 0

	// PostBlobSize is the total post SDS size in bytes.
	PostBlobSize = 8

	// PostFieldSize is the size of each field in bytes (int64).
	PostFieldSize = 8
)

// Metric names used in bitmap keys and counter events.
const (
	MetricLike = "like"
	MetricFav  = "fav"
)

// Entity type constants.
const (
	EntityPost = "post"
	EntityUser = "user"
)

// PostMetricIndex maps a metric name to its SDS byte offset for post entities.
func PostMetricIndex(metric string) (idx int, ok bool) {
	switch metric {
	case MetricLike:
		return PostFieldLike, true
	default:
		return 0, false
	}
}

// PostMetricNames returns all metric names for post entities in SDS offset order.
func PostMetricNames() []string {
	return []string{MetricLike}
}
