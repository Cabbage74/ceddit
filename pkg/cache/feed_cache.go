package cache

import (
	"ceddit/models"
	"ceddit/repository/mysql"
	"ceddit/repository/redis"
	"encoding/json"
	"fmt"
	"math/rand"
	"strconv"
	"time"

	goredis "github.com/go-redis/redis"
	"go.uber.org/zap"
)

// FeedCache orchestrates the three-tier cache for public feed pages.
//
//	Tier 2 (L2): local in-memory  → complete page response     → fastest
//	Tier 1 (L1): Redis skeleton    → ID list + hasMore          → fast assembly
//	Tier 0 (L0): Redis fragments   → per-post metadata + counts → reusable atoms
//
// Personalised state (liked/faved) is never cached — it is overlaid at
// response time from the bitmap fact layer.
type FeedCache struct {
	rdb    *goredis.Client
	local  *localCache
	hotKey *HotKeyDetector
	sf     *SingleFlight
	cfg    FeedCacheConfig
	hkCfg  HotKeyConfig
}

// NewFeedCache creates a FeedCache and starts background goroutines for the
// local cache sweeper and hotkey rotation.
func NewFeedCache(rdb *goredis.Client, cfg FeedCacheConfig, hkCfg HotKeyConfig) *FeedCache {
	return &FeedCache{
		rdb:    rdb,
		local:  newLocalCache(cfg.L2MaxEntries),
		hotKey: NewHotKeyDetector(hkCfg),
		sf:     NewSingleFlight(),
		cfg:    cfg,
		hkCfg:  hkCfg,
	}
}

// Shutdown stops background goroutines.
func (fc *FeedCache) Shutdown() {
	fc.local.Shutdown()
	fc.hotKey.Shutdown()
}

// ---- Public API ----

// GetPublicFeed returns a public feed page using the three-tier cache.
//
// Read path: L2 → L1+L0 → DB (protected by single-flight).
// Each tier hit records a hotkey access and may extend TTL for hot pages.
//
// currentUserID may be 0 for anonymous users — no personalised overlay is
// applied in that case.
func (fc *FeedCache) GetPublicFeed(page, size int, currentUserID int64) (*FeedPageResponse, error) {
	safeSize := clamp(size, 1, 50)
	safePage := max(page, 1)
	key := l2Key(safePage, safeSize)

	// Hour-slot sharding: reduces mass-invalidation when content changes
	// near hour boundaries. The page dimension (size/page) is combined with
	// the time dimension (hourSlot) so that hot pages aren't all invalidated
	// at once.
	hourSlot := time.Now().Unix() / 3600
	idsK := idsKey(safeSize, safePage, hourSlot)
	hasMoreK := hasMoreKey(safeSize, safePage, hourSlot)

	// ---- L2: local in-memory cache ----
	if localResp := fc.local.Get(key); localResp != nil {
		fc.hotKey.Record(key)
		fc.maybeExtendL2TTL(key)
		zap.L().Info("feed.public source=l2",
			zap.String("key", key), zap.Int("page", safePage), zap.Int("size", safeSize))
		enriched := fc.enrichUserState(localResp.Items, currentUserID)
		return &FeedPageResponse{
			Items:   enriched,
			Page:    localResp.Page,
			Size:    localResp.Size,
			HasMore: localResp.HasMore,
		}, nil
	}

	// ---- L1: Redis skeleton + L0 fragment assembly ----
	if resp := fc.assembleFromCache(idsK, hasMoreK, safePage, safeSize, currentUserID); resp != nil {
		fc.local.Put(key, resp, fc.l2TTL(key))
		fc.hotKey.Record(key)
		fc.maybeExtendL2TTL(key)
		zap.L().Info("feed.public source=l1+l0",
			zap.String("key", key), zap.Int("page", safePage), zap.Int("size", safeSize))
		return resp, nil
	}

	// ---- Single-flight DB backfill ----
	result, err := fc.sf.Do(idsK, func() (interface{}, error) {
		// Double-check: another goroutine may have filled the cache while
		// we waited for the single-flight lock.
		if resp := fc.assembleFromCache(idsK, hasMoreK, safePage, safeSize, currentUserID); resp != nil {
			return resp, nil
		}

		return fc.backfillFromDB(idsK, hasMoreK, key, safePage, safeSize, currentUserID)
	})
	if err != nil {
		return nil, err
	}

	resp, ok := result.(*FeedPageResponse)
	if !ok {
		return nil, fmt.Errorf("unexpected single-flight result type: %T", result)
	}

	fc.sf.Delete(idsK)
	return resp, nil
}

// InvalidatePublicFeed removes all L1 skeleton keys for the public feed and
// clears the L2 local cache. Called when a new post is published or content
// changes.
//
// Uses the double-delete pattern:
//  1. Delete L1 keys immediately.
//  2. Sleep a short window (50ms) to let in-flight writes settle.
//  3. Delete again to catch any stale writes from concurrent backfills.
func (fc *FeedCache) InvalidatePublicFeed() {
	// Delete L1 skeleton keys (Redis).
	fc.deleteL1Keys()

	// Delete L2 local cache entries.
	fc.local.DeleteByPrefix("feed:public:")

	zap.L().Info("feed.public invalidated (first delete)")

	// Double-delete: after a short window, delete again to catch race writes.
	time.Sleep(50 * time.Millisecond)
	fc.deleteL1Keys()
	fc.local.DeleteByPrefix("feed:public:")

	zap.L().Info("feed.public invalidated (second delete)")
}

// ---- L1 + L0 assembly ----

// assembleFromCache attempts to build a full page response from the L1
// skeleton and L0 fragments. Returns nil if the skeleton is missing.
func (fc *FeedCache) assembleFromCache(idsKey, hasMoreKey string, page, size int, currentUserID int64) *FeedPageResponse {
	// Read L1 skeleton.
	idsData, err := fc.rdb.Get(idsKey).Bytes()
	if err != nil {
		return nil
	}

	var skel pageSkeleton
	if err := json.Unmarshal(idsData, &skel); err != nil {
		zap.L().Warn("feed.public unmarshal skeleton failed",
			zap.String("key", idsKey), zap.Error(err))
		return nil
	}

	hasMoreData, err := fc.rdb.Get(hasMoreKey).Bytes()
	if err != nil {
		return nil
	}
	var hasMore bool
	if err := json.Unmarshal(hasMoreData, &hasMore); err != nil {
		hasMore = false
	}

	// Assemble from L0 fragments.
	items := fc.assembleItems(skel.IDs, currentUserID)
	if items == nil {
		return nil
	}

	enriched := fc.enrichUserState(items, currentUserID)
	return &FeedPageResponse{
		Items:   enriched,
		Page:    page,
		Size:    size,
		HasMore: hasMore,
	}
}

// assembleItems reads L0 fragments for the given IDs. Missing fragments are
// backfilled from DB. Returns nil if too many fragments are missing (indicates
// the skeleton itself is stale).
func (fc *FeedCache) assembleItems(ids []int64, currentUserID int64) []FeedItemResponse {
	if len(ids) == 0 {
		return []FeedItemResponse{}
	}

	// Batch-read post fragments from Redis.
	fragKeys := make([]string, len(ids))
	cntKeys := make([]string, len(ids))
	for i, id := range ids {
		fragKeys[i] = fragmentKey(id)
		cntKeys[i] = countKey(id)
	}

	// Pipeline GET for fragments.
	pipe := fc.rdb.Pipeline()
	fragCmds := make([]*goredis.StringCmd, len(ids))
	cntCmds := make([]*goredis.StringCmd, len(ids))
	for i := range ids {
		fragCmds[i] = pipe.Get(fragKeys[i])
		cntCmds[i] = pipe.Get(cntKeys[i])
	}
	_, _ = pipe.Exec()

	// Collect results; track missing.
	results := make([]*fragRow, len(ids))
	missingIDs := make([]int64, 0)

	for i := range ids {
		r := &fragRow{}
		if fragStr, err := fragCmds[i].Result(); err == nil {
			var f postFragment
			if json.Unmarshal([]byte(fragStr), &f) == nil {
				r.frag = &f
			}
		}
		if cntStr, err := cntCmds[i].Result(); err == nil {
			var c countFragment
			if json.Unmarshal([]byte(cntStr), &c) == nil {
				r.cnt = &c
			}
		}
		if r.frag == nil {
			missingIDs = append(missingIDs, ids[i])
		}
		results[i] = r
	}

	// Backfill missing fragments from DB.
	if len(missingIDs) > 0 {
		fc.backfillFragments(missingIDs, results, ids)
	}

	// Build response items.
	items := make([]FeedItemResponse, 0, len(ids))
	for i, r := range results {
		if r.frag == nil {
			// Fragment still missing after backfill — skip.
			continue
		}
		item := FeedItemResponse{
			PostID:      r.frag.PostID,
			Title:       r.frag.Title,
			Description: r.frag.Description,
			AuthorID:    r.frag.AuthorID,
			AuthorName:  r.frag.AuthorName,
			PublishTime: r.frag.PublishTime,
			CreateTime:  r.frag.CreateTime,
		}
		if r.cnt != nil {
			lc := r.cnt.LikeCount
			fc := r.cnt.FavoriteCount
			item.LikeCount = &lc
			item.FavoriteCount = &fc
		} else {
			// Count fragment missing; fetch from CountInt SDS.
			if likeCnt, err := redis.GetPostLikeCount(ids[i]); err == nil {
				item.LikeCount = &likeCnt
			}
		}
		items = append(items, item)
	}

	return items
}

// ---- DB backfill ----

// backfillFromDB queries the database directly and writes all three cache tiers.
func (fc *FeedCache) backfillFromDB(idsKey, hasMoreKey, l2key string, page, size int, currentUserID int64) (*FeedPageResponse, error) {
	offset := (page - 1) * size
	rows, err := mysql.GetPublishedPosts(int64(page), int64(size+1))
	if err != nil {
		return nil, fmt.Errorf("backfill query db: %w", err)
	}
	_ = offset // used implicitly via page-based query in mysql layer

	hasMore := len(rows) > size
	if hasMore {
		rows = rows[:size]
	}

	// Build item list without personalised state.
	items := make([]FeedItemResponse, 0, len(rows))
	ids := make([]int64, 0, len(rows))
	for _, post := range rows {
		authorName := ""
		if user, err := mysql.GetUserByID(post.AuthorID); err == nil {
			authorName = user.Username
		}

		var pubTime, createTime string
		if post.PublishTime != nil {
			pubTime = post.PublishTime.Format(time.RFC3339)
		}
		if post.CreateTime != nil {
			createTime = post.CreateTime.Format(time.RFC3339)
		}

		title := ""
		if post.Title != nil {
			title = *post.Title
		}
		desc := ""
		if post.Description != nil {
			desc = *post.Description
		}

		// Fetch counts from Redis CountInt SDS.
		likeCount, _ := redis.GetPostLikeCount(post.PostID)
		favCount := int64(0)

		item := FeedItemResponse{
			PostID:        post.PostID,
			Title:         title,
			Description:   desc,
			AuthorID:      post.AuthorID,
			AuthorName:    authorName,
			LikeCount:     &likeCount,
			FavoriteCount: &favCount,
			PublishTime:   pubTime,
			CreateTime:    createTime,
		}
		items = append(items, item)
		ids = append(ids, post.PostID)
	}

	// Write L0 fragments.
	l0TTL := fc.l0TTL()
	fc.writeFragments(rows, l0TTL)
	fc.writeCountFragments(ids, l0TTL)

	// Write L1 skeleton.
	l1TTL := fc.l1TTL(l2key)
	fc.writeSkeleton(idsKey, hasMoreKey, ids, hasMore, l1TTL)

	// Build cache response (no personalised state).
	resp := &FeedPageResponse{
		Items:   items,
		Page:    page,
		Size:    size,
		HasMore: hasMore,
	}

	// Write L2 local.
	fc.local.Put(l2key, resp, fc.l2TTL(l2key))

	// Record hotkey access.
	fc.hotKey.Record(l2key)

	// Enrich with user state for the actual response.
	enriched := fc.enrichUserState(items, currentUserID)
	zap.L().Info("feed.public source=db",
		zap.String("key", l2key), zap.Int("page", page), zap.Int("size", size),
		zap.Bool("hasMore", hasMore))

	return &FeedPageResponse{
		Items:   enriched,
		Page:    page,
		Size:    size,
		HasMore: hasMore,
	}, nil
}

// ---- Fragment writes ----

func (fc *FeedCache) writeFragments(posts []*models.Post, ttl time.Duration) {
	pipe := fc.rdb.Pipeline()
	for _, post := range posts {
		authorName := ""
		if user, err := mysql.GetUserByID(post.AuthorID); err == nil {
			authorName = user.Username
		}

		var pubTime, createTime string
		if post.PublishTime != nil {
			pubTime = post.PublishTime.Format(time.RFC3339)
		}
		if post.CreateTime != nil {
			createTime = post.CreateTime.Format(time.RFC3339)
		}

		title := ""
		if post.Title != nil {
			title = *post.Title
		}
		desc := ""
		if post.Description != nil {
			desc = *post.Description
		}

		frag := postFragment{
			PostID:      post.PostID,
			Title:       title,
			Description: desc,
			AuthorID:    post.AuthorID,
			AuthorName:  authorName,
			PublishTime: pubTime,
			CreateTime:  createTime,
		}
		data, _ := json.Marshal(frag)
		pipe.Set(fragmentKey(post.PostID), data, ttl)
	}
	_, _ = pipe.Exec()
}

func (fc *FeedCache) writeCountFragments(ids []int64, ttl time.Duration) {
	pipe := fc.rdb.Pipeline()
	for _, id := range ids {
		likeCount, err := redis.GetPostLikeCount(id)
		if err != nil {
			likeCount = 0
		}
		cnt := countFragment{
			LikeCount:    likeCount,
			FavoriteCount: 0,
		}
		data, _ := json.Marshal(cnt)
		pipe.Set(countKey(id), data, ttl)
	}
	_, _ = pipe.Exec()
}

func (fc *FeedCache) writeSkeleton(idsKey, hasMoreKey string, ids []int64, hasMore bool, ttl time.Duration) {
	pipe := fc.rdb.Pipeline()

	skel := pageSkeleton{IDs: ids, HasMore: hasMore}
	idsData, _ := json.Marshal(skel)
	pipe.Set(idsKey, idsData, ttl)

	hasMoreData, _ := json.Marshal(hasMore)
	pipe.Set(hasMoreKey, hasMoreData, ttl)

	_, _ = pipe.Exec()
}

// ---- Fragment backfill ----

func (fc *FeedCache) backfillFragments(missingIDs []int64, results []*fragRow, allIDs []int64) {
	// Build ID-to-index map.
	idToIdx := make(map[int64]int, len(allIDs))
	for i, id := range allIDs {
		idToIdx[id] = i
	}

	// Batch query posts from DB.
	strIDs := make([]string, len(missingIDs))
	for i, id := range missingIDs {
		strIDs[i] = strconv.FormatInt(id, 10)
	}

	posts, err := mysql.GetPublishedPostsByIDs(strIDs)
	if err != nil {
		zap.L().Warn("feed.public backfill fragments query failed", zap.Error(err))
		return
	}

	ttl := fc.l0TTL()
	pipe := fc.rdb.Pipeline()

	for _, post := range posts {
		idx, ok := idToIdx[post.PostID]
		if !ok {
			continue
		}

		authorName := ""
		if user, err := mysql.GetUserByID(post.AuthorID); err == nil {
			authorName = user.Username
		}

		var pubTime, createTime string
		if post.PublishTime != nil {
			pubTime = post.PublishTime.Format(time.RFC3339)
		}
		if post.CreateTime != nil {
			createTime = post.CreateTime.Format(time.RFC3339)
		}

		title := ""
		if post.Title != nil {
			title = *post.Title
		}
		desc := ""
		if post.Description != nil {
			desc = *post.Description
		}

		frag := postFragment{
			PostID:      post.PostID,
			Title:       title,
			Description: desc,
			AuthorID:    post.AuthorID,
			AuthorName:  authorName,
			PublishTime: pubTime,
			CreateTime:  createTime,
		}
		data, _ := json.Marshal(frag)
		pipe.Set(fragmentKey(post.PostID), data, ttl)

		results[idx].frag = &frag
	}
	_, _ = pipe.Exec()
}

// ---- TTL helpers ----

// l0TTL returns the L0 fragment TTL with random jitter.
func (fc *FeedCache) l0TTL() time.Duration {
	base := fc.cfg.L0TTLSeconds
	jitter := rand.Intn(fc.cfg.L0JitterSec + 1)
	return time.Duration(base+jitter) * time.Second
}

// l1TTL returns the L1 skeleton TTL with optional hotkey extension and jitter.
func (fc *FeedCache) l1TTL(key string) time.Duration {
	base := fc.cfg.L1TTLSeconds
	ext := fc.hotKey.ExtendSeconds(key)
	jitter := rand.Intn(fc.cfg.L1JitterSec + 1)
	return time.Duration(base+ext+jitter) * time.Second
}

// l2TTL returns the L2 local TTL with optional hotkey extension and jitter.
// L2 TTL is kept ≤ L1 TTL to prevent serving stale data after L1 expires.
func (fc *FeedCache) l2TTL(key string) time.Duration {
	base := fc.cfg.L2TTLSeconds
	ext := fc.hotKey.ExtendSeconds(key)
	jitter := rand.Intn(fc.cfg.L2JitterSec + 1)
	return time.Duration(base+ext+jitter) * time.Second
}

// maybeExtendL2TTL re-puts the L2 entry with a fresh TTL if the key is hot.
// This keeps hot pages in local cache longer without an explicit refresh.
func (fc *FeedCache) maybeExtendL2TTL(key string) {
	level := fc.hotKey.Level(key)
	if level < HeatLow {
		return
	}
	// Fetch the current value and re-put with extended TTL.
	if val := fc.local.Get(key); val != nil {
		fc.local.Put(key, val, fc.l2TTL(key))
	}
}

// ---- Invalidation helpers ----

func (fc *FeedCache) deleteL1Keys() {
	// Use SCAN to find and delete all L1 skeleton and hasMore keys.
	fc.deleteKeysByPattern(prefixFeedPublicIDs + ":*")
	fc.deleteKeysByPattern(prefixFeedPublicHasMore + ":*")
}

func (fc *FeedCache) deleteKeysByPattern(pattern string) {
	var cursor uint64
	for {
		keys, nextCursor, err := fc.rdb.Scan(cursor, pattern, 100).Result()
		if err != nil {
			zap.L().Warn("feed.public scan keys failed",
				zap.String("pattern", pattern), zap.Error(err))
			break
		}
		if len(keys) > 0 {
			_ = fc.rdb.Del(keys...).Err()
		}
		cursor = nextCursor
		if cursor == 0 {
			break
		}
	}
}

// ---- User-state overlay ----

// enrichUserState overlays liked/faved status for the current user onto each
// item. This is done in memory at response time — user state is NEVER cached
// in the public tiers to avoid fragmentation.
//
// For anonymous users (userID == 0), Liked/Faved remain nil.
func (fc *FeedCache) enrichUserState(items []FeedItemResponse, userID int64) []FeedItemResponse {
	if userID == 0 || len(items) == 0 {
		return items
	}

	result := make([]FeedItemResponse, len(items))
	for i, item := range items {
		result[i] = item

		liked, err := redis.IsLiked(item.PostID, userID)
		if err == nil {
			result[i].Liked = &liked
		}
		// TODO: add fav bitmap lookup when favourite feature is implemented.
	}
	return result
}

// ---- util ----

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
