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
	"github.com/trynafindbhumik/cinematch-backend/internal/middleware"
)

func main() {
	config.Load()

	ctx := context.Background()

	if err := db.Connect(ctx); err != nil {
		log.Fatal("DB connection failed:", err)
	}
	defer db.Close()

	fmt.Println("DB connected successfully!")

	// Run database migrations
	if err := db.RunMigrations(); err != nil {
		log.Fatal("Migration failed:", err)
	}
	fmt.Println("Migrations applied successfully!")

	// Create HTTP server with CORS middleware
	mux := http.NewServeMux()
	
	// Health check endpoint
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	})

	// Wrap with CORS middleware
	handler := middleware.CORS()(mux)

	server := &http.Server{
		Addr:         ":" + config.App.Port,
		Handler:      handler,
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
