package favorites

// Favorites repository for database operations on user's favorite movies.

import (
	"context"

	"github.com/trynafindbhumik/cinematch-backend/internal/modules/movie_collection"
)

// FavoriteMovie represents a movie in user's favorites
type FavoriteMovie struct {
	ID          int64    `json:"id"`
	MovieDBID   int      `json:"movie_db_id"`
	TMDBID      int      `json:"tmdb_id"`
	Title       string   `json:"title"`
	PosterURL   string   `json:"poster_url"`
	ReleaseYear int      `json:"release_year"`
	TMDBRating  int      `json:"tmdb_rating"`
	Genres      []string `json:"genres"`
	AddedAt     string   `json:"added_at"`
}

type Repository struct {
	sharedRepo *movie_collection.Repository
}

func NewRepository() *Repository {
	return &Repository{
		sharedRepo: movie_collection.NewRepository(),
	}
}

// GetFavorites retrieves user's favorite movies with cursor pagination and genre filter
func (r *Repository) GetFavorites(ctx context.Context, userID int64, cursor string, limit int, genre string) ([]FavoriteMovie, int, string, error) {
	filter := movie_collection.CollectionFilter{
		Cursor: cursor,
		Limit:  limit,
		Genre:  genre,
	}

	movies, totalCount, nextCursor, err := r.sharedRepo.GetUserMovies(ctx, userID, filter, movie_collection.StatusNone)
	if err != nil {
		return nil, 0, "", err
	}

	result := make([]FavoriteMovie, 0, len(movies))
	for _, m := range movies {
		result = append(result, FavoriteMovie{
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

	return result, totalCount, nextCursor, nil
}

// GetFavoriteIDs retrieves all TMDB IDs for user's favorites
func (r *Repository) GetFavoriteIDs(ctx context.Context, userID int64) ([]int, error) {
	return r.sharedRepo.GetFavoriteTMDBIDs(ctx, userID)
}

// SearchFavorites searches within user's favorites
func (r *Repository) SearchFavorites(ctx context.Context, userID int64, query string, cursor string, limit int) ([]FavoriteMovie, int, string, error) {
	filter := movie_collection.CollectionFilter{
		Query:  query,
		Cursor: cursor,
		Limit:  limit,
	}

	movies, totalCount, nextCursor, err := r.sharedRepo.SearchUserMovies(ctx, userID, filter, movie_collection.StatusNone)
	if err != nil {
		return nil, 0, "", err
	}

	result := make([]FavoriteMovie, 0, len(movies))
	for _, m := range movies {
		result = append(result, FavoriteMovie{
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

	return result, totalCount, nextCursor, nil
}

// GetFavoriteByTMDBID checks if a movie is in favorites
func (r *Repository) GetFavoriteByTMDBID(ctx context.Context, userID int64, tmdbID int) (*movie_collection.MovieExists, error) {
	return r.sharedRepo.GetMovieByTMDBID(ctx, tmdbID)
}

// UpsertMovie inserts or updates a movie
func (r *Repository) UpsertMovie(ctx context.Context, movie movie_collection.MovieInput) (int, error) {
	return r.sharedRepo.UpsertMovie(ctx, movie)
}

// AddToUserMovies adds a movie to user's favorites
func (r *Repository) AddToUserMovies(ctx context.Context, userID int64, movieDBID int) error {
	return r.sharedRepo.AddToFavorites(ctx, userID, movieDBID)
}

// DeleteFavoriteByID removes a favorite by user_movies.id
func (r *Repository) DeleteFavoriteByID(ctx context.Context, userID int64, id int64) error {
	return r.sharedRepo.DeleteFavoriteByID(ctx, userID, id)
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

// AddToFavoritesBatch batch adds movies to favorites
func (r *Repository) AddToFavoritesBatch(ctx context.Context, userID int64, movieDBIDs []int64) error {
	return r.sharedRepo.AddToFavoritesBatch(ctx, userID, movieDBIDs)
}
