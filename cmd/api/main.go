package main

// CineMatch Backend API
// Main entry point that initializes and starts the HTTP server.

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/trynafindbhumik/cinematch-backend/internal/config"
	"github.com/trynafindbhumik/cinematch-backend/internal/db"
	"github.com/trynafindbhumik/cinematch-backend/internal/modules/auth"
	"github.com/trynafindbhumik/cinematch-backend/internal/modules/export"
	"github.com/trynafindbhumik/cinematch-backend/internal/modules/favorites"
	"github.com/trynafindbhumik/cinematch-backend/internal/modules/genres"
	"github.com/trynafindbhumik/cinematch-backend/internal/modules/movies"
	"github.com/trynafindbhumik/cinematch-backend/internal/modules/profile"
	"github.com/trynafindbhumik/cinematch-backend/internal/modules/reactions"
	"github.com/trynafindbhumik/cinematch-backend/internal/modules/reviews"
	"github.com/trynafindbhumik/cinematch-backend/internal/modules/streaming_services"
	"github.com/trynafindbhumik/cinematch-backend/internal/modules/suggestion_tries"
	"github.com/trynafindbhumik/cinematch-backend/internal/modules/suggestions"
	"github.com/trynafindbhumik/cinematch-backend/internal/modules/watched"
	"github.com/trynafindbhumik/cinematch-backend/internal/modules/watchlist"
	"github.com/trynafindbhumik/cinematch-backend/internal/modules/weekly_suggestions"
	"github.com/trynafindbhumik/cinematch-backend/internal/routes"
	"github.com/trynafindbhumik/cinematch-backend/internal/shared/cloudinary"
	"github.com/trynafindbhumik/cinematch-backend/internal/shared/email"
	"github.com/trynafindbhumik/cinematch-backend/internal/shared/gemini"
	"github.com/trynafindbhumik/cinematch-backend/internal/shared/jwt"
	"github.com/trynafindbhumik/cinematch-backend/internal/shared/logger"
	"github.com/trynafindbhumik/cinematch-backend/internal/shared/redis"
	"github.com/trynafindbhumik/cinematch-backend/internal/shared/tmdb"
)

//	@title			CineMatch API
//	@version		1.0
//	@description	Backend API for CineMatch movie platform
//	@termsOfService	http://swagger.io/terms/

//	@contact.name	API Support
//	@contact.email	support@cinematch.com

//	@license.name	Apache 2.0
//	@license.url	http://www.apache.org/licenses/LICENSE-2.0.html

//	@securityDefinitions.apikey	BearerAuth
//	@in							header
//	@name						Authorization
//	@description				Type "Bearer" followed by your access token (e.g., "Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...")

//	@host		cinematch-backend-6m8y.onrender.com
//	@basePath	/
//	@schemes	https

func main() {
	// Load config from environment
	config.Load()
	config.LoadAuthConfig()
	email.LoadEmailConfig()

	logger.Info("Starting CineMatch Backend",
		logger.String("environment", config.App.Environment),
		logger.String("port", config.App.Port),
	)

	// Set JWT signing key from config
	jwt.SetJWTSigningKey(config.Auth.JWTSigningKey)

	// Initialize Cloudinary for image uploads
	cloudinary.Load()

	ctx := context.Background()

	// Connect to database
	if err := db.Connect(ctx); err != nil {
		logger.Fatal("Database connection failed", logger.Err(err))
	}
	defer db.Close()

	// Connect to Redis (caching and session storage)
	// If Redis fails, app continues without caching
	if err := redis.Load(); err != nil {
		logger.Warn("Redis connection failed, caching disabled", logger.Err(err))
	} else {
		defer redis.Close()
	}

	// Run database migrations
	if err := db.RunMigrations(); err != nil {
		logger.Fatal("Database migration failed", logger.Err(err))
	}
	logger.Info("Database migrations applied successfully")

	// Initialize modules
	authRepo := auth.NewRepository()
	authService := auth.NewService(authRepo)
	authHandler := auth.NewHandler(authService)

	profileRepo := profile.NewRepository()
	profileService := profile.NewService(profileRepo, authRepo)
	profileHandler := profile.NewHandler(profileService)

	genresRepo := genres.NewRepository()
	genresService := genres.NewService(genresRepo)
	genresHandler := genres.NewHandler(genresService)

	// Initialize TMDB client for movie data
	if err := tmdb.Load(); err != nil {
		logger.Warn("TMDB client initialization failed", logger.Err(err))
	}
	tmdbClient := tmdb.NewClient()

	moviesRepo := movies.NewRepository(tmdbClient, genresRepo)
	moviesService := movies.NewService(moviesRepo)
	moviesHandler := movies.NewHandler(moviesService)

	favoritesRepo := favorites.NewRepository()
	favoritesService := favorites.NewService(favoritesRepo, tmdbClient)
	favoritesHandler := favorites.NewHandler(favoritesService)

	watchlistRepo := watchlist.NewRepository()
	watchlistService := watchlist.NewService(watchlistRepo, tmdbClient)
	watchlistHandler := watchlist.NewHandler(watchlistService)

	watchedRepo := watched.NewRepository()
	watchedService := watched.NewService(watchedRepo, tmdbClient)
	watchedHandler := watched.NewHandler(watchedService)

	exportRepo := export.NewRepository()
	exportService := export.NewService(exportRepo)
	exportHandler := export.NewHandler(exportService)

	streamingServicesRepo := streaming_services.NewRepository()
	streamingServicesService := streaming_services.NewService(streamingServicesRepo)
	streamingServicesHandler := streaming_services.NewHandler(streamingServicesService)

	reviewsRepo := reviews.NewRepository(db.GetDB(), tmdbClient)
	reviewsService := reviews.NewService(reviewsRepo, db.GetDB(), tmdbClient)
	reviewsHandler := reviews.NewHandler(reviewsService)

	if err := gemini.Load(); err != nil {
		logger.Warn("Gemini client initialization failed", logger.Err(err))
	}
	geminiClient := gemini.NewClient()
	suggestionsRepo := suggestions.NewRepository()
	suggestionsService := suggestions.NewService(suggestionsRepo, geminiClient, moviesService)
	suggestionsHandler := suggestions.NewHandler(suggestionsService)

	weeklySuggestionsRepo := weekly_suggestions.NewRepository()
	weeklySuggestionsService := weekly_suggestions.NewService(weeklySuggestionsRepo, tmdbClient, geminiClient)
	weeklySuggestionsHandler := weekly_suggestions.NewHandler(weeklySuggestionsService)

	suggestionTriesRepo := suggestion_tries.NewRepository()
	suggestionTriesService := suggestion_tries.NewService(suggestionTriesRepo, tmdbClient, geminiClient)
	suggestionTriesHandler := suggestion_tries.NewHandler(suggestionTriesService)

	reactionsRepo := reactions.NewRepository()
	reactionsService := reactions.NewService(reactionsRepo, tmdbClient)
	reactionsHandler := reactions.NewHandler(reactionsService)

	// Setup routes and start HTTP server
	router := routes.SetupRouter(authHandler, profileHandler, moviesHandler, favoritesHandler, watchlistHandler, watchedHandler, exportHandler, genresHandler, streamingServicesHandler, reviewsHandler, suggestionsHandler, suggestionTriesHandler, weeklySuggestionsHandler, reactionsHandler)

	server := &http.Server{
		Addr:         ":" + config.App.Port,
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 120 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start server in background
	go func() {
		logger.Info("Server starting",
			logger.String("port", config.App.Port),
			logger.String("swagger_url", "http://localhost:"+config.App.Port+"/swagger/index.html"),
		)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("Server failed to start", logger.Err(err))
		}
	}()

	// Wait for shutdown signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("Shutting down server...")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Fatal("Server forced to shutdown", logger.Err(err))
	}
	logger.Info("Server exited gracefully")
	defer logger.Sync()
}
