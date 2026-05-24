package controller

import (
	"ceddit/models"
	"ceddit/pkg/response"
	"ceddit/service"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func SignUpHandler(c *gin.Context) {
	var p models.ParamSignUp
	if err := c.ShouldBindJSON(&p); err != nil {
		response.ResponseError(c, response.CodeInvalidParam)
		return
	}

	if err := service.SignUp(&p); err != nil {
		zap.L().Error("", zap.Error(err))
		response.ResponseErrorWithMsg(c, response.CodeFail, err.Error())
		return
	}

	response.ResponseSuccess(c, nil)
}

func LogInHandler(c *gin.Context) {
	var p models.ParamLogin
	if err := c.ShouldBindJSON(&p); err != nil {
		response.ResponseError(c, response.CodeInvalidParam)
		return
	}

	token, err := service.LogIn(&p)
	if err != nil {
		zap.L().Error("", zap.Error(err))
		response.ResponseErrorWithMsg(c, response.CodeFail, err.Error())
		return
	}

	response.ResponseSuccess(c, token)
}
