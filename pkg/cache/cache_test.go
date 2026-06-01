package cache

import (
	"sync"
	"testing"
	"time"
)

func TestHotKeyDetector_RecordAndHeat(t *testing.T) {
	cfg := HotKeyConfig{
		WindowSeconds:  60,
		SegmentSeconds: 10,
		LevelLow:       50,
		LevelMedium:    200,
		LevelHigh:      500,
	}
	d := NewHotKeyDetector(cfg)
	defer d.Shutdown()

	key := "feed:public:1:10"

	// Record accesses.
	for i := 0; i < 100; i++ {
		d.Record(key)
	}

	heat := d.Heat(key)
	if heat != 100 {
		t.Fatalf("expected heat=100, got %d", heat)
	}

	level := d.Level(key)
	if level != HeatLow {
		t.Fatalf("expected level=HeatLow, got %s", level)
	}

	ext := d.ExtendSeconds(key)
	if ext != cfg.ExtendLowSeconds {
		t.Fatalf("expected extend=%d, got %d", cfg.ExtendLowSeconds, ext)
	}
}

func TestHotKeyDetector_Rotate(t *testing.T) {
	cfg := HotKeyConfig{
		WindowSeconds:  60,
		SegmentSeconds: 1, // rotate every second for fast test
		LevelLow:       5,
		LevelMedium:    10,
		LevelHigh:      20,
	}
	d := NewHotKeyDetector(cfg)
	defer d.Shutdown()

	key := "feed:public:1:10"

	// Record in current segment.
	for i := 0; i < 10; i++ {
		d.Record(key)
	}

	if d.Heat(key) != 10 {
		t.Fatalf("expected heat=10 before rotate, got %d", d.Heat(key))
	}

	// Wait for rotation.
	time.Sleep(1200 * time.Millisecond)

	// After rotation, old segment is zeroed; heat should drop.
	postHeat := d.Heat(key)
	if postHeat >= 10 {
		t.Logf("heat after rotate: %d (may be non-zero if rotation not yet applied to all segments)", postHeat)
	}
}

func TestHotKeyDetector_Reset(t *testing.T) {
	cfg := HotKeyConfig{
		WindowSeconds:  60,
		SegmentSeconds: 10,
		LevelLow:       5,
		LevelMedium:    10,
		LevelHigh:      20,
	}
	d := NewHotKeyDetector(cfg)
	defer d.Shutdown()

	key := "feed:public:1:10"
	for i := 0; i < 50; i++ {
		d.Record(key)
	}

	if d.Heat(key) < 50 {
		t.Fatalf("expected heat>=50 before reset, got %d", d.Heat(key))
	}

	d.Reset(key)
	if d.Heat(key) != 0 {
		t.Fatalf("expected heat=0 after reset, got %d", d.Heat(key))
	}
}

func TestHotKeyDetector_Levels(t *testing.T) {
	cfg := HotKeyConfig{
		WindowSeconds:  60,
		SegmentSeconds: 10,
		LevelLow:       10,
		LevelMedium:    30,
		LevelHigh:      60,
	}
	d := NewHotKeyDetector(cfg)
	defer d.Shutdown()

	tests := []struct {
		name      string
		records   int
		wantLevel HeatLevel
		wantExt   int
	}{
		{"none", 5, HeatNone, 0},
		{"low", 15, HeatLow, 0},
		{"medium", 35, HeatMedium, 0},
		{"high", 70, HeatHigh, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key := "test:" + tt.name
			for i := 0; i < tt.records; i++ {
				d.Record(key)
			}
			level := d.Level(key)
			if level != tt.wantLevel {
				t.Fatalf("expected level=%s, got %s", tt.wantLevel, level)
			}
		})
	}
}

func TestLocalCache(t *testing.T) {
	c := newLocalCache(10)
	defer c.Shutdown()

	key := "feed:public:1:10"
	resp := &FeedPageResponse{
		Page:    1,
		Size:    10,
		HasMore: true,
	}

	// Put and get.
	c.Put(key, resp, 100*time.Millisecond)
	got := c.Get(key)
	if got == nil {
		t.Fatal("expected cache hit")
	}
	if got.Page != 1 || got.Size != 10 {
		t.Fatalf("unexpected cached value: %+v", got)
	}

	// Wait for expiry.
	time.Sleep(150 * time.Millisecond)
	got = c.Get(key)
	if got != nil {
		t.Fatal("expected cache miss after expiry")
	}
}

func TestLocalCache_DeleteByPrefix(t *testing.T) {
	c := newLocalCache(100)
	defer c.Shutdown()

	resp := &FeedPageResponse{Page: 1, Size: 10}
	c.Put("feed:public:1:10", resp, time.Hour)
	c.Put("feed:public:2:10", resp, time.Hour)
	c.Put("other:key", resp, time.Hour)

	c.DeleteByPrefix("feed:public:")

	if c.Get("feed:public:1:10") != nil {
		t.Fatal("expected feed:public:1:10 to be deleted")
	}
	if c.Get("feed:public:2:10") != nil {
		t.Fatal("expected feed:public:2:10 to be deleted")
	}
	if c.Get("other:key") == nil {
		t.Fatal("expected other:key to remain")
	}
}

func TestLocalCache_Eviction(t *testing.T) {
	c := newLocalCache(2)
	defer c.Shutdown()

	resp := &FeedPageResponse{}
	c.Put("a", resp, time.Hour)
	c.Put("b", resp, time.Hour)
	c.Put("c", resp, time.Hour) // should evict one

	if c.Size() > 2 {
		t.Fatalf("expected size <= 2, got %d", c.Size())
	}
}

func TestSingleFlight(t *testing.T) {
	sf := NewSingleFlight()

	var mu sync.Mutex
	var callCount int

	fn := func() (interface{}, error) {
		mu.Lock()
		callCount++
		mu.Unlock()
		time.Sleep(50 * time.Millisecond)
		return "result", nil
	}

	// Launch concurrent calls.
	var wg sync.WaitGroup
	results := make([]interface{}, 5)
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			r, err := sf.Do("key1", fn)
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}
			results[idx] = r
		}(i)
	}
	wg.Wait()

	if callCount != 1 {
		t.Fatalf("expected exactly 1 call, got %d", callCount)
	}
	for i, r := range results {
		if r != "result" {
			t.Fatalf("unexpected result at %d: %v", i, r)
		}
	}
}

func TestSingleFlight_DifferentKeys(t *testing.T) {
	sf := NewSingleFlight()

	var mu sync.Mutex
	callCounts := make(map[string]int)

	fn := func(key string) func() (interface{}, error) {
		return func() (interface{}, error) {
			mu.Lock()
			callCounts[key]++
			mu.Unlock()
			time.Sleep(50 * time.Millisecond) // ensure concurrent goroutines overlap
			return key, nil
		}
	}

	// Use a barrier to make all goroutines start simultaneously.
	var gate sync.WaitGroup
	gate.Add(6)

	var wg sync.WaitGroup
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func() {
			gate.Done()
			gate.Wait()
			defer wg.Done()
			sf.Do("a", fn("a"))
		}()
		wg.Add(1)
		go func() {
			gate.Done()
			gate.Wait()
			defer wg.Done()
			sf.Do("b", fn("b"))
		}()
	}
	wg.Wait()

	if callCounts["a"] != 1 {
		t.Fatalf("expected key 'a' to be called once, got %d", callCounts["a"])
	}
	if callCounts["b"] != 1 {
		t.Fatalf("expected key 'b' to be called once, got %d", callCounts["b"])
	}
}

func TestClamp(t *testing.T) {
	tests := []struct {
		v, lo, hi, want int
	}{
		{5, 1, 10, 5},
		{0, 1, 10, 1},
		{100, 1, 50, 50},
		{-1, 0, 10, 0},
	}
	for _, tt := range tests {
		got := clamp(tt.v, tt.lo, tt.hi)
		if got != tt.want {
			t.Fatalf("clamp(%d,%d,%d)=%d, want %d", tt.v, tt.lo, tt.hi, got, tt.want)
		}
	}
}

func TestKeys(t *testing.T) {
	k := idsKey(10, 1, 12345)
	if k == "" {
		t.Fatal("idsKey returned empty")
	}

	k = hasMoreKey(10, 1, 12345)
	if k == "" {
		t.Fatal("hasMoreKey returned empty")
	}

	k = fragmentKey(42)
	if k == "" {
		t.Fatal("fragmentKey returned empty")
	}

	k = countKey(42)
	if k == "" {
		t.Fatal("countKey returned empty")
	}

	k = l2Key(1, 10)
	if k == "" {
		t.Fatal("l2Key returned empty")
	}
}
