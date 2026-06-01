package cache

import "fmt"

// Redis key prefixes for the three-tier feed cache.
const (
	prefixFeedPublicIDs     = "feed:public:ids"     // L1 page skeleton
	prefixFeedPublicHasMore = "feed:public:hasmore"  // L1 hasMore flag
	prefixFeedFragment      = "feed:frag"            // L0 post metadata fragment
	prefixFeedCount         = "feed:cnt"             // L0 count fragment
)

// idsKey builds the L1 skeleton key for a public feed page.
// Uses size + hour-slot + page to reduce mass-invalidation on content changes.
func idsKey(size, page int, hourSlot int64) string {
	return fmt.Sprintf("%s:%d:%d:%d", prefixFeedPublicIDs, size, hourSlot, page)
}

// hasMoreKey builds the L1 hasMore flag key.
func hasMoreKey(size, page int, hourSlot int64) string {
	return fmt.Sprintf("%s:%d:%d:%d", prefixFeedPublicHasMore, size, hourSlot, page)
}

// fragmentKey builds the L0 per-post fragment key.
func fragmentKey(postID int64) string {
	return fmt.Sprintf("%s:%d", prefixFeedFragment, postID)
}

// countKey builds the L0 per-post count fragment key.
func countKey(postID int64) string {
	return fmt.Sprintf("%s:%d", prefixFeedCount, postID)
}

// l2Key builds the L2 local cache key for a public feed page.
func l2Key(page, size int) string {
	return fmt.Sprintf("feed:public:%d:%d", page, size)
}
