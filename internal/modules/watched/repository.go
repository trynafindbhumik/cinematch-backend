package watched

import (
	"context"

	"github.com/trynafindbhumik/cinematch-backend/internal/modules/movie_collection"
)

// Repository handles watched database operations
type Repository struct {
	sharedRepo *movie_collection.Repository
}

// NewRepository creates a new watched repository
func NewRepository() *Repository {
	return &Repository{
		sharedRepo: movie_collection.NewRepository(),
	}
}

// GetWatched retrieves user's watched movies with cursor pagination and genre filter
func (r *Repository) GetWatched(ctx context.Context, userID int64, cursor string, limit int, genre string) ([]WatchedMovie, int, string, error) {
	filter := movie_collection.CollectionFilter{
		Cursor: cursor,
		Limit:  limit,
		Genre:  genre,
	}

	movies, totalCount, nextCursor, err := r.sharedRepo.GetUserMovies(ctx, userID, filter, movie_collection.StatusWatched)
	if err != nil {
		return nil, 0, "", err
	}

	return convertToWatchedMovies(movies), totalCount, nextCursor, nil
}

// GetWatchedIDs retrieves all TMDB IDs for user's watched list
func (r *Repository) GetWatchedIDs(ctx context.Context, userID int64) ([]int, error) {
	return r.sharedRepo.GetTMDBIDsByStatus(ctx, userID, movie_collection.StatusWatched)
}

// SearchWatched searches within user's watched list
func (r *Repository) SearchWatched(ctx context.Context, userID int64, query string, cursor string, limit int) ([]WatchedMovie, int, string, error) {
	filter := movie_collection.CollectionFilter{
		Query:  query,
		Cursor: cursor,
		Limit:  limit,
	}

	movies, totalCount, nextCursor, err := r.sharedRepo.SearchUserMovies(ctx, userID, filter, movie_collection.StatusWatched)
	if err != nil {
		return nil, 0, "", err
	}

	return convertToWatchedMovies(movies), totalCount, nextCursor, nil
}

// GetMovieByTMDBID checks if a movie exists in movies table
func (r *Repository) GetMovieByTMDBID(ctx context.Context, tmdbID int) (*movie_collection.MovieExists, error) {
	return r.sharedRepo.GetMovieByTMDBID(ctx, tmdbID)
}

// UpsertMovie inserts or updates a movie in the movies table
func (r *Repository) UpsertMovie(ctx context.Context, movie movie_collection.MovieInput) (int, error) {
	return r.sharedRepo.UpsertMovie(ctx, movie)
}

// AddToUserMovies adds a movie to user's collection with given status
func (r *Repository) AddToUserMovies(ctx context.Context, userID int64, movieDBID int, tmdbID int) error {
	return r.sharedRepo.AddToUserMovies(ctx, userID, movieDBID, tmdbID, movie_collection.StatusWatched)
}

// DeleteWatchedByID removes a movie from watched
func (r *Repository) DeleteWatchedByID(ctx context.Context, userID int64, id int64) error {
	return r.sharedRepo.DeleteUserMovie(ctx, userID, id, movie_collection.StatusWatched)
}

// GetGenresByTMDBIDs retrieves genre names by their TMDB IDs
func (r *Repository) GetGenresByTMDBIDs(ctx context.Context, tmdbIDs []int) ([]string, error) {
	return r.sharedRepo.GetGenresByTMDBIDs(ctx, tmdbIDs)
}

// GetMoviesByTMDBIDs retrieves multiple movies by TMDB IDs
func (r *Repository) GetMoviesByTMDBIDs(ctx context.Context, tmdbIDs []int) (map[int]*movie_collection.MovieExists, error) {
	return r.sharedRepo.GetMoviesByTMDBIDs(ctx, tmdbIDs)
}

// UpsertMoviesBatch batch upserts movies
func (r *Repository) UpsertMoviesBatch(ctx context.Context, movies []movie_collection.MovieInput) (map[int]int, error) {
	return r.sharedRepo.UpsertMoviesBatch(ctx, movies)
}

// AddToUserMoviesBatch batch adds movies to user's collection
func (r *Repository) AddToUserMoviesBatch(ctx context.Context, userID int64, entries []movie_collection.UserMovieEntry) error {
	return r.sharedRepo.AddToUserMoviesBatch(ctx, userID, entries, movie_collection.StatusWatched)
}

// convertToWatchedMovies converts shared UserMovie slice to WatchedMovie slice
func convertToWatchedMovies(movies []movie_collection.UserMovie) []WatchedMovie {
	result := make([]WatchedMovie, 0, len(movies))
	for _, m := range movies {
		result = append(result, WatchedMovie{
			ID:          m.ID,
			MovieDBID:   m.MovieDBID,
			TMDBID:      m.TMDBID,
			Title:       m.Title,
			PosterURL:   m.PosterURL,
			ReleaseYear: m.ReleaseYear,
			TMDBRating:  m.TMDBRating,
			Genres:      m.Genres,
			AddedAt:     m.AddedAt.Format("2006-01-02T15:04:05Z07:00"),
		})
	}
	return result
}
