package countint

import (
	"testing"
)

func TestEncodeDecodeUserCounts(t *testing.T) {
	tests := []struct {
		following, follower int64
	}{
		{0, 0},
		{100, 50},
		{1 << 20, 1 << 30},
		{-1, -1}, // negative should still encode/decode correctly
	}

	for _, tt := range tests {
		blob := EncodeUserCounts(tt.following, tt.follower)
		if len(blob) != UserBlobSize {
			t.Errorf("blob size = %d, want %d", len(blob), UserBlobSize)
		}
		following, follower := DecodeUserCounts(blob)
		if following != tt.following || follower != tt.follower {
			t.Errorf("EncodeDecodeUserCounts(%d,%d) = (%d,%d)",
				tt.following, tt.follower, following, follower)
		}
	}
}

func TestEncodeDecodePostLikeCount(t *testing.T) {
	tests := []int64{0, 1, 42, 99999, 1 << 40}

	for _, like := range tests {
		blob := EncodePostLikeCount(like)
		if len(blob) != PostBlobSize {
			t.Errorf("blob size = %d, want %d", len(blob), PostBlobSize)
		}
		got := DecodePostLikeCount(blob)
		if got != like {
			t.Errorf("EncodeDecodePostLikeCount(%d) = %d", like, got)
		}
	}
}

func TestNewBlobs(t *testing.T) {
	ub := NewUserBlob()
	if len(ub) != UserBlobSize {
		t.Errorf("NewUserBlob size = %d", len(ub))
	}
	f, fl := DecodeUserCounts(ub)
	if f != 0 || fl != 0 {
		t.Errorf("NewUserBlob = (%d,%d), want (0,0)", f, fl)
	}

	pb := NewPostBlob()
	if len(pb) != PostBlobSize {
		t.Errorf("NewPostBlob size = %d", len(pb))
	}
	if l := DecodePostLikeCount(pb); l != 0 {
		t.Errorf("NewPostBlob like = %d, want 0", l)
	}
}

func TestDecodeShortData(t *testing.T) {
	// Too-short data should return 0, not panic.
	if f, fl := DecodeUserCounts([]byte{1, 2, 3}); f != 0 || fl != 0 {
		t.Errorf("short user data should return zeros")
	}
	if l := DecodePostLikeCount([]byte{1, 2, 3}); l != 0 {
		t.Errorf("short post data should return 0")
	}
	if l := DecodePostLikeCount(nil); l != 0 {
		t.Errorf("nil data should return 0")
	}
}

func TestFieldIndependence(t *testing.T) {
	// Verify that modifying one field doesn't affect the other.
	blob := EncodeUserCounts(100, 200)

	// Corrupt the follower field and verify following is unchanged.
	for i := UserFollowerOffset; i < UserBlobSize; i++ {
		blob[i] = 0xFF
	}
	following, _ := DecodeUserCounts(blob)
	if following != 100 {
		t.Errorf("following should still be 100 after corrupting follower bytes, got %d", following)
	}

	// Fresh blob: corrupt following, verify follower unchanged.
	blob2 := EncodeUserCounts(300, 400)
	for i := UserFollowingOffset; i < UserFollowerOffset; i++ {
		blob2[i] = 0x00
	}
	_, follower := DecodeUserCounts(blob2)
	if follower != 400 {
		t.Errorf("follower should still be 400 after zeroing following bytes, got %d", follower)
	}
}
