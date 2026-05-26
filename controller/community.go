package controller

import (
	"ceddit/pkg/response"
	"ceddit/service"
	"strconv"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func CommunityHandler(c *gin.Context) {
	data, err := service.GetCommunityList()
	if err != nil {
		zap.L().Error("service.GetCommunityList() failed", zap.Error(err))
		response.ResponseError(c, response.CodeFail)
		return
	}
	response.ResponseSuccess(c, data)
}

func CommunityDetailHandler(c *gin.Context) {
	communityID := c.Param("id")
	id, err := strconv.ParseInt(communityID, 10, 64)
	if err != nil {
		response.ResponseError(c, response.CodeInvalidParam)
		return
	}
	data, err := service.GetCommunity(id)
	if err != nil {
		zap.L().Error("service.GetCommunity() failed", zap.Error(err))
		response.ResponseError(c, response.CodeFail)
		return
	}
	response.ResponseSuccess(c, data)
}
