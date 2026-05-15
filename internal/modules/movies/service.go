package movies

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/trynafindbhumik/cinematch-backend/internal/shared/redis"
)

const (
	// Cache TTLs
	TrendingCacheTTL = 30 * 60 // 30 minutes
	SearchCacheTTL   = 15 * 60 // 15 minutes
	MovieCacheTTL    = 60 * 60 // 1 hour
)

// Service handles movie business logic
type Service struct {
	repo *Repository
}

// NewService creates a new movies service
func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

// SearchMovies searches movies with Redis caching
func (s *Service) SearchMovies(ctx context.Context, query string, page int) (*SearchResponse, error) {
	// Check cache first
	cacheKey := fmt.Sprintf("movies:search:%s:%d", query, page)
	cached, err := s.getCachedResponse(ctx, cacheKey)
	if err == nil && cached != nil {
		return cached.(*SearchResponse), nil
	}

	// Fetch from TMDB
	result, err := s.repo.SearchMovies(ctx, query, page)
	if err != nil {
		return nil, err
	}

	// Cache the result
	s.cacheResponse(ctx, cacheKey, result, SearchCacheTTL)

	return result, nil
}

// GetTrending gets trending movies with Redis caching
func (s *Service) GetTrending(ctx context.Context, page int) (*TrendingResponse, error) {
	// Check cache first
	cacheKey := fmt.Sprintf("movies:trending:%d", page)
	cached, err := s.getCachedResponse(ctx, cacheKey)
	if err == nil && cached != nil {
		return cached.(*TrendingResponse), nil
	}

	// Fetch from TMDB
	result, err := s.repo.GetTrending(ctx, page)
	if err != nil {
		return nil, err
	}

	// Cache the result
	s.cacheResponse(ctx, cacheKey, result, TrendingCacheTTL)

	return result, nil
}

// GetMovieByID gets movie details by TMDB ID
// Movie details from TMDB are cached, but vote counts and user reaction are always fetched fresh
func (s *Service) GetMovieByID(ctx context.Context, tmdbID int, userID int64) (*MovieDetailsResponse, error) {
	// Check cache first for base movie data
	cacheKey := fmt.Sprintf("movies:detail:%d", tmdbID)
	cached, err := s.getCachedResponse(ctx, cacheKey)
	if err == nil && cached != nil {
		result := cached.(*MovieDetailsResponse).DeepCopy()
		// Always fetch fresh votes and user reaction from DB (not cached)
		s.fillVotesAndReaction(ctx, result, tmdbID, userID)
		return result, nil
	}

	// Fetch from TMDB
	result, err := s.repo.GetMovieByID(ctx, tmdbID)
	if err != nil {
		return nil, err
	}

	// Ensure movie exists in our DB
	_ = s.repo.EnsureMovieExists(ctx, tmdbID, result.Title, result.PosterURL, result.BackdropURL, result.ReleaseYear, result.TMDBRating, result.Genres)

	// Fetch fresh votes and user reaction from DB (not cached)
	s.fillVotesAndReaction(ctx, result, tmdbID, userID)

	// Cache the result
	s.cacheResponse(ctx, cacheKey, result, MovieCacheTTL)

	return result, nil
}

// fillVotesAndReaction fetches vote counts and user reaction fresh from DB
func (s *Service) fillVotesAndReaction(ctx context.Context, result *MovieDetailsResponse, tmdbID int, userID int64) {
	// Get vote counts from our DB (always fresh)
	votes, _ := s.repo.GetMovieVotes(ctx, tmdbID)
	if votes != nil {
		result.TotalCount = votes.TotalCount
		result.LikeCount = votes.LikeCount
		result.LoveCount = votes.LoveCount
		result.DislikeCount = votes.DislikeCount
		result.HateCount = votes.HateCount
		result.SkipCount = votes.SkipCount
	}

	// Get user's reaction for this movie (always fresh)
	if userID > 0 {
		reaction, _ := s.repo.GetUserReaction(ctx, tmdbID, userID)
		result.UserReaction = reaction
	}
}

// GetMovieVideos gets movie videos with caching
func (s *Service) GetMovieVideos(ctx context.Context, tmdbID int) (*VideosResponse, error) {
	cacheKey := fmt.Sprintf("movies:videos:%d", tmdbID)
	cached, err := s.getCachedVideosResponse(ctx, cacheKey)
	if err == nil && cached != nil {
		return cached, nil
	}

	result, err := s.repo.GetMovieVideos(ctx, tmdbID)
	if err != nil {
		return nil, err
	}

	s.cacheVideosResponse(ctx, cacheKey, result, MovieCacheTTL)

	return result, nil
}

// getCachedResponse retrieves cached response from Redis
func (s *Service) getCachedResponse(ctx context.Context, key string) (interface{}, error) {
	rdb := redis.Client()
	if rdb == nil {
		return nil, fmt.Errorf("redis not available")
	}

	data, err := rdb.Get(ctx, key).Bytes()
	if err != nil {
		return nil, err
	}

	switch {
	case hasPrefix(key, "movies:trending"):
		var resp TrendingResponse
		if err := json.Unmarshal(data, &resp); err != nil {
			return nil, err
		}
		return &resp, nil
	case hasPrefix(key, "movies:search"):
		var resp SearchResponse
		if err := json.Unmarshal(data, &resp); err != nil {
			return nil, err
		}
		return &resp, nil
	case hasPrefix(key, "movies:detail"):
		var resp MovieDetailsResponse
		if err := json.Unmarshal(data, &resp); err != nil {
			return nil, err
		}
		return &resp, nil
	default:
		return nil, fmt.Errorf("unknown cache key format")
	}
}

func (s *Service) getCachedVideosResponse(ctx context.Context, key string) (*VideosResponse, error) {
	rdb := redis.Client()
	if rdb == nil {
		return nil, fmt.Errorf("redis not available")
	}

	data, err := rdb.Get(ctx, key).Bytes()
	if err != nil {
		return nil, err
	}

	var resp VideosResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// cacheResponse stores response in Redis
func (s *Service) cacheResponse(ctx context.Context, key string, data interface{}, ttl int) {
	rdb := redis.Client()
	if rdb == nil {
		return
	}

	jsonData, err := json.Marshal(data)
	if err != nil {
		return
	}

	rdb.Set(ctx, key, jsonData, time.Duration(ttl)*time.Second)
}

func (s *Service) cacheVideosResponse(ctx context.Context, key string, data *VideosResponse, ttl int) {
	rdb := redis.Client()
	if rdb == nil {
		return
	}

	jsonData, err := json.Marshal(data)
	if err != nil {
		return
	}

	rdb.Set(ctx, key, jsonData, time.Duration(ttl)*time.Second)
}

func hasPrefix(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}