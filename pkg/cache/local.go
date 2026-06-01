package cache

import (
	"sync"
	"time"
)

// localCache is a TTL-based in-memory cache for complete page responses (L2).
// It is the fastest tier: a hit avoids Redis and DB entirely.
//
// Design:
//   - map + sync.RWMutex for concurrent read safety.
//   - Each entry carries an expiry; stale entries are skipped on Get.
//   - A background goroutine periodically sweeps expired entries.
type localCache struct {
	mu       sync.RWMutex
	entries  map[string]*localEntry
	maxSize  int           // soft cap on entries
	stopCh   chan struct{}
	stopOnce sync.Once
}

type localEntry struct {
	value   *FeedPageResponse
	expires time.Time
}

// newLocalCache creates a local cache and starts the background sweep goroutine.
func newLocalCache(maxSize int) *localCache {
	if maxSize <= 0 {
		maxSize = 1000
	}
	c := &localCache{
		entries: make(map[string]*localEntry),
		maxSize: maxSize,
		stopCh:  make(chan struct{}),
	}
	go c.sweepLoop()
	return c
}

// Get returns the cached value if present and not expired.
func (c *localCache) Get(key string) *FeedPageResponse {
	c.mu.RLock()
	ent, ok := c.entries[key]
	c.mu.RUnlock()
	if !ok {
		return nil
	}
	if time.Now().After(ent.expires) {
		return nil
	}
	return ent.value
}

// Put stores a value with the given TTL. If the cache exceeds soft cap, a
// single random-ish entry is evicted (simple FIFO-approximate eviction).
func (c *localCache) Put(key string, value *FeedPageResponse, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Simple eviction: if at capacity and key is new, evict one entry.
	if _, exists := c.entries[key]; !exists && len(c.entries) >= c.maxSize {
		for k := range c.entries {
			delete(c.entries, k)
			break
		}
	}

	c.entries[key] = &localEntry{
		value:   value,
		expires: time.Now().Add(ttl),
	}
}

// Delete removes a key from the cache.
func (c *localCache) Delete(key string) {
	c.mu.Lock()
	delete(c.entries, key)
	c.mu.Unlock()
}

// DeleteByPrefix removes all keys that start with the given prefix.
// Used for bulk invalidation (e.g., all pages of public feed).
func (c *localCache) DeleteByPrefix(prefix string) {
	c.mu.Lock()
	for k := range c.entries {
		if len(k) >= len(prefix) && k[:len(prefix)] == prefix {
			delete(c.entries, k)
		}
	}
	c.mu.Unlock()
}

// Size returns the current number of entries.
func (c *localCache) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.entries)
}

// Shutdown stops the background sweep goroutine.
func (c *localCache) Shutdown() {
	c.stopOnce.Do(func() {
		close(c.stopCh)
	})
}

// sweepLoop periodically removes expired entries.
func (c *localCache) sweepLoop() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-c.stopCh:
			return
		case <-ticker.C:
			c.sweep()
		}
	}
}

func (c *localCache) sweep() {
	c.mu.Lock()
	now := time.Now()
	for k, ent := range c.entries {
		if now.After(ent.expires) {
			delete(c.entries, k)
		}
	}
	c.mu.Unlock()
}
