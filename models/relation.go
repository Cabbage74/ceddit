package models

import "time"

// Following represents an active follow relationship (physical delete on unfollow).
type Following struct {
	ID         int64     `db:"id" json:"id,string"`
	FromUserID int64     `db:"from_user_id" json:"from_user_id,string"`
	ToUserID   int64     `db:"to_user_id" json:"to_user_id,string"`
	CreatedAt  time.Time `db:"created_at" json:"created_at"`
	UpdatedAt  time.Time `db:"updated_at" json:"updated_at"`
}

// Follower is the async projection table optimized for "who follows me" queries.
type Follower struct {
	ID          int64     `db:"id" json:"id,string"`
	ToUserID    int64     `db:"to_user_id" json:"to_user_id,string"`
	FromUserID  int64     `db:"from_user_id" json:"from_user_id,string"`
	CreatedAt   time.Time `db:"created_at" json:"created_at"`
	UpdatedAt   time.Time `db:"updated_at" json:"updated_at"`
	LastEventID *string   `db:"last_event_id" json:"last_event_id,omitempty"`
}

// Outbox is the event table whose binlog is tailed by Canal.
type Outbox struct {
	ID            int64     `db:"id" json:"id,string"`
	AggregateType string    `db:"aggregate_type" json:"aggregate_type"`
	AggregateID   int64     `db:"aggregate_id" json:"aggregate_id,string"`
	Type          string    `db:"type" json:"type"`
	Payload       string    `db:"payload" json:"payload"`
	CreatedAt     time.Time `db:"created_at" json:"created_at"`
}

// OutboxPayload is the JSON stored in outbox.payload.
type OutboxPayload struct {
	EventID    string `json:"event_id"`
	FromUserID int64  `json:"from_user_id,string"`
	ToUserID   int64  `json:"to_user_id,string"`
	OccurredAt string `json:"occurred_at"` // RFC3339 millis
}

// FollowListItem is one entry in "who I follow" list.
type FollowListItem struct {
	UserID    int64     `json:"user_id,string"`
	Username  string    `json:"username"`
	CreatedAt time.Time `json:"created_at"`
}

// FollowerListItem is one entry in "who follows me" list.
type FollowerListItem struct {
	UserID    int64     `json:"user_id,string"`
	Username  string    `json:"username"`
	IsMutual  bool      `json:"is_mutual"`
	CreatedAt time.Time `json:"created_at"`
}

// RespFollowList is the response for following/follower list queries.
type RespFollowList struct {
	List       []FollowListItem `json:"list"`
	NextCursor string           `json:"next_cursor,omitempty"`
	Total      int64            `json:"total"`
}

// RespFollowerList is the response for follower list queries.
type RespFollowerList struct {
	List       []FollowerListItem `json:"list"`
	NextCursor string             `json:"next_cursor,omitempty"`
	Total      int64              `json:"total"`
}

// RespRelation is the response after a follow/unfollow action.
type RespRelation struct {
	ID         int64     `json:"id,string"`
	FromUserID int64     `json:"from_user_id,string"`
	ToUserID   int64     `json:"to_user_id,string"`
	CreatedAt  time.Time `json:"created_at"`
}
