package models

import "time"

type Community struct {
	CommunityID   int64      `json:"community_id,string" db:"community_id"`
	CommunityName string     `json:"community_name" db:"community_name"`
	Introdution   string     `json:"introduction,omitempty" db:"introduction"`
	CreateTime    *time.Time `json:"create_time,omitempty" db:"create_time"`
}
