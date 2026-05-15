// fetch_streaming_services.go
// Usage: go run scripts/fetch_streaming_services.go
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
)

const (
	baseURL = "https://api.themoviedb.org/3"
)

var apiKey string
var dbPool *pgxpool.Pool

type WatchProvidersResponse struct {
	Results []Provider `json:"results"`
}

type Provider struct {
	ProviderID      int    `json:"provider_id"`
	ProviderName    string `json:"provider_name"`
	LogoPath        string `json:"logo_path"`
	DisplayPriority int    `json:"display_priority"`
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	godotenv.Load()

	apiKey = os.Getenv("TMDB_API_KEY")
	if apiKey == "" {
		return fmt.Errorf("TMDB_API_KEY environment variable not set")
	}

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		return fmt.Errorf("DATABASE_URL environment variable not set")
	}

	ctx := context.Background()

	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		return fmt.Errorf("failed to ping database: %w", err)
	}
	dbPool = pool

	fmt.Println("Connected to database")
	fmt.Println("Fetching streaming services from TMDB...")

	providers, err := fetchAllProviders(ctx)
	if err != nil {
		return fmt.Errorf("failed to fetch providers: %w", err)
	}

	fmt.Printf("Found %d providers, inserting into database...\n", len(providers))

	for _, p := range providers {
		logoURL := ""
		if p.LogoPath != "" {
			logoURL = fmt.Sprintf("https://image.tmdb.org/t/p/w92%s", p.LogoPath)
		}

		_, err := dbPool.Exec(ctx, `
			INSERT INTO streaming_services (name, icon_url, tmdb_id)
			VALUES ($1, $2, $3)
			ON CONFLICT (name) DO UPDATE SET
				icon_url = EXCLUDED.icon_url,
				tmdb_id = EXCLUDED.tmdb_id
		`, p.ProviderName, logoURL, p.ProviderID)

		if err != nil {
			fmt.Printf("Warning: failed to insert %s: %v\n", p.ProviderName, err)
		} else {
			fmt.Printf("Inserted/Updated: %s (TMDB ID: %d)\n", p.ProviderName, p.ProviderID)
		}
	}

	fmt.Println("\nDone! Streaming services synced from TMDB.")

	count := 0
	dbPool.QueryRow(ctx, "SELECT COUNT(*) FROM streaming_services").Scan(&count)
	fmt.Printf("Total streaming services in database: %d\n", count)

	return nil
}

func fetchAllProviders(ctx context.Context) ([]Provider, error) {
	var allProviders []Provider
	seen := make(map[int]bool)

	regions := []string{"US", "GB", "IN", "AU", "CA", "DE", "FR", "BR"}

	for _, region := range regions {
		providers, err := fetchProvidersByRegion(ctx, region)
		if err != nil {
			fmt.Printf("Warning: failed to fetch providers for %s: %v\n", region, err)
			continue
		}

		for _, p := range providers {
			if !seen[p.ProviderID] {
				seen[p.ProviderID] = true
				allProviders = append(allProviders, p)
			}
		}
	}

	return allProviders, nil
}

func fetchProvidersByRegion(ctx context.Context, region string) ([]Provider, error) {
	url := fmt.Sprintf("%s/watch/providers/movie?api_key=%s&watch_region=%s", baseURL, apiKey, region)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("TMDB API returned status %d: %s", resp.StatusCode, string(body))
	}

	var result WatchProvidersResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return result.Results, nil
}