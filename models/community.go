package models

type Community struct {
	CommunityID int64 `json:"community_id" db:"community_id"`
	CommunityName string `json:"community_name" db:"community_name"`
}
