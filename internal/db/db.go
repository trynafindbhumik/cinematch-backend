package db

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/trynafindbhumik/cinematch-backend/internal/shared/logger"
)

var pool *pgxpool.Pool

func Pool() *pgxpool.Pool {
	if pool == nil {
		panic("database not connected — call db.Connect() first")
	}
	return pool
}

// GetDB returns the database pool (alias for Pool for compatibility)
func GetDB() *pgxpool.Pool {
	return Pool()
}

func Connect(ctx context.Context) error {
	cfg, err := pgxpool.ParseConfig(os.Getenv("DATABASE_URL"))
	if err != nil {
		return fmt.Errorf("failed to parse DATABASE_URL: %w", err)
	}

	cfg.MaxConns = 10
	cfg.MinConns = 2
	cfg.MaxConnLifetime = time.Hour
	cfg.MaxConnIdleTime = 30 * time.Minute
	cfg.HealthCheckPeriod = 1 * time.Minute

	// Always use extended protocol for compatibility with Supabase pooler
	cfg.ConnConfig.DefaultQueryExecMode = pgx.QueryExecModeCacheDescribe

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	p, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return fmt.Errorf("failed to create connection pool: %w", err)
	}

	if err := p.Ping(ctx); err != nil {
		return fmt.Errorf("failed to ping database: %w", err)
	}

	pool = p

	logger.Info("Database connected successfully",
		logger.Int32("max_conns", cfg.MaxConns),
		logger.Int32("min_conns", cfg.MinConns),
	)
	return nil
}

func Close() {
	if pool != nil {
		pool.Close()
	}
}
