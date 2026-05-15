package favorites

// Favorites service for managing user's favorite movies.

import (
	"context"
	"strconv"

	"github.com/trynafindbhumik/cinematch-backend/internal/modules/movie_collection"
	"github.com/trynafindbhumik/cinematch-backend/internal/shared/logger"
	"github.com/trynafindbhumik/cinematch-backend/internal/shared/tmdb"
	"go.uber.org/zap"
)

type Service struct {
	repo       *Repository
	cache      *movie_collection.Cache
	tmdbClient *tmdb.Client
}

func NewService(repo *Repository, tmdbClient *tmdb.Client) *Service {
	return &Service{
		repo:       repo,
		cache:      movie_collection.NewCache(),
		tmdbClient: tmdbClient,
	}
}

// GetFavorites retrieves user's favorites with pagination and genre filter
func (s *Service) GetFavorites(ctx context.Context, userID int64, cursor string, limit int, genre string) (*GetFavoritesResponse, error) {
	favMovies, totalCount, nextCursor, err := s.repo.GetFavorites(ctx, userID, cursor, limit, genre)
	if err != nil {
		return nil, err
	}

	return &GetFavoritesResponse{
		Favorites:  favMovies,
		NextCursor: nextCursor,
		Limit:      limit,
		TotalCount: totalCount,
	}, nil
}

// GetFavoriteIDs retrieves all TMDB IDs for user's favorites
func (s *Service) GetFavoriteIDs(ctx context.Context, userID int64) (*GetFavoriteIDsResponse, error) {
	tmdbIDs, err := s.repo.GetFavoriteIDs(ctx, userID)
	if err != nil {
		return nil, err
	}

	return &GetFavoriteIDsResponse{
		TMDBIDs: tmdbIDs,
	}, nil
}

// DeleteFavorite removes a movie from favorites by user_movies.id
func (s *Service) DeleteFavorite(ctx context.Context, userID int64, id int64) error {
	return s.repo.DeleteFavoriteByID(ctx, userID, id)
}

// AddFavoritesResult contains the result of adding multiple favorites
type AddFavoritesResult struct {
	Added    []int           `json:"added"`
	Failed   map[int]string  `json:"failed,omitempty"` // tmdbID -> error message
}

func contains(slice []int, item int) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// AddFavorites adds multiple movies to favorites using batch operations
//
// Flow:
// 1. Batch check DB for all tmdbIDs (1 query)
// 2. For movies NOT in DB → check cache
// 3. For movies NOT in cache → fetch from TMDB
// 4. Batch upsert all movies needing insert (from cache or TMDB)
// 5. Batch add all to user_movies favorites
func (s *Service) AddFavorites(ctx context.Context, userID int64, tmdbIDs []int) (*AddFavoritesResult, error) {
	if len(tmdbIDs) == 0 {
		return &AddFavoritesResult{Added: []int{}}, nil
	}

	result := &AddFavoritesResult{
		Added:  make([]int, 0, len(tmdbIDs)),
		Failed: make(map[int]string),
	}

	// Step 1: Batch check which movies already exist in DB
	existingMovies, err := s.repo.GetMoviesByTMDBIDs(ctx, tmdbIDs)
	if err != nil {
		return nil, err
	}

	// Categorize tmdbIDs
	notInDB := make([]int, 0, len(tmdbIDs)) // need to check cache or TMDB
	for _, tmdbID := range tmdbIDs {
		if existing, ok := existingMovies[tmdbID]; ok && len(existing.Genres) > 0 {
			// Already in DB - will add directly
			logger.Info("AddFavorites: movie found in DB",
				logger.Int("tmdb_id", tmdbID),
				logger.Int("user_id", int(userID)),
				zap.String("source", "db"),
			)
		} else {
			notInDB = append(notInDB, tmdbID)
		}
	}

	// Step 2: For movies not in DB, check cache and fetch from TMDB if needed
	moviesToUpsert := make([]movie_collection.MovieInput, 0, len(notInDB))
	for _, tmdbID := range notInDB {
		cachedMovie, err := s.cache.GetMovie(ctx, tmdbID)
		if err == nil && cachedMovie != nil && len(cachedMovie.Genres) > 0 {
			moviesToUpsert = append(moviesToUpsert, *cachedMovie)
		} else {
			// Not in cache, need to fetch from TMDB
			details, err := s.tmdbClient.GetMovieDetails(tmdbID)
			if err != nil {
				result.Failed[tmdbID] = err.Error()
				continue
			}

			genres := make([]string, 0, len(details.Genres))
			for _, g := range details.Genres {
				genres = append(genres, g.Name)
			}

			year := 0
			if len(details.ReleaseDate) >= 4 {
				year, _ = strconv.Atoi(details.ReleaseDate[:4])
			}

			movieInput := movie_collection.MovieInput{
				TMDBID:      details.ID,
				Title:       details.Title,
				PosterURL:   tmdb.PosterURL(details.PosterPath),
				BackdropURL: tmdb.BackdropURL(details.BackdropPath),
				ReleaseYear: year,
				TMDBRating:  int(details.VoteAverage * 10),
				Genres:      genres,
			}
			moviesToUpsert = append(moviesToUpsert, movieInput)

			// Cache for future use
			_ = s.cache.SetMovie(ctx, tmdbID, movieInput)
		}
	}

	// Step 3: Batch upsert movies (from cache or TMDB) - only if we have any
	var newMovieDBIDs map[int]int
	if len(moviesToUpsert) > 0 {
		logger.Info("AddFavorites: upserting movies batch",
			logger.Int("user_id", int(userID)),
			logger.Int("movie_count", len(moviesToUpsert)),
		)
		newMovieDBIDs, err = s.repo.UpsertMoviesBatch(ctx, moviesToUpsert)
		if err != nil {
			logger.Error("AddFavorites: UpsertMoviesBatch failed",
				logger.Int("user_id", int(userID)),
				logger.Err(err),
			)
			return nil, err
		}
		logger.Info("AddFavorites: upsert complete",
			logger.Int("user_id", int(userID)),
			logger.Int("upserted_count", len(newMovieDBIDs)),
		)
	} else {
		newMovieDBIDs = make(map[int]int)
	}

	// Step 4: Collect all movieDBIDs to add to favorites
	allMovieDBIDs := make([]int64, 0, len(tmdbIDs))
	for _, tmdbID := range tmdbIDs {
		if existing, ok := existingMovies[tmdbID]; ok && len(existing.Genres) > 0 {
			allMovieDBIDs = append(allMovieDBIDs, int64(existing.ID))
		} else if dbID, ok := newMovieDBIDs[tmdbID]; ok {
			allMovieDBIDs = append(allMovieDBIDs, int64(dbID))
		}
	}

	logger.Info("AddFavorites: adding to user_movies",
		logger.Int("user_id", int(userID)),
		logger.Int("total_to_add", len(allMovieDBIDs)),
	)

	// Step 5: Batch add all to user_movies favorites
	if len(allMovieDBIDs) > 0 {
		if err := s.repo.AddToFavoritesBatch(ctx, userID, allMovieDBIDs); err != nil {
			logger.Error("AddFavorites: AddToFavoritesBatch failed",
				logger.Int("user_id", int(userID)),
				logger.Err(err),
			)
			return nil, err
		}
	}

	// Step 6: Build result - all movies that were successfully linked
	for _, tmdbID := range tmdbIDs {
		if _, failed := result.Failed[tmdbID]; failed {
			continue // Skip failed ones
		}
		if existing, ok := existingMovies[tmdbID]; ok && len(existing.Genres) > 0 {
			result.Added = append(result.Added, tmdbID)
		} else if _, ok := newMovieDBIDs[tmdbID]; ok {
			result.Added = append(result.Added, tmdbID)
		}
	}

	logger.Info("AddFavorites: complete",
		logger.Int("user_id", int(userID)),
		logger.Int("added", len(result.Added)),
	)

	return result, nil
}

// addSingleFavorite adds a single movie to favorites (kept for backward compatibility, uses individual operations)
func (s *Service) addSingleFavorite(ctx context.Context, userID int64, tmdbID int) error {
	// Check if movie exists in local database with valid genres
	movieInDB, err := s.repo.GetFavoriteByTMDBID(ctx, userID, tmdbID)
	if err != nil {
		return err
	}

	if movieInDB != nil && len(movieInDB.Genres) > 0 {
		return s.repo.AddToUserMovies(ctx, userID, movieInDB.ID)
	}

	// Check Redis cache
	cachedMovie, err := s.cache.GetMovie(ctx, tmdbID)
	if err == nil && cachedMovie != nil && len(cachedMovie.Genres) > 0 {
		movieDBID, err := s.repo.UpsertMovie(ctx, *cachedMovie)
		if err != nil {
			return err
		}
		return s.repo.AddToUserMovies(ctx, userID, movieDBID)
	}

	// Fetch from TMDB API (always has complete data including genres)
	details, err := s.tmdbClient.GetMovieDetails(tmdbID)
	if err != nil {
		return err
	}

	// Convert genres to strings
	genres := make([]string, 0, len(details.Genres))
	for _, g := range details.Genres {
		genres = append(genres, g.Name)
	}

	year := 0
	if len(details.ReleaseDate) >= 4 {
		year, _ = strconv.Atoi(details.ReleaseDate[:4])
	}

	// Save to movies table
	movieInput := movie_collection.MovieInput{
		TMDBID:      details.ID,
		Title:       details.Title,
		PosterURL:   tmdb.PosterURL(details.PosterPath),
		BackdropURL: tmdb.BackdropURL(details.BackdropPath),
		ReleaseYear: year,
		TMDBRating:  int(details.VoteAverage * 10),
		Genres:      genres,
	}

	movieDBID, err := s.repo.UpsertMovie(ctx, movieInput)
	if err != nil {
		return err
	}

	if err := s.repo.AddToUserMovies(ctx, userID, movieDBID); err != nil {
		return err
	}

	// Cache for future use
	_ = s.cache.SetMovie(ctx, tmdbID, movieInput)

	return nil
}

// SearchFavorites searches within user's favorites
func (s *Service) SearchFavorites(ctx context.Context, userID int64, query string, cursor string, limit int) (*GetFavoritesResponse, error) {
	movies, totalCount, nextCursor, err := s.repo.SearchFavorites(ctx, userID, query, cursor, limit)
	if err != nil {
		return nil, err
	}

	return &GetFavoritesResponse{
		Favorites:  movies,
		NextCursor: nextCursor,
		Limit:      limit,
		TotalCount: totalCount,
	}, nil
}
