package controller

import (
	"ceddit/models"
	"ceddit/service"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func SignUpHandler(c *gin.Context) {
	var p models.ParamSignUp
	if err := c.ShouldBindJSON(&p); err != nil {
		ResponseError(c, CodeInvalidParam)
		return
	}

	if err := service.SignUp(&p); err != nil {
		zap.L().Error("", zap.Error(err))
		ResponseErrorWithMsg(c, CodeFail, err.Error())
		return
	}

	ResponseSuccess(c, nil)
}

func LogInHandler(c *gin.Context) {
	var p models.ParamLogin
	if err := c.ShouldBindJSON(&p); err != nil {
		ResponseError(c, CodeInvalidParam)
		return
	}

	if err := service.LogIn(&p); err != nil {
		zap.L().Error("", zap.Error(err))
		ResponseErrorWithMsg(c, CodeFail, err.Error())
		return
	}

	ResponseSuccess(c, nil)
}
