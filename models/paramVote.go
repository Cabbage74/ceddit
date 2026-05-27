package models

type ParamVote struct {
	PostID int64 `json:"post_id,string" binding:"required"`
	// 1 means up, 0 means neural, -1 means down
	Direction int `json:"direction" binding:"oneof=1 0 -1"`
}
