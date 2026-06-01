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

	result, err := service.VoteForPost(userID, &p)
	if err != nil {
		zap.L().Error("service.VoteForPost() failed", zap.Error(err))
		response.ResponseError(c, response.CodeFail)
		return
	}

	// Return changed + liked so the client can:
	//  - changed=false → ignore duplicate clicks gracefully
	//  - liked=true/false → update button state without a second request
	response.ResponseSuccess(c, result)
}
