package controller

import (
	"ceddit/models"
	"ceddit/pkg/auth"
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

	pair, refreshToken, err := service.SignUp(&p)
	if err != nil {
		zap.L().Error("signup failed", zap.Error(err))
		response.ResponseErrorWithMsg(c, response.CodeFail, err.Error())
		return
	}

	auth.SetRefreshCookie(c, refreshToken)
	response.ResponseSuccess(c, pair)
}

func LogInHandler(c *gin.Context) {
	var p models.ParamLogIn
	if err := c.ShouldBindJSON(&p); err != nil {
		response.ResponseError(c, response.CodeInvalidParam)
		return
	}

	pair, refreshToken, err := service.LogIn(&p)
	if err != nil {
		zap.L().Error("login failed", zap.Error(err))
		response.ResponseErrorWithMsg(c, response.CodeFail, err.Error())
		return
	}

	auth.SetRefreshCookie(c, refreshToken)
	response.ResponseSuccess(c, pair)
}

func RefreshHandler(c *gin.Context) {
	oldToken, err := auth.GetRefreshTokenFromCookie(c)
	if err != nil {
		response.ResponseErrorWithMsg(c, response.CodeInvalidAuth, "missing refresh token")
		return
	}

	pair, newRefreshToken, err := service.RefreshTokens(oldToken)
	if err == auth.ErrTokenLeaked {
		zap.L().Warn("refresh token leak detected")
		auth.ClearRefreshCookie(c)
		response.ResponseErrorWithMsg(c, response.CodeInvalidAuth, err.Error())
		return
	}
	if err != nil {
		zap.L().Error("refresh failed", zap.Error(err))
		auth.ClearRefreshCookie(c)
		response.ResponseErrorWithMsg(c, response.CodeInvalidAuth, err.Error())
		return
	}

	auth.SetRefreshCookie(c, newRefreshToken)
	response.ResponseSuccess(c, pair)
}

func LogoutHandler(c *gin.Context) {
	refreshToken, err := auth.GetRefreshTokenFromCookie(c)
	if err != nil {
		// No cookie — already effectively logged out.
		auth.ClearRefreshCookie(c)
		response.ResponseSuccess(c, nil)
		return
	}

	// Look up the refresh token to find the user.
	val, err := auth.GetRefreshValue(refreshToken)
	if err != nil {
		// Token not found in Redis — nothing to revoke.
		auth.ClearRefreshCookie(c)
		response.ResponseSuccess(c, nil)
		return
	}

	if err := service.RevokeUserTokens(val.UserID); err != nil {
		zap.L().Error("revoke tokens failed", zap.Error(err))
		response.ResponseErrorWithMsg(c, response.CodeFail, err.Error())
		return
	}

	auth.ClearRefreshCookie(c)
	response.ResponseSuccess(c, nil)
}
