package watchlist

import (
	"context"
	"strconv"

	"github.com/trynafindbhumik/cinematch-backend/internal/modules/movie_collection"
	"github.com/trynafindbhumik/cinematch-backend/internal/shared/logger"
	"github.com/trynafindbhumik/cinematch-backend/internal/shared/tmdb"
)

// Service handles watchlist business logic
type Service struct {
	repo       *Repository
	cache      *movie_collection.Cache
	tmdbClient *tmdb.Client
}

// NewService creates a new watchlist service
func NewService(repo *Repository, tmdbClient *tmdb.Client) *Service {
	return &Service{
		repo:       repo,
		cache:      movie_collection.NewCache(),
		tmdbClient: tmdbClient,
	}
}

// GetWatchlist retrieves user's watchlist with pagination and genre filter
func (s *Service) GetWatchlist(ctx context.Context, userID int64, cursor string, limit int, genre string) (*GetWatchlistResponse, error) {
	movies, totalCount, nextCursor, err := s.repo.GetWatchlist(ctx, userID, cursor, limit, genre)
	if err != nil {
		return nil, err
	}

	return &GetWatchlistResponse{
		Movies:     movies,
		NextCursor: nextCursor,
		Limit:      limit,
		TotalCount: totalCount,
	}, nil
}

// GetWatchlistIDs retrieves all TMDB IDs for user's watchlist
func (s *Service) GetWatchlistIDs(ctx context.Context, userID int64) (*GetWatchlistIDsResponse, error) {
	tmdbIDs, err := s.repo.GetWatchlistIDs(ctx, userID)
	if err != nil {
		return nil, err
	}

	return &GetWatchlistIDsResponse{
		TMDBIDs: tmdbIDs,
	}, nil
}

// SearchWatchlist searches within user's watchlist
func (s *Service) SearchWatchlist(ctx context.Context, userID int64, query string, cursor string, limit int) (*GetWatchlistResponse, error) {
	movies, totalCount, nextCursor, err := s.repo.SearchWatchlist(ctx, userID, query, cursor, limit)
	if err != nil {
		return nil, err
	}

	return &GetWatchlistResponse{
		Movies:     movies,
		NextCursor: nextCursor,
		Limit:      limit,
		TotalCount: totalCount,
	}, nil
}

// AddToWatchlistResult contains the result of adding multiple to watchlist
type AddToWatchlistResult struct {
	Added  []int           `json:"added"`
	Failed map[int]string  `json:"failed,omitempty"` // tmdbID -> error message
}

// AddToWatchlist adds movies to user's watchlist using batch operations
//
// Flow:
// 1. Batch check DB for all tmdbIDs (1 query)
// 2. For movies NOT in DB → check cache
// 3. For movies NOT in cache → fetch from TMDB
// 4. Batch upsert all movies needing insert (from cache or TMDB)
// 5. Batch add all to user_movies watchlist
func (s *Service) AddToWatchlist(ctx context.Context, userID int64, req *AddToWatchlistRequest) (*AddToWatchlistResult, error) {
	if len(req.TMDBIDs) == 0 {
		return &AddToWatchlistResult{Added: []int{}}, nil
	}

	result := &AddToWatchlistResult{
		Added:  make([]int, 0, len(req.TMDBIDs)),
		Failed: make(map[int]string),
	}

	// Step 1: Batch check which movies already exist in DB
	existingMovies, err := s.repo.GetMoviesByTMDBIDs(ctx, req.TMDBIDs)
	if err != nil {
		return nil, err
	}

	// Categorize tmdbIDs
	notInDB := make([]int, 0, len(req.TMDBIDs))
	alreadyInDBEntries := make([]movie_collection.UserMovieEntry, 0)

	for _, tmdbID := range req.TMDBIDs {
		if existing, ok := existingMovies[tmdbID]; ok && len(existing.Genres) > 0 {
			alreadyInDBEntries = append(alreadyInDBEntries, movie_collection.UserMovieEntry{
				MovieDBID: existing.ID,
				TMDBID:    tmdbID,
			})
			result.Added = append(result.Added, tmdbID)
		} else {
			notInDB = append(notInDB, tmdbID)
		}
	}

	// Declare these outside the if/else so they're accessible later
	var moviesToUpsert []movie_collection.MovieInput
	var tmdbIDToDBID map[int]int

	if len(notInDB) == 0 {
		// All movies already in DB, continue to add to user_movies
	} else {

	// Step 2: For movies not in DB, check cache
	moviesToUpsert = make([]movie_collection.MovieInput, 0, len(notInDB))
	for _, tmdbID := range notInDB {
		cachedMovie, err := s.cache.GetMovie(ctx, tmdbID)
		if err == nil && cachedMovie != nil && len(cachedMovie.Genres) > 0 {
			moviesToUpsert = append(moviesToUpsert, *cachedMovie)
		} else {
			// Not in cache, need to fetch from TMDB
			details, err := s.tmdbClient.GetMovieDetails(tmdbID)
			if err != nil {
				result.Failed[tmdbID] = err.Error()
				logger.Error("AddToWatchlist: failed to fetch from TMDB",
					logger.Int("tmdb_id", tmdbID),
					logger.Err(err),
				)
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

	if len(moviesToUpsert) == 0 {
		// All cache/TMDB fetches failed, but we still have alreadyInDBEntries
	} else {
		// Step 3: Batch upsert movies (from cache or TMDB)
		tmdbIDToDBID, err = s.repo.UpsertMoviesBatch(ctx, moviesToUpsert)
		if err != nil {
			return nil, err
		}

		// Add new entries to result.Added
		for _, m := range moviesToUpsert {
			if _, ok := tmdbIDToDBID[m.TMDBID]; ok {
				result.Added = append(result.Added, m.TMDBID)
			}
		}
	}
	}

	// Build entries for all movies to add to user_movies
	allEntries := make([]movie_collection.UserMovieEntry, 0, len(req.TMDBIDs))

	// Add already in DB entries
	for _, entry := range alreadyInDBEntries {
		allEntries = append(allEntries, entry)
	}

	// Add new entries for upserted movies
	for _, m := range moviesToUpsert {
		if dbID, ok := tmdbIDToDBID[m.TMDBID]; ok {
			allEntries = append(allEntries, movie_collection.UserMovieEntry{
				MovieDBID: dbID,
				TMDBID:    m.TMDBID,
			})
		}
	}

	// Step 4: Batch add all to user_movies watchlist
	if len(allEntries) > 0 {
		if err := s.repo.AddToUserMoviesBatch(ctx, userID, allEntries); err != nil {
			return nil, err
		}
	}

	// Remove duplicates from result.Added
	seen := make(map[int]bool)
	uniqueAdded := make([]int, 0, len(result.Added))
	for _, id := range result.Added {
		if !seen[id] {
			seen[id] = true
			uniqueAdded = append(uniqueAdded, id)
		}
	}
	result.Added = uniqueAdded

	return result, nil
}

// findMovieByTMDBID is a helper to find a movie by its TMDB ID
func findMovieByTMDBID(movies []movie_collection.MovieInput, tmdbID int) (movie_collection.MovieInput, bool) {
	for _, m := range movies {
		if m.TMDBID == tmdbID {
			return m, true
		}
	}
	return movie_collection.MovieInput{}, false
}

// addSingleToWatchlist adds a single movie to watchlist
func (s *Service) addSingleToWatchlist(ctx context.Context, userID int64, tmdbID int) error {
	// Step 1: Check if movie exists in movies table with valid genres
	existingMovie, err := s.repo.GetMovieByTMDBID(ctx, tmdbID)
	if err != nil {
		return err
	}

	if existingMovie != nil && len(existingMovie.Genres) > 0 {
		return s.repo.AddToUserMovies(ctx, userID, existingMovie.ID, tmdbID)
	}

	// Step 2: Check Redis cache
	cachedMovie, err := s.cache.GetMovie(ctx, tmdbID)
	if err == nil && cachedMovie != nil && len(cachedMovie.Genres) > 0 {
		movieDBID, err := s.repo.UpsertMovie(ctx, *cachedMovie)
		if err != nil {
			return err
		}
		return s.repo.AddToUserMovies(ctx, userID, movieDBID, tmdbID)
	}

	// Step 3: Fetch from TMDB (always has complete data including genres)
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

	if err := s.repo.AddToUserMovies(ctx, userID, movieDBID, tmdbID); err != nil {
		return err
	}

	// Cache for future use
	_ = s.cache.SetMovie(ctx, tmdbID, movieInput)

	return nil
}

// DeleteFromWatchlist removes a movie from watchlist
func (s *Service) DeleteFromWatchlist(ctx context.Context, userID int64, id int64) error {
	return s.repo.DeleteWatchlistByID(ctx, userID, id)
}
