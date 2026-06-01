package cache

import (
	"sync"
	"sync/atomic"
	"time"
)

// HotKeyDetector tracks per-key access frequency using a fixed-size sliding
// window of time slices. Each key maps to an int64 array (one slot per slice);
// a background goroutine advances the window and zeroes the next slice.
//
// This is a lightweight, local-only detector — no distributed coordination.
// Combined with Redis TTL extension it covers the majority of hotspot scenarios
// without the complexity of a distributed hot-key system.
type HotKeyDetector struct {
	mu       sync.RWMutex
	counters map[string][]int64 // key → per-segment counters
	current  int32              // current segment index (atomic for record fast-path)
	segments int                // total number of segments

	cfg HotKeyConfig

	stopCh chan struct{}
	once   sync.Once
}

// NewHotKeyDetector creates a detector and starts its background rotation
// goroutine. Call Shutdown() to stop it.
func NewHotKeyDetector(cfg HotKeyConfig) *HotKeyDetector {
	seg := cfg.WindowSeconds / cfg.SegmentSeconds
	if seg < 1 {
		seg = 1
	}

	d := &HotKeyDetector{
		counters:  make(map[string][]int64),
		segments:  seg,
		cfg:       cfg,
		stopCh:    make(chan struct{}),
	}

	go d.rotateLoop()
	return d
}

// Shutdown stops the background rotation goroutine.
func (d *HotKeyDetector) Shutdown() {
	d.once.Do(func() {
		close(d.stopCh)
	})
}

// Record increments the access counter for key in the current time slice.
// This is the fast path: one atomic load + one array increment. O(1).
func (d *HotKeyDetector) Record(key string) {
	idx := int(atomic.LoadInt32(&d.current))

	d.mu.RLock()
	arr, ok := d.counters[key]
	d.mu.RUnlock()

	if ok {
		// Best-effort increment; Go slice indexing is safe if the array
		// exists and idx is in bounds (always true within the lock-free
		// fast path since segments never changes after init).
		if idx < len(arr) {
			atomic.AddInt64(&arr[idx], 1)
		}
		return
	}

	// Slow path: first access for this key.
	d.mu.Lock()
	// Double-check after acquiring write lock.
	if arr, ok = d.counters[key]; ok {
		d.mu.Unlock()
		if idx < len(arr) {
			atomic.AddInt64(&arr[idx], 1)
		}
		return
	}
	arr = make([]int64, d.segments)
	arr[idx] = 1
	d.counters[key] = arr
	d.mu.Unlock()
}

// Heat returns the total access count for key over the current sliding window.
// O(segments), where segments is typically 6–12 — negligible.
func (d *HotKeyDetector) Heat(key string) int {
	d.mu.RLock()
	arr, ok := d.counters[key]
	d.mu.RUnlock()
	if !ok {
		return 0
	}

	var sum int64
	for i := range arr {
		sum += atomic.LoadInt64(&arr[i])
	}
	return int(sum)
}

// Level maps the current heat value to a HeatLevel using configured thresholds.
func (d *HotKeyDetector) Level(key string) HeatLevel {
	h := d.Heat(key)
	if h >= d.cfg.LevelHigh {
		return HeatHigh
	}
	if h >= d.cfg.LevelMedium {
		return HeatMedium
	}
	if h >= d.cfg.LevelLow {
		return HeatLow
	}
	return HeatNone
}

// ExtendSeconds returns the additional TTL seconds for a key based on its
// current heat level, or 0 if below the low threshold.
func (d *HotKeyDetector) ExtendSeconds(key string) int {
	switch d.Level(key) {
	case HeatHigh:
		return d.cfg.ExtendHighSeconds
	case HeatMedium:
		return d.cfg.ExtendMediumSeconds
	case HeatLow:
		return d.cfg.ExtendLowSeconds
	default:
		return 0
	}
}

// Reset zeroes the counter array for a key. Called on cache invalidation so
// stale hotness doesn't influence future caching decisions.
func (d *HotKeyDetector) Reset(key string) {
	d.mu.Lock()
	delete(d.counters, key)
	d.mu.Unlock()
}

// rotateLoop advances the window every segmentSeconds. It zeroes the next
// segment for ALL tracked keys, implementing the sliding window.
func (d *HotKeyDetector) rotateLoop() {
	ticker := time.NewTicker(time.Duration(d.cfg.SegmentSeconds) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-d.stopCh:
			return
		case <-ticker.C:
			d.rotate()
		}
	}
}

func (d *HotKeyDetector) rotate() {
	next := (int(atomic.LoadInt32(&d.current)) + 1) % d.segments

	d.mu.RLock()
	// Zero the next segment for all tracked keys.
	for _, arr := range d.counters {
		if next < len(arr) {
			atomic.StoreInt64(&arr[next], 0)
		}
	}
	d.mu.RUnlock()

	atomic.StoreInt32(&d.current, int32(next))
}
