package controller

import (
	"ceddit/models"
	"ceddit/pkg/response"
	"ceddit/pkg/snowflake"
	"ceddit/service"
	"strconv"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func CreateDraftHandler(c *gin.Context) {
	userID, err := getCurrentUserID(c)
	if err != nil {
		response.ResponseError(c, response.CodeNeedAuth)
		return
	}

	postID := snowflake.GenID()
	if err := service.CreateDraft(userID, postID); err != nil {
		zap.L().Error("service.CreateDraft() failed", zap.Error(err))
		response.ResponseError(c, response.CodeFail)
		return
	}

	response.ResponseSuccess(c, &models.RespCreateDraft{PostID: postID})
}

func PresignHandler(c *gin.Context) {
	var p models.ParamPresign
	if err := c.ShouldBindJSON(&p); err != nil {
		response.ResponseError(c, response.CodeInvalidParam)
		return
	}

	userID, err := getCurrentUserID(c)
	if err != nil {
		response.ResponseError(c, response.CodeNeedAuth)
		return
	}

	data, err := service.GetPresignedURL(userID, &p)
	if err != nil {
		zap.L().Error("service.GetPresignedURL() failed", zap.Error(err))
		response.ResponseErrorWithMsg(c, response.CodeFail, err.Error())
		return
	}

	response.ResponseSuccess(c, data)
}

func ConfirmContentHandler(c *gin.Context) {
	postID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.ResponseError(c, response.CodeInvalidParam)
		return
	}

	var p models.ParamContentConfirm
	if err := c.ShouldBindJSON(&p); err != nil {
		response.ResponseError(c, response.CodeInvalidParam)
		return
	}

	userID, err := getCurrentUserID(c)
	if err != nil {
		response.ResponseError(c, response.CodeNeedAuth)
		return
	}

	if err := service.ConfirmContent(postID, userID, &p); err != nil {
		zap.L().Error("service.ConfirmContent() failed", zap.Error(err))
		response.ResponseErrorWithMsg(c, response.CodeContentNotOK, err.Error())
		return
	}

	response.ResponseSuccess(c, nil)
}

func PatchPostHandler(c *gin.Context) {
	postID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.ResponseError(c, response.CodeInvalidParam)
		return
	}

	var p models.ParamPatchPost
	if err := c.ShouldBindJSON(&p); err != nil {
		response.ResponseError(c, response.CodeInvalidParam)
		return
	}

	userID, err := getCurrentUserID(c)
	if err != nil {
		response.ResponseError(c, response.CodeNeedAuth)
		return
	}

	if err := service.PatchPost(postID, userID, &p); err != nil {
		zap.L().Error("service.PatchPost() failed", zap.Error(err))
		response.ResponseErrorWithMsg(c, response.CodeFail, err.Error())
		return
	}

	response.ResponseSuccess(c, nil)
}

func PublishHandler(c *gin.Context) {
	postID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.ResponseError(c, response.CodeInvalidParam)
		return
	}

	userID, err := getCurrentUserID(c)
	if err != nil {
		response.ResponseError(c, response.CodeNeedAuth)
		return
	}

	if err := service.PublishPost(postID, userID); err != nil {
		zap.L().Error("service.PublishPost() failed", zap.Error(err))
		response.ResponseErrorWithMsg(c, response.CodeFail, err.Error())
		return
	}

	response.ResponseSuccess(c, nil)
}

func PostDetailHandler(c *gin.Context) {
	postID := c.Param("id")
	id, err := strconv.ParseInt(postID, 10, 64)
	if err != nil {
		response.ResponseError(c, response.CodeInvalidParam)
		return
	}
	data, err := service.GetPost(id)
	if err != nil {
		zap.L().Error("service.GetPost() failed", zap.Error(err))
		response.ResponseError(c, response.CodeFail)
		return
	}
	response.ResponseSuccess(c, data)
}

func EasyPostHandler(c *gin.Context) {
	page, size := getPageInfo(c)
	data, err := service.GetEasyPostList(page, size)
	if err != nil {
		zap.L().Error("service.GetPostList() failed", zap.Error(err))
		response.ResponseError(c, response.CodeFail)
		return
	}
	response.ResponseSuccess(c, data)
}

func PostHandler(c *gin.Context) {
	p := &models.ParamPostList{
		Page:  1,
		Size:  10,
		Order: models.OrderTime,
	}
	if err := c.ShouldBindQuery(p); err != nil {
		response.ResponseError(c, response.CodeInvalidParam)
		return
	}

	data, err := service.GetPostList(p)
	if err != nil {
		zap.L().Error("service.GetPostList() failed", zap.Error(err))
		response.ResponseError(c, response.CodeFail)
		return
	}
	response.ResponseSuccess(c, data)
}
