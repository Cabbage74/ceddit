package controller

import (
	"errors"
	"strconv"

	"github.com/gin-gonic/gin"
)

const (
	CtxUserIDKey = "userID"
)

func getCurrentUserID(c *gin.Context) (userID int64, err error) {
	uid, ok := c.Get(CtxUserIDKey)
	if !ok {
		err = errors.New("getCurrentUserID() failed")
		return
	}
	userID, ok = uid.(int64)
	if !ok {
		err = errors.New("getCurrentUserID() failed")
		return
	}
	return
}

func getPageInfo(c *gin.Context) (int64, int64) {
	pageStr := c.Query("page")
	sizeStr := c.Query("size")
	page, err := strconv.ParseInt(pageStr, 10, 64)
	if err != nil {
		page = 1
	}
	size, err := strconv.ParseInt(sizeStr, 10, 64)
	if err != nil {
		size = 10
	}

	return page, size
}
