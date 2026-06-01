package routes

import (
	"ceddit/controller"
	"ceddit/middleware"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
)

func Setup() *gin.Engine {
	if viper.GetString("app.mode") == gin.ReleaseMode {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.New()

	r.Use(middleware.GinLogger(), middleware.GinRecovery(true), middleware.RateLimitMiddleware(20*time.Millisecond, 50))

	v1 := r.Group("/api/v1")

	// Public endpoints (no auth required).
	v1.POST("/signup", controller.SignUpHandler)
	v1.POST("/login", controller.LogInHandler)
	v1.POST("/refresh", controller.RefreshHandler)
	v1.POST("/logout", controller.LogoutHandler)

	// Public feed (auth optional — liked/faved overlaid if JWT present).
	v1.GET("/feed", middleware.JWTAuthOptionalMiddleware(), controller.PublicFeedHandler)

	// Protected endpoints.
	v1.Use(middleware.JWTAuthMiddleware())
	{
		v1.POST("/post/draft", controller.CreateDraftHandler)
		v1.POST("/storage/presign", controller.PresignHandler)
		v1.POST("/post/:id/content/confirm", controller.ConfirmContentHandler)
		v1.PATCH("/post/:id", controller.PatchPostHandler)
		v1.POST("/post/:id/publish", controller.PublishHandler)

		v1.GET("/easypost", controller.EasyPostHandler)
		v1.GET("/post/:id", controller.PostDetailHandler)
		v1.GET("/post", controller.PostHandler)

		v1.POST("/vote", controller.VoteHandler)

		// User relations
		v1.POST("/users/:to_user_id/follow", controller.FollowHandler)
		v1.POST("/users/:to_user_id/unfollow", controller.UnfollowHandler)
		v1.GET("/users/:user_id/following", controller.FollowingListHandler)
		v1.GET("/users/:user_id/followers", controller.FollowerListHandler)
	}

	return r
}
