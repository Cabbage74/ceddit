package controller

import (
	"ceddit/pkg/response"
	"ceddit/service"
	"database/sql"
	"errors"
	"strconv"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// FollowHandler handles POST /api/v1/users/:to_user_id/follow
func FollowHandler(c *gin.Context) {
	fromUserID, err := getCurrentUserID(c)
	if err != nil {
		response.ResponseError(c, response.CodeNeedAuth)
		return
	}

	toUserID, err := strconv.ParseInt(c.Param("to_user_id"), 10, 64)
	if err != nil {
		response.ResponseError(c, response.CodeInvalidParam)
		return
	}

	idempotencyKey := c.GetHeader("Idempotency-Key")
	if idempotencyKey == "" {
		response.ResponseErrorWithMsg(c, response.CodeInvalidParam, "Idempotency-Key header is required")
		return
	}

	resp, err := service.Follow(fromUserID, toUserID, idempotencyKey)
	if err != nil {
		zap.L().Error("follow failed",
			zap.Int64("from", fromUserID),
			zap.Int64("to", toUserID),
			zap.Error(err),
		)
		response.ResponseErrorWithMsg(c, response.CodeFail, err.Error())
		return
	}

	response.ResponseSuccess(c, resp)
}

// UnfollowHandler handles POST /api/v1/users/:to_user_id/unfollow
func UnfollowHandler(c *gin.Context) {
	fromUserID, err := getCurrentUserID(c)
	if err != nil {
		response.ResponseError(c, response.CodeNeedAuth)
		return
	}

	toUserID, err := strconv.ParseInt(c.Param("to_user_id"), 10, 64)
	if err != nil {
		response.ResponseError(c, response.CodeInvalidParam)
		return
	}

	idempotencyKey := c.GetHeader("Idempotency-Key")
	if idempotencyKey == "" {
		response.ResponseErrorWithMsg(c, response.CodeInvalidParam, "Idempotency-Key header is required")
		return
	}

	err = service.Unfollow(fromUserID, toUserID, idempotencyKey)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) || errors.Is(err, service.ErrNotFollowing) {
			response.ResponseErrorWithMsg(c, response.CodeFail, "not following this user")
			return
		}
		zap.L().Error("unfollow failed",
			zap.Int64("from", fromUserID),
			zap.Int64("to", toUserID),
			zap.Error(err),
		)
		response.ResponseErrorWithMsg(c, response.CodeFail, err.Error())
		return
	}

	response.ResponseSuccess(c, nil)
}

// FollowingListHandler handles GET /api/v1/users/:user_id/following
func FollowingListHandler(c *gin.Context) {
	userID, err := strconv.ParseInt(c.Param("user_id"), 10, 64)
	if err != nil {
		response.ResponseError(c, response.CodeInvalidParam)
		return
	}

	cursor := c.Query("cursor")
	limitStr := c.DefaultQuery("limit", "20")
	limit, err := strconv.Atoi(limitStr)
	if err != nil {
		limit = 20
	}

	resp, err := service.GetFollowingList(userID, cursor, limit)
	if err != nil {
		zap.L().Error("get following list failed", zap.Error(err))
		response.ResponseErrorWithMsg(c, response.CodeFail, err.Error())
		return
	}

	response.ResponseSuccess(c, resp)
}

// FollowerListHandler handles GET /api/v1/users/:user_id/followers
func FollowerListHandler(c *gin.Context) {
	userID, err := strconv.ParseInt(c.Param("user_id"), 10, 64)
	if err != nil {
		response.ResponseError(c, response.CodeInvalidParam)
		return
	}

	cursor := c.Query("cursor")
	limitStr := c.DefaultQuery("limit", "20")
	limit, err := strconv.Atoi(limitStr)
	if err != nil {
		limit = 20
	}

	resp, err := service.GetFollowerList(userID, cursor, limit)
	if err != nil {
		zap.L().Error("get follower list failed", zap.Error(err))
		response.ResponseErrorWithMsg(c, response.CodeFail, err.Error())
		return
	}

	response.ResponseSuccess(c, resp)
}
