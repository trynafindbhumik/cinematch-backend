package routes

import (
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/swaggo/files"
	"github.com/swaggo/gin-swagger"
	"github.com/trynafindbhumik/cinematch-backend/internal/config"
	"github.com/trynafindbhumik/cinematch-backend/internal/modules/auth"

	_ "github.com/trynafindbhumik/cinematch-backend/docs"
)

func SetupRouter(authHandler *auth.Handler) *gin.Engine {
	// Set GIN mode based on environment
	if config.App.Environment == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.Default()

	// CORS middleware using gin-contrib/cors
	r.Use(cors.New(cors.Config{
		AllowOrigins:     config.App.AllowedOrigins,
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Authorization", "Content-Type", "X-Requested-With", "X-Device-Name"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	// Health check (public)
	// @Summary Health check
	// @Description Returns OK if server is running
	// @Tags health
	// @Produce json
	// @Success 200 {object} map[string]string
	// @Router /health [get]
	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	// Swagger docs
	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	// Auth routes
	authGroup := r.Group("/v1/auth")
	{
		authGroup.POST("/signup", authHandler.Signup)
		authGroup.POST("/verify", authHandler.Verify)
		authGroup.POST("/resend", authHandler.ResendVerification)
		authGroup.POST("/login", authHandler.Login)
		authGroup.POST("/logout", authHandler.Logout)
		authGroup.POST("/forgot-password", authHandler.ForgotPassword)
		authGroup.POST("/reset-password", authHandler.ResetPassword)
	}

	return r
}