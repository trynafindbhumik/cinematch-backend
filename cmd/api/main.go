package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/trynafindbhumik/cinematch-backend/internal/config"
	"github.com/trynafindbhumik/cinematch-backend/internal/db"
	"github.com/trynafindbhumik/cinematch-backend/internal/modules/auth"
	"github.com/trynafindbhumik/cinematch-backend/internal/routes"
	"github.com/trynafindbhumik/cinematch-backend/internal/shared/email"
	"github.com/trynafindbhumik/cinematch-backend/internal/shared/jwt"
)

func main() {
	config.Load()
	config.LoadAuthConfig()
	email.LoadEmailConfig()

	// Set JWT signing key
	jwt.SetJWTSigningKey(config.Auth.JWTSigningKey)

	ctx := context.Background()

	if err := db.Connect(ctx); err != nil {
		log.Fatal("DB connection failed:", err)
	}
	defer db.Close()

	// Run database migrations
	if err := db.RunMigrations(); err != nil {
		log.Fatal("Migration failed:", err)
	}
	fmt.Println("Migrations applied successfully!")

	// Initialize auth module
	authRepo := auth.NewRepository()
	authService := auth.NewService(authRepo)
	authHandler := auth.NewHandler(authService)

	// Setup router
	router := routes.SetupRouter(authHandler)

	// Create HTTP server
	server := &http.Server{
		Addr:         ":" + config.App.Port,
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start server in goroutine
	go func() {
		fmt.Printf("Server running on port %s\n", config.App.Port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal("Server failed:", err)
		}
	}()

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	fmt.Println("Shutting down server...")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Fatal("Server forced to shutdown:", err)
	}
	fmt.Println("Server exited")
}
