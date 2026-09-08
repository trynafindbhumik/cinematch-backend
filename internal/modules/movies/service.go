package movies

import (
	"context"
	"fmt"
	"time"

	"github.com/trynafindbhumik/cinematch-backend/internal/shared/redis"
)

const (
	// Cache TTLs
	TrendingCacheTTL = 30 * time.Minute
	SearchCacheTTL   = 15 * time.Minute
	MovieCacheTTL    = 1 * time.Hour
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
	cacheKey := fmt.Sprintf("movies:search:%s:%d", query, page)
	if cached, err := redis.Get[SearchResponse](ctx, cacheKey); err == nil && cached != nil {
		return cached, nil
	}

	result, err := s.repo.SearchMovies(ctx, query, page)
	if err != nil {
		return nil, err
	}

	_ = redis.Set(ctx, cacheKey, result, SearchCacheTTL)

	return result, nil
}

// GetTrending gets trending movies with Redis caching
func (s *Service) GetTrending(ctx context.Context, page int) (*TrendingResponse, error) {
	cacheKey := fmt.Sprintf("movies:trending:%d", page)
	if cached, err := redis.Get[TrendingResponse](ctx, cacheKey); err == nil && cached != nil {
		return cached, nil
	}

	result, err := s.repo.GetTrending(ctx, page)
	if err != nil {
		return nil, err
	}

	_ = redis.Set(ctx, cacheKey, result, TrendingCacheTTL)

	return result, nil
}

// GetMovieByID gets movie details by TMDB ID
// Movie details from TMDB are cached, but vote counts and user reaction are always fetched fresh
func (s *Service) GetMovieByID(ctx context.Context, tmdbID int, userID int64) (*MovieDetailsResponse, error) {
	cacheKey := fmt.Sprintf("movies:detail:%d", tmdbID)
	if cached, err := redis.Get[MovieDetailsResponse](ctx, cacheKey); err == nil && cached != nil {
		result := cached.DeepCopy()
		s.fillVotesAndReaction(ctx, result, tmdbID, userID)
		return result, nil
	}

	result, err := s.repo.GetMovieByID(ctx, tmdbID)
	if err != nil {
		return nil, err
	}

	// Ensure movie exists in our DB
	_ = s.repo.EnsureMovieExists(ctx, tmdbID, result.Title, result.PosterURL, result.BackdropURL, result.ReleaseYear, result.TMDBRating, result.Genres)

	// Fetch fresh votes and user reaction from DB (not cached)
	s.fillVotesAndReaction(ctx, result, tmdbID, userID)

	// Cache base movie details
	_ = redis.Set(ctx, cacheKey, result, MovieCacheTTL)

	return result, nil
}

// fillVotesAndReaction fetches vote counts and user reaction fresh from DB
func (s *Service) fillVotesAndReaction(ctx context.Context, result *MovieDetailsResponse, tmdbID int, userID int64) {
	votes, _ := s.repo.GetMovieVotes(ctx, tmdbID)
	if votes != nil {
		result.TotalCount = votes.TotalCount
		result.LikeCount = votes.LikeCount
		result.LoveCount = votes.LoveCount
		result.DislikeCount = votes.DislikeCount
		result.HateCount = votes.HateCount
		result.SkipCount = votes.SkipCount
	}

	if userID > 0 {
		reaction, _ := s.repo.GetUserReaction(ctx, tmdbID, userID)
		result.UserReaction = reaction
	}
}

// GetMovieVideos gets movie videos with caching
func (s *Service) GetMovieVideos(ctx context.Context, tmdbID int) (*VideosResponse, error) {
	cacheKey := fmt.Sprintf("movies:videos:%d", tmdbID)
	if cached, err := redis.Get[VideosResponse](ctx, cacheKey); err == nil && cached != nil {
		return cached, nil
	}

	result, err := s.repo.GetMovieVideos(ctx, tmdbID)
	if err != nil {
		return nil, err
	}

	_ = redis.Set(ctx, cacheKey, result, MovieCacheTTL)

	return result, nil
}