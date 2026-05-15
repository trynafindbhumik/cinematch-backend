// Package movie_collection provides shared functionality for user's movie collections.
// This file contains Redis caching utilities shared by all collection modules.
package movie_collection

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/trynafindbhumik/cinematch-backend/internal/modules/movies"
	"github.com/trynafindbhumik/cinematch-backend/internal/shared/redis"
)

// Cache TTL constants
const (
	MovieCacheTTL    = 60 * 60 // 1 hour
	TrendingCacheTTL = 30 * 60 // 30 minutes
)

// CacheKeyPrefixes for different cache types
const (
	CacheKeyMovie    = "movie:"
	CacheKeyTrending = "movies:trending:"
)

// Cache provides Redis caching functionality for movie collections
type Cache struct{}

// NewCache creates a new cache instance
func NewCache() *Cache {
	return &Cache{}
}

// GetMovie retrieves a movie from Redis cache.
// It checks both individual movie cache and trending caches.
func (c *Cache) GetMovie(ctx context.Context, tmdbID int) (*MovieInput, error) {
	rdb := redis.Client()
	if rdb == nil {
		return nil, fmt.Errorf("redis not available")
	}

	cacheKey := fmt.Sprintf("%s%d", CacheKeyMovie, tmdbID)
	data, err := rdb.Get(ctx, cacheKey).Bytes()
	if err == nil {
		var movie MovieInput
		if err := json.Unmarshal(data, &movie); err == nil {
			return &movie, nil
		}
	}

	// If not in individual cache, check all trending caches (up to 500 pages)
	return c.searchTrendingCache(ctx, tmdbID)
}

// SetMovie stores a movie in Redis cache
func (c *Cache) SetMovie(ctx context.Context, tmdbID int, movie MovieInput) error {
	rdb := redis.Client()
	if rdb == nil {
		return fmt.Errorf("redis not available")
	}

	cacheKey := fmt.Sprintf("%s%d", CacheKeyMovie, tmdbID)
	jsonData, err := json.Marshal(movie)
	if err != nil {
		return fmt.Errorf("failed to marshal movie: %w", err)
	}

	if err := rdb.Set(ctx, cacheKey, jsonData, MovieCacheTTL*time.Second).Err(); err != nil {
		return fmt.Errorf("failed to cache movie: %w", err)
	}

	return nil
}

// searchTrendingCache searches through trending caches for a movie
func (c *Cache) searchTrendingCache(ctx context.Context, tmdbID int) (*MovieInput, error) {
	rdb := redis.Client()
	if rdb == nil {
		return nil, fmt.Errorf("redis not available")
	}

	for page := 1; page <= 500; page++ {
		trendingKey := fmt.Sprintf("%s%d", CacheKeyTrending, page)
		trendingData, err := rdb.Get(ctx, trendingKey).Bytes()
		if err != nil {
			continue
		}

		var trendingResp movies.TrendingResponse
		if err := json.Unmarshal(trendingData, &trendingResp); err != nil {
			continue
		}

		for _, m := range trendingResp.Movies {
			if m.TMDBID == tmdbID {
				// Only return if movie has genres, otherwise fall through to TMDB
				if len(m.Genres) > 0 {
					return &MovieInput{
						TMDBID:      m.TMDBID,
						Title:       m.Title,
						PosterURL:   m.PosterURL,
						BackdropURL: m.BackdropURL,
						ReleaseYear: m.ReleaseYear,
						TMDBRating:  m.TMDBRating,
						Genres:      m.Genres,
					}, nil
				}
			}
		}
	}

	return nil, fmt.Errorf("movie not found in cache")
}

// ClearCache removes a movie from cache
func (c *Cache) ClearCache(ctx context.Context, tmdbID int) error {
	rdb := redis.Client()
	if rdb == nil {
		return fmt.Errorf("redis not available")
	}

	cacheKey := fmt.Sprintf("%s%d", CacheKeyMovie, tmdbID)
	return rdb.Del(ctx, cacheKey).Err()
}
