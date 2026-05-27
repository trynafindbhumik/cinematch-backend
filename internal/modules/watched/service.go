package watched

import (
	"context"
	"strconv"

	"github.com/trynafindbhumik/cinematch-backend/internal/modules/movie_collection"
	"github.com/trynafindbhumik/cinematch-backend/internal/shared/logger"
	"github.com/trynafindbhumik/cinematch-backend/internal/shared/tmdb"
)

// Service handles watched business logic
type Service struct {
	repo       *Repository
	cache      *movie_collection.Cache
	tmdbClient *tmdb.Client
}

// NewService creates a new watched service
func NewService(repo *Repository, tmdbClient *tmdb.Client) *Service {
	return &Service{
		repo:       repo,
		cache:      movie_collection.NewCache(),
		tmdbClient: tmdbClient,
	}
}

// GetWatched retrieves user's watched list with pagination and genre filter
func (s *Service) GetWatched(ctx context.Context, userID int64, cursor string, limit int, genre string) (*GetWatchedResponse, error) {
	movies, totalCount, nextCursor, err := s.repo.GetWatched(ctx, userID, cursor, limit, genre)
	if err != nil {
		return nil, err
	}

	return &GetWatchedResponse{
		Movies:     movies,
		NextCursor: nextCursor,
		Limit:      limit,
		TotalCount: totalCount,
	}, nil
}

// GetWatchedIDs retrieves all TMDB IDs for user's watched list
func (s *Service) GetWatchedIDs(ctx context.Context, userID int64) (*GetWatchedIDsResponse, error) {
	tmdbIDs, err := s.repo.GetWatchedIDs(ctx, userID)
	if err != nil {
		return nil, err
	}

	return &GetWatchedIDsResponse{
		TMDBIDs: tmdbIDs,
	}, nil
}

// SearchWatched searches within user's watched list
func (s *Service) SearchWatched(ctx context.Context, userID int64, query string, cursor string, limit int) (*GetWatchedResponse, error) {
	movies, totalCount, nextCursor, err := s.repo.SearchWatched(ctx, userID, query, cursor, limit)
	if err != nil {
		return nil, err
	}

	return &GetWatchedResponse{
		Movies:     movies,
		NextCursor: nextCursor,
		Limit:      limit,
		TotalCount: totalCount,
	}, nil
}

// AddToWatchedResult contains the result of adding multiple to watched
type AddToWatchedResult struct {
	Added  []int           `json:"added"`
	Failed map[int]string  `json:"failed,omitempty"` // tmdbID -> error message
}

// AddToWatched adds movies to user's watched list using batch operations
//
// Flow:
// 1. Batch check DB for all tmdbIDs (1 query)
// 2. For movies NOT in DB → check cache
// 3. For movies NOT in cache → fetch from TMDB
// 4. Batch upsert all movies needing insert (from cache or TMDB)
// 5. Batch add all to user_movies watched
func (s *Service) AddToWatched(ctx context.Context, userID int64, req *AddToWatchedRequest) (*AddToWatchedResult, error) {
	if len(req.TMDBIDs) == 0 {
		return &AddToWatchedResult{Added: []int{}}, nil
	}

	result := &AddToWatchedResult{
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

	// moviesToUpsert and tmdbIDToDBID now declared outside if/else
	moviesToUpsert := make([]movie_collection.MovieInput, 0, len(notInDB))
	tmdbIDToDBID := make(map[int]int)

	if len(notInDB) == 0 {
		// All movies already in DB, still need to add to user_movies
	} else {
		// Step 2: For movies not in DB, check cache
		for _, tmdbID := range notInDB {
			cachedMovie, err := s.cache.GetMovie(ctx, tmdbID)
			if err == nil && cachedMovie != nil && len(cachedMovie.Genres) > 0 {
				moviesToUpsert = append(moviesToUpsert, *cachedMovie)
			} else {
				// Not in cache, need to fetch from TMDB
				details, err := s.tmdbClient.GetMovieDetails(tmdbID)
				if err != nil {
					result.Failed[tmdbID] = err.Error()
					logger.Error("AddToWatched: failed to fetch from TMDB",
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

			// Build new entries for newly upserted movies
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

	// Step 4: Batch add all to user_movies watched
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

// findMovieByTMDBIDWatched is a helper to find a movie by its TMDB ID
func findMovieByTMDBIDWatched(movies []movie_collection.MovieInput, tmdbID int) (movie_collection.MovieInput, bool) {
	for _, m := range movies {
		if m.TMDBID == tmdbID {
			return m, true
		}
	}
	return movie_collection.MovieInput{}, false
}

// addSingleToWatched adds a single movie to watched
func (s *Service) addSingleToWatched(ctx context.Context, userID int64, tmdbID int) error {
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

// DeleteFromWatched removes a movie from watched
func (s *Service) DeleteFromWatched(ctx context.Context, userID int64, id int64) error {
	return s.repo.DeleteWatchedByID(ctx, userID, id)
}
