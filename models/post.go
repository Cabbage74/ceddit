package models

import "time"

type Post struct {
	PostID      int64      `json:"post_id,string" db:"post_id"`
	AuthorID    int64      `json:"author_id,string" db:"author_id"`
	CommunityID int64      `json:"community_id,string" db:"community_id" binding:"required"`
	Status      int32      `json:"status" db:"status"`
	Title       string     `json:"title" db:"title" binding:"required"`
	Content     string     `json:"content" db:"content" binding:"required"`
	CreateTime  *time.Time `json:"create_time" db:"create_time"`
}

type PostDetail struct {
	AuthorName    string `json:"author_name"`
	CommunityName string `json:"community_name"`
	*Post
}
