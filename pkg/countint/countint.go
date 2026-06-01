// Package countint implements a Redis SDS (String) compatible binary counter
// layout. Multiple int64 count fields are packed into a single contiguous byte
// slice, eliminating per-field key and metadata overhead.
//
// Layout (little-endian int64):
//
//	User (16 bytes): [following_count:8][follower_count:8]
//	Post  (8 bytes): [like_count:8]
package countint

import "encoding/binary"

// ---- User CountInt ----

const (
	UserFollowingOffset = 0 // byte offset of following_count
	UserFollowerOffset  = 8 // byte offset of follower_count
	UserBlobSize        = 16
)

// ---- Post CountInt ----

const (
	PostLikeOffset = 0 // byte offset of like_count
	PostBlobSize   = 8
)

// NewUserBlob returns a zero-filled user CountInt blob.
func NewUserBlob() []byte {
	return make([]byte, UserBlobSize)
}

// NewPostBlob returns a zero-filled post CountInt blob.
func NewPostBlob() []byte {
	return make([]byte, PostBlobSize)
}

// ---- encode helpers ----

// EncodeUserCounts packs following and follower counts into a 16-byte blob.
func EncodeUserCounts(following, follower int64) []byte {
	b := make([]byte, UserBlobSize)
	binary.LittleEndian.PutUint64(b[UserFollowingOffset:], uint64(following))
	binary.LittleEndian.PutUint64(b[UserFollowerOffset:], uint64(follower))
	return b
}

// EncodePostLikeCount packs a like count into an 8-byte blob.
func EncodePostLikeCount(like int64) []byte {
	b := make([]byte, PostBlobSize)
	binary.LittleEndian.PutUint64(b[PostLikeOffset:], uint64(like))
	return b
}

// ---- decode helpers ----

// DecodeUserCounts unpacks a 16-byte blob into following and follower counts.
// Returns (0, 0) if data is shorter than expected.
func DecodeUserCounts(data []byte) (following, follower int64) {
	if len(data) < UserBlobSize {
		return 0, 0
	}
	following = int64(binary.LittleEndian.Uint64(data[UserFollowingOffset:]))
	follower = int64(binary.LittleEndian.Uint64(data[UserFollowerOffset:]))
	return
}

// DecodePostLikeCount unpacks an 8-byte blob into a like count.
// Returns 0 if data is shorter than expected.
func DecodePostLikeCount(data []byte) int64 {
	if len(data) < PostBlobSize {
		return 0
	}
	return int64(binary.LittleEndian.Uint64(data[PostLikeOffset:]))
}
