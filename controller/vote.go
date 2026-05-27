package controller

import (
	"ceddit/models"
	"ceddit/pkg/response"
	"ceddit/service"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func VoteHandler(c *gin.Context) {
	var p models.ParamVote
	if err := c.ShouldBindJSON(&p); err != nil {
		response.ResponseError(c, response.CodeInvalidParam)
		return
	}
	userID, err := getCurrentUserID(c)
	if err != nil {
		response.ResponseError(c, response.CodeNeedAuth)
		return
	}
	if err := service.VoteForPost(userID, &p); err != nil {
		zap.L().Error("service.VoteForPost() failed", zap.Error(err))
		response.ResponseError(c, response.CodeFail)
		return
	}
	response.ResponseSuccess(c, nil)
}
