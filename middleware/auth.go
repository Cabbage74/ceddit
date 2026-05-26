package middleware

import (
	"ceddit/controller"
	"ceddit/pkg/jwt"
	"ceddit/pkg/response"
	"strings"

	"github.com/gin-gonic/gin"
)

func JWTAuthMiddleware() func(c *gin.Context) {
	return func(c *gin.Context) {
		authHeader := c.Request.Header.Get("Authorization")
		if authHeader == "" {
			response.ResponseError(c, response.CodeNeedAuth)
			c.Abort()
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if !(len(parts) == 2 && parts[0] == "Bearer") {
			response.ResponseError(c, response.CodeInvalidAuth)
			c.Abort()
			return
		}

		myClaims, err := jwt.ParseToken(parts[1])
		if err != nil {
			response.ResponseErrorWithMsg(c, response.CodeInvalidAuth, err.Error())
			c.Abort()
			return
		}

		c.Set(controller.CtxUserIDKey, myClaims.UserID)
		c.Next()
	}
}
