package middleware

import (
	"ceddit/pkg/response"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/juju/ratelimit"
)

func RateLimitMiddleware(fillInterval time.Duration, cap int64) func(c *gin.Context) {
	bucket := ratelimit.NewBucket(fillInterval, cap)
	return func(c *gin.Context) {
		if bucket.TakeAvailable(1) == 0 {
			response.ResponseError(c, response.CodeFail)
			c.Abort()
			return
		}
		c.Next()
	}
}
