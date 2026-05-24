package routes

import (
	"ceddit/controller"
	"ceddit/middleware"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
)

func Setup() *gin.Engine {
	if viper.GetString("app.mode") == gin.ReleaseMode {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.New()

	r.Use(middleware.GinLogger(), middleware.GinRecovery(true))

	r.POST("/signup", controller.SignUpHandler)
	r.POST("/login", controller.LogInHandler)
	r.POST("/islogin", middleware.JWTAuthMiddleware(), func(c *gin.Context) {
		c.String(http.StatusOK, "yes")
	})

	return r
}
