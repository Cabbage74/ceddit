package bitmap

import (
	"testing"
)

func TestChunkOf(t *testing.T) {
	tests := []struct {
		userID int64
		chunk  int64
	}{
		{0, 0},
		{1, 0},
		{ChunkSize - 1, 0},
		{ChunkSize, 1},
		{ChunkSize + 1, 1},
		{2 * ChunkSize, 2},
		{10 * ChunkSize, 10},
	}

	for _, tt := range tests {
		got := ChunkOf(tt.userID)
		if got != tt.chunk {
			t.Errorf("ChunkOf(%d) = %d, want %d", tt.userID, got, tt.chunk)
		}
	}
}

func TestBitOf(t *testing.T) {
	tests := []struct {
		userID int64
		bit    int64
	}{
		{0, 0},
		{1, 1},
		{ChunkSize - 1, ChunkSize - 1},
		{ChunkSize, 0},
		{ChunkSize + 1, 1},
		{ChunkSize + 42, 42},
	}

	for _, tt := range tests {
		got := BitOf(tt.userID)
		if got != tt.bit {
			t.Errorf("BitOf(%d) = %d, want %d", tt.userID, got, tt.bit)
		}
	}
}

func TestChunkBitRoundTrip(t *testing.T) {
	// For any userID, chunk*ChunkSize + bit should equal the original.
	for _, uid := range []int64{0, 1, 42, 32767, 32768, 100000, 999999} {
		chunk := ChunkOf(uid)
		bit := BitOf(uid)
		reconstructed := chunk*ChunkSize + bit
		if reconstructed != uid {
			t.Errorf("round-trip failed for %d: chunk=%d, bit=%d, reconstructed=%d",
				uid, chunk, bit, reconstructed)
		}
	}
}

func TestKey(t *testing.T) {
	key := Key("like", "post", 456, 0)
	expected := "bm:like:post:456:0"
	if key != expected {
		t.Errorf("Key = %q, want %q", key, expected)
	}

	key2 := Key("fav", "article", 789, 3)
	expected2 := "bm:fav:article:789:3"
	if key2 != expected2 {
		t.Errorf("Key = %q, want %q", key2, expected2)
	}
}

func TestKeyPattern(t *testing.T) {
	pattern := KeyPattern("like", "post", 456)
	expected := "bm:like:post:456:*"
	if pattern != expected {
		t.Errorf("KeyPattern = %q, want %q", pattern, expected)
	}
}

func TestChunkSize(t *testing.T) {
	if ChunkSize != 32768 {
		t.Error("ChunkSize changed — update migration docs and key builders")
	}
}
