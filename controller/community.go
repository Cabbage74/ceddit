package controller

import (
	"ceddit/pkg/response"
	"ceddit/service"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func CommunityHandler(c *gin.Context) {
	data, err := service.GetCommunityList()
	if err != nil {
		zap.L().Error("service.GetCommunityList() fail", zap.Error(err))
		response.ResponseError(c, response.CodeFail)
		return
	}
	response.ResponseSuccess(c, data)
}
