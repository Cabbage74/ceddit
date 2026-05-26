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

func CreatePostHandler(c *gin.Context) {
	var p models.Post
	if err := c.ShouldBindJSON(&p); err != nil {
		response.ResponseError(c, response.CodeInvalidParam)
		return
	}
	
	p.PostID = snowflake.GenID()
	userID, err := getCurrentUserID(c)
	if err != nil {
		response.ResponseError(c, response.CodeNeedAuth)
		return
	}
	p.AuthorID = userID
	
	if err := service.CreatePost(&p); err != nil {
		zap.L().Error("service.CreatePost() failed", zap.Error(err))
		response.ResponseError(c, response.CodeFail)
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


func PostHandler(c *gin.Context) {
	page, size := getPageInfo(c)
	data, err := service.GetPostList(page, size)
	if err != nil {
		zap.L().Error("service.GetPostList() failed", zap.Error(err))
		response.ResponseError(c, response.CodeFail)
	}
	response.ResponseSuccess(c, data)
}
