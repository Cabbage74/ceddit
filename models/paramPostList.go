package models

const (
	OrderTime  = "time"
	OrderScore = "score"
)

type ParamPostList struct {
	Page  int64  `json:"page" form:"page"`
	Size  int64  `json:"size" form:"size"`
	Order string `json:"order" form:"order"`
}
