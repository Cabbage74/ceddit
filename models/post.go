package models

import "time"

type Post struct {
	PostID           int64      `json:"post_id,string" db:"id"`
	Title            *string    `json:"title" db:"title"`
	Description      *string    `json:"description" db:"description"`
	ContentObjectKey *string    `json:"content_object_key" db:"content_object_key"`
	ContentETag      *string    `json:"content_etag" db:"content_etag"`
	ContentSize      *int64     `json:"content_size" db:"content_size"`
	ContentSHA256    *string    `json:"content_sha256" db:"content_sha256"`
	AuthorID         int64      `json:"author_id,string" db:"author_id"`
	Status           string     `json:"status" db:"status"`
	CreateTime       *time.Time `json:"create_time" db:"create_time"`
	PublishTime      *time.Time `json:"publish_time" db:"publish_time"`
	UpdateTime       *time.Time `json:"update_time" db:"update_time"`
}

type PostDetail struct {
	AuthorName string `json:"author_name"`
	VoteNum    int64  `json:"vote_num"`
	Content    string `json:"content,omitempty"`
	*Post
}

type ParamPresign struct {
	Scene       string `json:"scene" binding:"required"`
	PostID      int64  `json:"post_id,string" binding:"required"`
	ContentType string `json:"content_type" binding:"required"`
	Ext         string `json:"ext" binding:"required"`
}

type ParamContentConfirm struct {
	ObjectKey string `json:"object_key" binding:"required"`
	ETag      string `json:"etag" binding:"required"`
	Size      int64  `json:"size" binding:"required"`
	SHA256    string `json:"sha256" binding:"required"`
}

type ParamPatchPost struct {
	Title string `json:"title" binding:"required"`
}

type RespPresign struct {
	ObjectKey string `json:"object_key"`
	PutURL    string `json:"put_url"`
	ExpiresIn int    `json:"expires_in"`
}

type RespCreateDraft struct {
	PostID int64 `json:"post_id,string"`
}
