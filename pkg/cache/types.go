// Package cache implements a three-tier cache architecture for feed pages:
//
//	L2 — local in-memory cache (Caffeine-style, fastest, full page response)
//	L1 — Redis page skeleton cache (ID list + hasMore, for fast assembly)
//	L0 — Redis fragment cache (per-post metadata + counts, for page assembly)
//
// Personalised state (liked/faved) is never stored in cache; it is overlaid
// at response time via bitmap lookups to avoid cache fragmentation.
package cache

// FeedPageResponse is the full page stored in L2 (local) and returned to clients.
// Liked/Faved are NOT stored here — they are overlaid at response time.
type FeedPageResponse struct {
	Items   []FeedItemResponse `json:"items"`
	Page    int                `json:"page"`
	Size    int                `json:"size"`
	HasMore bool               `json:"has_more"`
}

// FeedItemResponse is one item in the feed. LikeCount and FavoriteCount are
// public aggregates; Liked and Faved are user-specific and overlaid.
type FeedItemResponse struct {
	PostID        int64  `json:"post_id,string"`
	Title         string `json:"title"`
	Description   string `json:"description"`
	AuthorID      int64  `json:"author_id,string"`
	AuthorName    string `json:"author_name"`
	LikeCount     *int64 `json:"like_count"`
	FavoriteCount *int64 `json:"favorite_count"`
	PublishTime   string `json:"publish_time"`
	CreateTime    string `json:"create_time"`
	// User-specific — not cached; overlaid at response time.
	Liked *bool `json:"liked,omitempty"`
	Faved *bool `json:"faved,omitempty"`
}

// pageSkeleton is the L1 cache value: the ID list and hasMore flag needed to
// assemble a page from fragments.
type pageSkeleton struct {
	IDs     []int64 `json:"ids"`
	HasMore bool    `json:"has_more"`
}

// postFragment is the L0 cache value for a single post's metadata.
// Stored as JSON in Redis under feed:frag:{postID}.
type postFragment struct {
	PostID      int64  `json:"post_id,string"`
	Title       string `json:"title"`
	Description string `json:"description"`
	AuthorID    int64  `json:"author_id,string"`
	AuthorName  string `json:"author_name"`
	PublishTime string `json:"publish_time"`
	CreateTime  string `json:"create_time"`
}

// countFragment is the L0 cache value for a single post's aggregate counts.
// Stored as JSON in Redis under feed:cnt:{postID}.
type countFragment struct {
	LikeCount    int64 `json:"like_count"`
	FavoriteCount int64 `json:"favorite_count"`
}

// HeatLevel classifies a key's current hotness.
type HeatLevel int

const (
	HeatNone   HeatLevel = iota // below low threshold
	HeatLow                     // ≥ levelLow
	HeatMedium                  // ≥ levelMedium
	HeatHigh                    // ≥ levelHigh
)

// fragRow holds the assembled fragment and count for a single post during
// page assembly from L0 cache.
type fragRow struct {
	frag *postFragment
	cnt  *countFragment
}

func (l HeatLevel) String() string {
	switch l {
	case HeatNone:
		return "none"
	case HeatLow:
		return "low"
	case HeatMedium:
		return "medium"
	case HeatHigh:
		return "high"
	default:
		return "unknown"
	}
}
