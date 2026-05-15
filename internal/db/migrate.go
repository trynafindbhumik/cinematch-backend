package db

import (
	"embed"
	"fmt"
	"os"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/trynafindbhumik/cinematch-backend/internal/shared/logger"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

func RunMigrations() error {
	source, err := iofs.New(migrationsFS, "migrations")
	if err != nil {
		return fmt.Errorf("failed to create migration source: %w", err)
	}

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		return fmt.Errorf("DATABASE_URL environment variable is not set")
	}

	// Open migrations from embedded FS
	m, err := migrate.NewWithSourceInstance(
		"iofs",
		source,
		databaseURL,
	)
	if err != nil {
		return fmt.Errorf("failed to create migrate instance: %w", err)
	}
	defer func() {
		_, err := m.Close()
		if err != nil {
			logger.Error("Migration close error", logger.Err(err))
		}
	}()

	// Run pending migrations
	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	return nil
}
