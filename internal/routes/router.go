package routes

// Route configuration and middleware setup for the API.

import (
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	"github.com/trynafindbhumik/cinematch-backend/internal/config"
	"github.com/trynafindbhumik/cinematch-backend/internal/modules/auth"
	"github.com/trynafindbhumik/cinematch-backend/internal/modules/export"
	"github.com/trynafindbhumik/cinematch-backend/internal/modules/favorites"
	"github.com/trynafindbhumik/cinematch-backend/internal/modules/genres"
	"github.com/trynafindbhumik/cinematch-backend/internal/modules/movies"
	"github.com/trynafindbhumik/cinematch-backend/internal/modules/profile"
	"github.com/trynafindbhumik/cinematch-backend/internal/modules/reviews"
	"github.com/trynafindbhumik/cinematch-backend/internal/modules/streaming_services"
	"github.com/trynafindbhumik/cinematch-backend/internal/modules/suggestions"
	"github.com/trynafindbhumik/cinematch-backend/internal/modules/suggestion_tries"
	"github.com/trynafindbhumik/cinematch-backend/internal/modules/watched"
	"github.com/trynafindbhumik/cinematch-backend/internal/modules/watchlist"
	"github.com/trynafindbhumik/cinematch-backend/internal/modules/weekly_suggestions"
	"github.com/trynafindbhumik/cinematch-backend/internal/modules/reactions"
	"github.com/trynafindbhumik/cinematch-backend/internal/shared/logger"
	"github.com/trynafindbhumik/cinematch-backend/internal/shared/middleware"

	_ "github.com/trynafindbhumik/cinematch-backend/docs"
)

func SetupRouter(
	authHandler *auth.Handler,
	profileHandler *profile.Handler,
	moviesHandler *movies.Handler,
	favoritesHandler *favorites.Handler,
	watchlistHandler *watchlist.Handler,
	watchedHandler *watched.Handler,
	exportHandler *export.Handler,
	genresHandler *genres.Handler,
	streamingServicesHandler *streaming_services.Handler,
	reviewsHandler *reviews.Handler,
	suggestionsHandler *suggestions.Handler,
	suggestionTriesHandler *suggestion_tries.Handler,
	weeklySuggestionsHandler *weekly_suggestions.Handler,
	reactionsHandler *reactions.Handler,
) *gin.Engine {
	// Set Gin mode based on environment
	if config.App.Environment == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.New()

	// Add panic recovery middleware
	r.Use(gin.Recovery())

	// Add request logging middleware
	r.Use(logger.RequestLogger())

	// CORS configuration
	r.Use(cors.New(cors.Config{
		AllowOrigins:     config.App.AllowedOrigins,
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Authorization", "Content-Type", "X-Requested-With", "X-Device-Name"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	// Health check endpoint (public)
	//	@Summary		Health check
	//	@Description	Returns OK if server is running
	//	@Tags			health
	//	@Produce		json
	//	@Success		200	{object}	map[string]string
	//	@Router			/health [get]
	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	// Swagger documentation
	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	// Auth routes (public)
	authGroup := r.Group("/v1/auth")
	{
		authGroup.POST("/signup", authHandler.Signup)
		authGroup.POST("/verify", authHandler.Verify)
		authGroup.POST("/resend", authHandler.ResendVerification)
		authGroup.POST("/resend-reset", authHandler.ResendReset)
		authGroup.POST("/login", authHandler.Login)
		authGroup.POST("/logout", authHandler.Logout)
		authGroup.POST("/refresh", authHandler.RefreshToken)
		authGroup.POST("/forgot-password", authHandler.ForgotPassword)
		authGroup.POST("/reset-password", authHandler.ResetPassword)
	}

	// Auth routes (protected - require authentication)
	authProtectedGroup := r.Group("/v1/auth")
	authProtectedGroup.Use(profile.AuthMiddleware())
	{
		authProtectedGroup.POST("/init-verify", authHandler.InitVerify)
	}

	// Auth routes for sessions (supports both Bearer token and magic link)
	authSessionsGroup := r.Group("/v1/auth")
	{
		authSessionsGroup.GET("/sessions", authHandler.GetAllSessions)
		authSessionsGroup.DELETE("/sessions", authHandler.DeleteSession)
	}

	// Profile routes (protected)
	profileGroup := r.Group("/v1/profile")
	profileGroup.Use(profile.AuthMiddleware())
	{
		profileGroup.GET("/me", profileHandler.GetProfile)
		profileGroup.PUT("/me", profileHandler.UpdateProfile)
		profileGroup.DELETE("/me", profileHandler.DeleteAccount)
		profileGroup.DELETE("/me/picture", profileHandler.DeleteProfilePicture)
		profileGroup.PUT("/password", profileHandler.ChangePassword)
		profileGroup.PUT("/disable", profileHandler.DisableAccount)
		profileGroup.POST("/email/change", profileHandler.InitiateEmailChange)
		profileGroup.POST("/email/resend", profileHandler.ResendEmailChange)
		profileGroup.POST("/verify", profileHandler.VerifyEmail)
	}

	// Movies routes (protected)
	moviesGroup := r.Group("/v1/movies")
	moviesGroup.Use(profile.AuthMiddleware())
	{
		moviesGroup.GET("/search", moviesHandler.Search)
		moviesGroup.GET("/trending", moviesHandler.GetTrending)
		moviesGroup.GET("/:tmdb_id", moviesHandler.GetByID)
		moviesGroup.GET("/:tmdb_id/videos", moviesHandler.GetVideos)
		moviesGroup.GET("/:tmdb_id/reviews", reviewsHandler.GetMovieReviews)
	}

	// Favorites routes (protected)
	favoritesGroup := r.Group("/v1/favorites")
	favoritesGroup.Use(profile.AuthMiddleware())
	{
		favoritesGroup.GET("", favoritesHandler.GetFavorites)
		favoritesGroup.GET("/ids", favoritesHandler.GetFavoriteIDs)
		favoritesGroup.GET("/search", favoritesHandler.SearchFavorites)
		favoritesGroup.POST("", favoritesHandler.AddFavorites)
		favoritesGroup.DELETE("/:id", favoritesHandler.DeleteFavorite)
	}

	// Watchlist routes (protected)
	watchlistGroup := r.Group("/v1/watchlist")
	watchlistGroup.Use(profile.AuthMiddleware())
	{
		watchlistGroup.GET("", watchlistHandler.GetWatchlist)
		watchlistGroup.GET("/ids", watchlistHandler.GetWatchlistIDs)
		watchlistGroup.GET("/search", watchlistHandler.SearchWatchlist)
		watchlistGroup.POST("", watchlistHandler.AddToWatchlist)
		watchlistGroup.DELETE("/:id", watchlistHandler.DeleteFromWatchlist)
	}

	// Watched routes (protected)
	watchedGroup := r.Group("/v1/watched")
	watchedGroup.Use(profile.AuthMiddleware())
	{
		watchedGroup.GET("", watchedHandler.GetWatched)
		watchedGroup.GET("/ids", watchedHandler.GetWatchedIDs)
		watchedGroup.GET("/search", watchedHandler.SearchWatched)
		watchedGroup.POST("", watchedHandler.AddToWatched)
		watchedGroup.DELETE("/:id", watchedHandler.DeleteFromWatched)
	}

	// Export routes (protected)
	exportGroup := r.Group("/v1/export")
	exportGroup.Use(profile.AuthMiddleware())
	{
		exportGroup.POST("", exportHandler.ExportData)
	}

	// Genres routes (protected)
	genresGroup := r.Group("/v1/genres")
	genresGroup.Use(profile.AuthMiddleware())
	{
		genresGroup.GET("", genresHandler.GetAllGenres)
		genresGroup.GET("/mine", genresHandler.GetUserGenres)
		genresGroup.POST("/:genreId", genresHandler.AddUserGenre)
		genresGroup.DELETE("/:genreId", genresHandler.RemoveUserGenre)
	}

	// Streaming Services routes (protected)
	streamingServicesGroup := r.Group("/v1/streaming-services")
	streamingServicesGroup.Use(profile.AuthMiddleware())
	{
		streamingServicesGroup.GET("", streamingServicesHandler.GetAllStreamingServices)
		streamingServicesGroup.GET("/search", streamingServicesHandler.SearchStreamingServices)
		streamingServicesGroup.GET("/mine", streamingServicesHandler.GetUserStreamingServices)
		streamingServicesGroup.PUT("", streamingServicesHandler.UpdateUserStreamingServices)
		streamingServicesGroup.DELETE("/bulk", streamingServicesHandler.RemoveUserStreamingServicesBulk)
		streamingServicesGroup.DELETE("/:serviceId", streamingServicesHandler.RemoveUserStreamingService)
	}

	// Reviews routes (protected)
	reviewsGroup := r.Group("/v1/reviews")
	reviewsGroup.Use(profile.AuthMiddleware())
	{
		reviewsGroup.GET("", reviewsHandler.GetUserReviews)
		reviewsGroup.POST("", reviewsHandler.CreateReview)
		reviewsGroup.PATCH("/:id", reviewsHandler.UpdateReview)
		reviewsGroup.DELETE("/:id", reviewsHandler.DeleteReview)
	}

	// Suggestions routes (protected)
	suggestionsGroup := r.Group("/v1/suggestions")
	suggestionsGroup.Use(profile.AuthMiddleware())
	suggestionsGroup.Use(middleware.Timeout(middleware.TimeoutConfig{Timeout: 120 * time.Second}))
	{
		suggestionsGroup.GET("/generate", suggestionsHandler.GenerateSuggestions)
		suggestionsGroup.GET("/next", suggestionsHandler.GetNext)
	}

	// Weekly Suggestions routes (protected)
	weeklySuggestionsGroup := r.Group("/v1/weekly-suggestions")
	weeklySuggestionsGroup.Use(profile.AuthMiddleware())
	weeklySuggestionsGroup.Use(middleware.Timeout(middleware.TimeoutConfig{Timeout: 120 * time.Second}))
	{
		weeklySuggestionsGroup.GET("", weeklySuggestionsHandler.GetWeeklySuggestions)
	}

	// Suggestion Tries routes (protected) - 3 tries per week
	suggestionTriesGroup := r.Group("/v1/suggestion-tries")
	suggestionTriesGroup.Use(profile.AuthMiddleware())
	suggestionTriesGroup.Use(middleware.Timeout(middleware.TimeoutConfig{Timeout: 120 * time.Second}))
	{
		suggestionTriesGroup.GET("/generate", suggestionTriesHandler.GenerateSuggestions)
	}

	// Reactions routes (protected)
	reactionsGroup := r.Group("/v1/reactions")
	reactionsGroup.Use(profile.AuthMiddleware())
	{
		reactionsGroup.POST("", reactionsHandler.AddReaction)
	}

	return r
}
