package redis

import (
	"ceddit/pkg/countint"
	"fmt"
	"testing"

	"github.com/go-redis/redis"
)

// newTestClient connects to the local dev Redis (DB 15) or skips the test.
func newTestClient(t *testing.T) *redis.Client {
	t.Helper()
	rdb := redis.NewClient(&redis.Options{
		Addr: "127.0.0.1:16379",
		DB:   15, // isolated DB for tests
	})
	if _, err := rdb.Ping().Result(); err != nil {
		t.Skipf("Redis not available at 127.0.0.1:16379: %v", err)
	}
	return rdb
}

// TestLuaScriptAtomicity tests the core Lua script that powers atomic
// increment on a binary CountInt blob.
func TestLuaScriptAtomicity(t *testing.T) {
	rdb := newTestClient(t)
	defer func() {
		_ = rdb.FlushDB().Err()
		_ = rdb.Close()
	}()

	// Pre-load to enable EVALSHA.
	if err := countint.CountIncrByScript.Load(rdb).Err(); err != nil {
		t.Fatalf("load script: %v", err)
	}

	userKey := "test:ucnt:1"

	// --- auto-create on first incr ---
	n, err := countint.CountIncrByScript.Run(rdb, []string{userKey},
		countint.UserFollowingOffset, 1, countint.UserBlobSize,
	).Int64()
	if err != nil {
		t.Fatalf("first incr: %v", err)
	}
	if n != 1 {
		t.Fatalf("after first incr = %d, want 1", n)
	}

	// --- second incr on same field ---
	n, err = countint.CountIncrByScript.Run(rdb, []string{userKey},
		countint.UserFollowingOffset, 2, countint.UserBlobSize,
	).Int64()
	if err != nil {
		t.Fatalf("second incr: %v", err)
	}
	if n != 3 {
		t.Fatalf("after second incr = %d, want 3", n)
	}

	// --- different field (follower, offset 8) ---
	n, err = countint.CountIncrByScript.Run(rdb, []string{userKey},
		countint.UserFollowerOffset, 7, countint.UserBlobSize,
	).Int64()
	if err != nil {
		t.Fatalf("follower incr: %v", err)
	}
	if n != 7 {
		t.Fatalf("follower incr = %d, want 7", n)
	}

	// --- read back and verify both fields are independent ---
	data, err := rdb.Get(userKey).Bytes()
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	following, follower := countint.DecodeUserCounts(data)
	if following != 3 || follower != 7 {
		t.Errorf("blob = (following=%d, follower=%d), want (3, 7)", following, follower)
	}

	// --- decrement ---
	n, err = countint.CountIncrByScript.Run(rdb, []string{userKey},
		countint.UserFollowingOffset, -2, countint.UserBlobSize,
	).Int64()
	if err != nil {
		t.Fatalf("decr: %v", err)
	}
	if n != 1 {
		t.Fatalf("after decr = %d, want 1", n)
	}

	// --- floor at 0 (cannot go negative) ---
	n, err = countint.CountIncrByScript.Run(rdb, []string{userKey},
		countint.UserFollowingOffset, -5, countint.UserBlobSize,
	).Int64()
	if err != nil {
		t.Fatalf("decr below zero: %v", err)
	}
	if n != 0 {
		t.Fatalf("after decr below zero = %d, want 0 (floor)", n)
	}

	// --- post CountInt ---
	postKey := "test:pcnt:1"
	n, err = countint.CountIncrByScript.Run(rdb, []string{postKey},
		countint.PostLikeOffset, 42, countint.PostBlobSize,
	).Int64()
	if err != nil {
		t.Fatalf("post like incr: %v", err)
	}
	if n != 42 {
		t.Fatalf("post like = %d, want 42", n)
	}

	postData, _ := rdb.Get(postKey).Bytes()
	if l := countint.DecodePostLikeCount(postData); l != 42 {
		t.Errorf("read back post like = %d, want 42", l)
	}

	// --- non-existent key read: GET on utf-8 key produces redis.Nil ---
	val, err := rdb.Get("test:ucnt:nonexistent").Result()
	if err != redis.Nil {
		t.Errorf("expected redis.Nil for nonexistent key, got: val=%q err=%v", val, err)
	}

	t.Log("Lua script atomicity tests passed ✓")
}

// TestSetAndGetRoundtrip verifies the encode→SET→GET→decode pipeline
// used by the reconciler.
func TestSetAndGetRoundtrip(t *testing.T) {
	rdb := newTestClient(t)
	defer func() {
		_ = rdb.FlushDB().Err()
		_ = rdb.Close()
	}()

	// Simulate what the reconciler does: encode → SET
	blob := countint.EncodeUserCounts(1234, 5678)
	if err := rdb.Set("test:ucnt:roundtrip", blob, 0).Err(); err != nil {
		t.Fatalf("set: %v", err)
	}

	// Simulate what the API reads: GET → decode
	data, err := rdb.Get("test:ucnt:roundtrip").Bytes()
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	following, follower := countint.DecodeUserCounts(data)
	if following != 1234 || follower != 5678 {
		t.Errorf("roundtrip = (%d, %d), want (1234, 5678)", following, follower)
	}

	// Post roundtrip.
	blob2 := countint.EncodePostLikeCount(888)
	if err := rdb.Set("test:pcnt:roundtrip", blob2, 0).Err(); err != nil {
		t.Fatalf("set post: %v", err)
	}
	data2, _ := rdb.Get("test:pcnt:roundtrip").Bytes()
	if l := countint.DecodePostLikeCount(data2); l != 888 {
		t.Errorf("post roundtrip = %d, want 888", l)
	}

	t.Log("Set/Get roundtrip tests passed ✓")
}

// TestKeyNaming verifies the key pattern matches the documented convention.
func TestKeyNaming(t *testing.T) {
	tests := []struct {
		fn       func(int64) string
		input    int64
		expected string
	}{
		{userCountKey, 123, "ucnt:123"},
		{userCountKey, 0, "ucnt:0"},
		{postCountKey, 456, "pcnt:456"},
	}

	for _, tt := range tests {
		got := tt.fn(tt.input)
		if got != tt.expected {
			t.Errorf("key(%d) = %q, want %q", tt.input, got, tt.expected)
		}
	}
	t.Log("Key naming tests passed ✓")
}

// Ensure the test file compiles with unused imports.
var _ = fmt.Sprintf
