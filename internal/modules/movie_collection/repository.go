// Package movie_collection provides shared functionality for user's movie collections
// (favorites, watchlist, watched).
package movie_collection

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/trynafindbhumik/cinematch-backend/internal/db"
)

// RepositoryInterface defines the interface for movie collection repository operations.
// This allows for easy mocking in tests and better dependency injection.
type RepositoryInterface interface {
	// Movie operations
	GetMovieByTMDBID(ctx context.Context, tmdbID int) (*MovieExists, error)
	UpsertMovie(ctx context.Context, movie MovieInput) (int, error)
	AddToUserMovies(ctx context.Context, userID int64, movieDBID int, tmdbID int, status Status) error
	GetGenresByTMDBIDs(ctx context.Context, tmdbIDs []int) ([]string, error)

	// User movie operations
	GetUserMovieByID(ctx context.Context, userID int64, id int64, status Status) (*UserMovie, error)
	DeleteUserMovie(ctx context.Context, userID int64, id int64, status Status) error
}

// Repository handles database operations for movie collections.
// It provides shared functionality used by favorites, watchlist, and watched modules.
type Repository struct{}

// NewRepository creates a new movie collection repository
func NewRepository() *Repository {
	return &Repository{}
}

// GetMovieByTMDBID retrieves a movie from the movies table by TMDB ID
func (r *Repository) GetMovieByTMDBID(ctx context.Context, tmdbID int) (*MovieExists, error) {
	var movie MovieExists
	var genres []string

	err := db.Pool().QueryRow(ctx, `
		SELECT id, tmdb_id, title, poster_url, COALESCE(backdrop_url, ''), release_year, tmdb_rating, genres
		FROM movies
		WHERE tmdb_id = $1
	`, tmdbID).Scan(
		&movie.ID,
		&movie.TMDBID,
		&movie.Title,
		&movie.PosterURL,
		&movie.BackdropURL,
		&movie.ReleaseYear,
		&movie.TMDBRating,
		&genres,
	)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get movie: %w", err)
	}

	movie.Genres = genres
	return &movie, nil
}

// UpsertMovie inserts or updates a movie in the movies table.
// Returns the movie's database ID.
func (r *Repository) UpsertMovie(ctx context.Context, movie MovieInput) (int, error) {
	var movieDBID int
	err := db.Pool().QueryRow(ctx, `
		INSERT INTO movies (tmdb_id, title, poster_url, backdrop_url, release_year, tmdb_rating, genres)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (tmdb_id) DO UPDATE SET
			title = EXCLUDED.title,
			poster_url = EXCLUDED.poster_url,
			backdrop_url = EXCLUDED.backdrop_url,
			release_year = EXCLUDED.release_year,
			tmdb_rating = EXCLUDED.tmdb_rating,
			genres = EXCLUDED.genres,
			updated_at = NOW()
		RETURNING id
	`, movie.TMDBID, movie.Title, movie.PosterURL, movie.BackdropURL, movie.ReleaseYear, movie.TMDBRating, movie.Genres).Scan(&movieDBID)

	if err != nil {
		return 0, fmt.Errorf("failed to upsert movie: %w", err)
	}

	return movieDBID, nil
}

// AddToUserMovies adds a movie to user's collection with the given status.
// Uses upsert to handle cases where the movie already exists in user's collection.
func (r *Repository) AddToUserMovies(ctx context.Context, userID int64, movieDBID int, tmdbID int, status Status) error {
	_, err := db.Pool().Exec(ctx, `
		INSERT INTO user_movies (user_id, movie_id, status, is_favorite)
		VALUES ($1, $2, $3, false)
		ON CONFLICT (user_id, movie_id) 
		DO UPDATE SET 
			status = $3,
			is_favorite = CASE WHEN $3 = 'watchlist' THEN false ELSE is_favorite END,
			updated_at = NOW()
	`, userID, movieDBID, status)

	if err != nil {
		return fmt.Errorf("failed to add to user_movies: %w", err)
	}

	return nil
}

// AddToFavorites adds a movie to user's favorites.
// If movie already exists in user's collection, updates is_favorite to true.
func (r *Repository) AddToFavorites(ctx context.Context, userID int64, movieDBID int) error {
	_, err := db.Pool().Exec(ctx, `
		INSERT INTO user_movies (user_id, movie_id, is_favorite)
		VALUES ($1, $2, true)
		ON CONFLICT (user_id, movie_id) 
		DO UPDATE SET 
			is_favorite = true,
			updated_at = NOW()
	`, userID, movieDBID)

	if err != nil {
		return fmt.Errorf("failed to add to favorites: %w", err)
	}

	return nil
}

// GetGenresByTMDBIDs retrieves genre names by their TMDB IDs.
// Returns empty slice if no IDs provided.
func (r *Repository) GetGenresByTMDBIDs(ctx context.Context, tmdbIDs []int) ([]string, error) {
	if len(tmdbIDs) == 0 {
		return []string{}, nil
	}

	placeholders := make([]string, len(tmdbIDs))
	args := make([]interface{}, len(tmdbIDs))
	for i, id := range tmdbIDs {
		placeholders[i] = fmt.Sprintf("$%d", i+1)
		args[i] = id
	}

	query := fmt.Sprintf(`
		SELECT name FROM genres WHERE tmdb_id IN (%s)
	`, strings.Join(placeholders, ","))

	rows, err := db.Pool().Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to get genres: %w", err)
	}
	defer rows.Close()

	var genres []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		genres = append(genres, name)
	}

	return genres, rows.Err()
}

// GetUserMovies retrieves user's movies with cursor pagination and optional genre filter.
// The status parameter determines which collection to query (watchlist/watched).
// For favorites, use StatusNone and check is_favorite=true separately.
func (r *Repository) GetUserMovies(ctx context.Context, userID int64, filter CollectionFilter, status Status) ([]UserMovie, int, string, error) {
	var movies []UserMovie
	var totalCount int

	// Build base query based on status
	var baseQuery, countQuery string

	if status == StatusNone {
		// Favorites query - check is_favorite flag
		baseQuery = `
			SELECT um.id, m.id, m.tmdb_id, m.title, m.poster_url, m.release_year,
				m.tmdb_rating, m.genres, um.added_at
			FROM user_movies um
			JOIN movies m ON um.movie_id = m.id
			WHERE um.user_id = $1 AND um.is_favorite = true
		`
		countQuery = `
			SELECT COUNT(*) FROM user_movies um
			JOIN movies m ON um.movie_id = m.id
			WHERE um.user_id = $1 AND um.is_favorite = true
		`
	} else {
		// Watchlist or Watched query
		baseQuery = fmt.Sprintf(`
			SELECT um.id, m.id, m.tmdb_id, m.title, m.poster_url, m.release_year,
				m.tmdb_rating, m.genres, um.added_at
			FROM user_movies um
			JOIN movies m ON um.movie_id = m.id
			WHERE um.user_id = $1 AND um.status = '%s'
		`, status)
		countQuery = fmt.Sprintf(`
			SELECT COUNT(*) FROM user_movies um
			JOIN movies m ON um.movie_id = m.id
			WHERE um.user_id = $1 AND um.status = '%s'
		`, status)
	}

	// Build args slice
	args := []interface{}{userID}
	argIndex := 2

	// Handle cursor (id for cursor-based pagination)
	if filter.Cursor != "" {
		cursorID, err := strconv.ParseInt(filter.Cursor, 10, 64)
		if err != nil {
			return nil, 0, "", fmt.Errorf("invalid cursor: %w", err)
		}
		baseQuery += fmt.Sprintf(" AND um.id < $%d", argIndex)
		countQuery += fmt.Sprintf(" AND um.id < $%d", argIndex)
		args = append(args, cursorID)
		argIndex++
	}

	// Handle genre filter
	if filter.Genre != "" {
		baseQuery += fmt.Sprintf(" AND $%d = ANY(m.genres)", argIndex)
		countQuery += fmt.Sprintf(" AND $%d = ANY(m.genres)", argIndex)
		args = append(args, filter.Genre)
		argIndex++
	}

	// Count query uses same args without limit
	countArgs := make([]interface{}, len(args))
	copy(countArgs, args)
	err := db.Pool().QueryRow(ctx, countQuery, countArgs...).Scan(&totalCount)
	if err != nil {
		return nil, 0, "", fmt.Errorf("failed to count movies: %w", err)
	}

	// Order by added_at descending and limit
	baseQuery += " ORDER BY um.added_at DESC"
	baseQuery += fmt.Sprintf(" LIMIT $%d", argIndex)
	args = append(args, filter.Limit)

	rows, err := db.Pool().Query(ctx, baseQuery, args...)
	if err != nil {
		return nil, 0, "", fmt.Errorf("failed to query movies: %w", err)
	}
	defer rows.Close()

	var nextCursor string
	for rows.Next() {
		var movie UserMovie
		var genres []string
		var addedAt sql.NullTime

		err := rows.Scan(
			&movie.ID,
			&movie.MovieDBID,
			&movie.TMDBID,
			&movie.Title,
			&movie.PosterURL,
			&movie.ReleaseYear,
			&movie.TMDBRating,
			&genres,
			&addedAt,
		)
		if err != nil {
			return nil, 0, "", fmt.Errorf("failed to scan movie: %w", err)
		}

		movie.Genres = genres
		if addedAt.Valid {
			movie.AddedAt = addedAt.Time
		}

		movies = append(movies, movie)
	}

	// Calculate next_cursor from last movie's id - only if we got a full page
	if len(movies) == filter.Limit {
		lastMovie := movies[len(movies)-1]
		nextCursor = fmt.Sprintf("%d", lastMovie.ID)
	}

	return movies, totalCount, nextCursor, nil
}

// SearchUserMovies searches within user's movies by title.
// Supports both status-based queries and favorites query.
// Uses cursor-based pagination when filter.Cursor is provided.
func (r *Repository) SearchUserMovies(ctx context.Context, userID int64, filter CollectionFilter, status Status) ([]UserMovie, int, string, error) {
	searchPattern := "%" + strings.ToLower(filter.Query) + "%"

	var baseQuery, countQuery string

	if status == StatusNone {
		countQuery = `
			SELECT COUNT(*) FROM user_movies um
			JOIN movies m ON um.movie_id = m.id
			WHERE um.user_id = $1 AND um.is_favorite = true
			AND (LOWER(m.title) LIKE $2)
		`
		baseQuery = `
			SELECT um.id, m.id, m.tmdb_id, m.title, m.poster_url, m.release_year,
				m.tmdb_rating, m.genres, um.added_at
			FROM user_movies um
			JOIN movies m ON um.movie_id = m.id
			WHERE um.user_id = $1 AND um.is_favorite = true
			AND (LOWER(m.title) LIKE $2)
		`
	} else {
		countQuery = fmt.Sprintf(`
			SELECT COUNT(*) FROM user_movies um
			JOIN movies m ON um.movie_id = m.id
			WHERE um.user_id = $1 AND um.status = '%s'
			AND (LOWER(m.title) LIKE $2)
		`, status)
		baseQuery = fmt.Sprintf(`
			SELECT um.id, m.id, m.tmdb_id, m.title, m.poster_url, m.release_year,
				m.tmdb_rating, m.genres, um.added_at
			FROM user_movies um
			JOIN movies m ON um.movie_id = m.id
			WHERE um.user_id = $1 AND um.status = '%s'
			AND (LOWER(m.title) LIKE $2)
		`, status)
	}

	var totalCount int
	err := db.Pool().QueryRow(ctx, countQuery, userID, searchPattern).Scan(&totalCount)
	if err != nil {
		return nil, 0, "", fmt.Errorf("failed to count search results: %w", err)
	}

	// Build args slice for query
	args := []interface{}{userID, searchPattern}
	argIndex := 3

	// Handle cursor-based pagination
	var nextCursor string
	if filter.Cursor != "" {
		cursorID, err := strconv.ParseInt(filter.Cursor, 10, 64)
		if err != nil {
			return nil, 0, "", fmt.Errorf("invalid cursor: %w", err)
		}
		baseQuery += fmt.Sprintf(" AND um.id < $%d", argIndex)
		args = append(args, cursorID)
		argIndex++
	}

	// Order by id descending and limit
	baseQuery += " ORDER BY um.id DESC"
	baseQuery += fmt.Sprintf(" LIMIT $%d", argIndex)
	args = append(args, filter.Limit)

	rows, err := db.Pool().Query(ctx, baseQuery, args...)
	if err != nil {
		return nil, 0, "", fmt.Errorf("failed to search movies: %w", err)
	}
	defer rows.Close()

	var movies []UserMovie
	for rows.Next() {
		var movie UserMovie
		var genres []string
		var addedAt sql.NullTime

		err := rows.Scan(
			&movie.ID,
			&movie.MovieDBID,
			&movie.TMDBID,
			&movie.Title,
			&movie.PosterURL,
			&movie.ReleaseYear,
			&movie.TMDBRating,
			&genres,
			&addedAt,
		)
		if err != nil {
			return nil, 0, "", fmt.Errorf("failed to scan movie: %w", err)
		}

		movie.Genres = genres
		if addedAt.Valid {
			movie.AddedAt = addedAt.Time
		}

		movies = append(movies, movie)
	}

	// Calculate next_cursor from last movie's id - only if we got a full page
	if len(movies) == filter.Limit {
		lastMovie := movies[len(movies)-1]
		nextCursor = fmt.Sprintf("%d", lastMovie.ID)
	}

	return movies, totalCount, nextCursor, nil
}

// GetUserMovieByID retrieves a specific user's movie by user_movies.id
func (r *Repository) GetUserMovieByID(ctx context.Context, userID int64, id int64, status Status) (*UserMovie, error) {
	var movie UserMovie
	var genres []string
	var addedAt sql.NullTime

	var query string
	if status == StatusNone {
		query = `
			SELECT um.id, m.id, m.tmdb_id, m.title, m.poster_url, m.release_year,
			       m.tmdb_rating, m.genres, um.added_at
			FROM user_movies um
			JOIN movies m ON um.movie_id = m.id
			WHERE um.id = $1 AND um.user_id = $2 AND um.is_favorite = true
		`
	} else {
		query = fmt.Sprintf(`
			SELECT um.id, m.id, m.tmdb_id, m.title, m.poster_url, m.release_year,
			       m.tmdb_rating, m.genres, um.added_at
			FROM user_movies um
			JOIN movies m ON um.movie_id = m.id
			WHERE um.id = $1 AND um.user_id = $2 AND um.status = '%s'
		`, status)
	}

	err := db.Pool().QueryRow(ctx, query, id, userID).Scan(
		&movie.ID, &movie.MovieDBID, &movie.TMDBID,
		&movie.Title,
		&movie.PosterURL,
		&movie.ReleaseYear,
		&movie.TMDBRating,
		&genres,
		&addedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get movie: %w", err)
	}

	movie.Genres = genres
	if addedAt.Valid {
		movie.AddedAt = addedAt.Time
	}

	return &movie, nil
}

// GetFavoriteTMDBIDs retrieves all TMDB IDs for user's favorites
func (r *Repository) GetFavoriteTMDBIDs(ctx context.Context, userID int64) ([]int, error) {
	rows, err := db.Pool().Query(ctx, `
		SELECT m.tmdb_id FROM user_movies um
		JOIN movies m ON um.movie_id = m.id
		WHERE um.user_id = $1 AND um.is_favorite = true
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to query favorite IDs: %w", err)
	}
	defer rows.Close()

	var tmdbIDs []int
	for rows.Next() {
		var tmdbID int
		if err := rows.Scan(&tmdbID); err != nil {
			return nil, fmt.Errorf("failed to scan tmdb ID: %w", err)
		}
		tmdbIDs = append(tmdbIDs, tmdbID)
	}

	return tmdbIDs, nil
}

// GetStatusByID retrieves the status and is_favorite for a user_movies entry
func (r *Repository) GetStatusByID(ctx context.Context, userID int64, id int64) (string, bool, error) {
	var status string
	var isFavorite bool
	err := db.Pool().QueryRow(ctx, `
		SELECT status, is_favorite FROM user_movies 
		WHERE id = $1 AND user_id = $2
	`, id, userID).Scan(&status, &isFavorite)
	if err == pgx.ErrNoRows {
		return "", false, fmt.Errorf("not found")
	}
	if err != nil {
		return "", false, fmt.Errorf("failed to get status: %w", err)
	}
	return status, isFavorite, nil
}

// DeleteUserMovie removes a movie from user's collection.
// If the movie is also a favorite (for watchlist/watched), changes status to 'none'.
// Otherwise deletes the row.
func (r *Repository) DeleteUserMovie(ctx context.Context, userID int64, id int64, status Status) error {
	_, isFavorite, err := r.GetStatusByID(ctx, userID, id)
	if err != nil {
		return fmt.Errorf("item not found")
	}

	if isFavorite {
		// Movie is also a favorite, just change status to 'none'
		_, err = db.Pool().Exec(ctx, `
			UPDATE user_movies SET status = 'none', updated_at = NOW()
			WHERE id = $1 AND user_id = $2
		`, id, userID)
	} else {
		// Not a favorite, delete the row
		_, err = db.Pool().Exec(ctx, `
			DELETE FROM user_movies WHERE id = $1 AND user_id = $2
		`, id, userID)
	}

	if err != nil {
		return fmt.Errorf("failed to delete: %w", err)
	}
	return nil
}

// DeleteFavoriteByID removes a favorite by user_movies.id.
// If status is 'none', deletes the row; otherwise just sets is_favorite=false.
func (r *Repository) DeleteFavoriteByID(ctx context.Context, userID int64, id int64) error {
	statusStr, _, err := r.GetStatusByID(ctx, userID, id)
	if err != nil {
		return fmt.Errorf("favorite not found")
	}

	if statusStr == "none" {
		// Only existed for favorite, delete it
		result, err := db.Pool().Exec(ctx, `
			DELETE FROM user_movies WHERE id = $1 AND user_id = $2
		`, id, userID)
		if err != nil {
			return fmt.Errorf("failed to delete favorite: %w", err)
		}
		if result.RowsAffected() == 0 {
			return fmt.Errorf("favorite not found")
		}
	} else {
		// Movie is in watchlist or watched, just update is_favorite
		result, err := db.Pool().Exec(ctx, `
			UPDATE user_movies SET is_favorite = false, updated_at = NOW()
			WHERE id = $1 AND user_id = $2
		`, id, userID)
		if err != nil {
			return fmt.Errorf("failed to update favorite: %w", err)
		}
		if result.RowsAffected() == 0 {
			return fmt.Errorf("favorite not found")
		}
	}

	return nil
}

// GetTMDBIDsByStatus retrieves all TMDB IDs for user's collection by status
func (r *Repository) GetTMDBIDsByStatus(ctx context.Context, userID int64, status Status) ([]int, error) {
	if status == StatusNone {
		return r.GetFavoriteTMDBIDs(ctx, userID)
	}

	rows, err := db.Pool().Query(ctx, `
		SELECT m.tmdb_id FROM user_movies um
		JOIN movies m ON um.movie_id = m.id
		WHERE um.user_id = $1 AND um.status = $2
	`, userID, status)
	if err != nil {
		return nil, fmt.Errorf("failed to query TMDB IDs: %w", err)
	}
	defer rows.Close()

	var tmdbIDs []int
	for rows.Next() {
		var tmdbID int
		if err := rows.Scan(&tmdbID); err != nil {
			return nil, fmt.Errorf("failed to scan tmdb ID: %w", err)
		}
		tmdbIDs = append(tmdbIDs, tmdbID)
	}

	return tmdbIDs, nil
}

// GetMoviesByTMDBIDs retrieves multiple movies by their TMDB IDs in a single query.
// Returns a map of tmdbID -> MovieExists for movies found in the database.
func (r *Repository) GetMoviesByTMDBIDs(ctx context.Context, tmdbIDs []int) (map[int]*MovieExists, error) {
	if len(tmdbIDs) == 0 {
		return make(map[int]*MovieExists), nil
	}

	placeholders := make([]string, len(tmdbIDs))
	args := make([]interface{}, len(tmdbIDs))
	for i, id := range tmdbIDs {
		placeholders[i] = fmt.Sprintf("$%d", i+1)
		args[i] = id
	}

	query := fmt.Sprintf(`
		SELECT id, tmdb_id, title, poster_url, COALESCE(backdrop_url, ''), release_year, tmdb_rating, genres
		FROM movies
		WHERE tmdb_id IN (%s)
	`, strings.Join(placeholders, ","))

	rows, err := db.Pool().Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to get movies: %w", err)
	}
	defer rows.Close()

	result := make(map[int]*MovieExists)
	for rows.Next() {
		var movie MovieExists
		var genres []string

		err := rows.Scan(
			&movie.ID,
			&movie.TMDBID,
			&movie.Title,
			&movie.PosterURL,
			&movie.BackdropURL,
			&movie.ReleaseYear,
			&movie.TMDBRating,
			&genres,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan movie: %w", err)
		}
		movie.Genres = genres
		result[movie.TMDBID] = &movie
	}

	return result, rows.Err()
}

// UpsertMoviesBatch inserts or updates multiple movies in a single query.
// Returns a map of tmdbID -> movieDBID for all movies (both inserted and updated).
func (r *Repository) UpsertMoviesBatch(ctx context.Context, movies []MovieInput) (map[int]int, error) {
	if len(movies) == 0 {
		return make(map[int]int), nil
	}

	// Build batch insert query
	valueStrings := make([]string, 0, len(movies))
	valueArgs := make([]interface{}, 0, len(movies)*7)
	argIndex := 1

	for _, m := range movies {
		values := make([]string, 7)
		values[0] = fmt.Sprintf("($%d", argIndex)
		valueArgs = append(valueArgs, m.TMDBID)
		argIndex++

		values[1] = fmt.Sprintf("$%d", argIndex)
		valueArgs = append(valueArgs, m.Title)
		argIndex++

		values[2] = fmt.Sprintf("$%d", argIndex)
		valueArgs = append(valueArgs, m.PosterURL)
		argIndex++

		values[3] = fmt.Sprintf("$%d", argIndex)
		valueArgs = append(valueArgs, m.BackdropURL)
		argIndex++

		values[4] = fmt.Sprintf("$%d", argIndex)
		valueArgs = append(valueArgs, m.ReleaseYear)
		argIndex++

		values[5] = fmt.Sprintf("$%d", argIndex)
		valueArgs = append(valueArgs, m.TMDBRating)
		argIndex++

		values[6] = fmt.Sprintf("$%d)", argIndex)
		valueArgs = append(valueArgs, m.Genres)
		argIndex++

		valueStrings = append(valueStrings, strings.Join(values, ", "))
	}

	query := fmt.Sprintf(`
		INSERT INTO movies (tmdb_id, title, poster_url, backdrop_url, release_year, tmdb_rating, genres)
		VALUES %s
		ON CONFLICT (tmdb_id) DO UPDATE SET
			title = EXCLUDED.title,
			poster_url = EXCLUDED.poster_url,
			backdrop_url = EXCLUDED.backdrop_url,
			release_year = EXCLUDED.release_year,
			tmdb_rating = EXCLUDED.tmdb_rating,
			genres = EXCLUDED.genres,
			updated_at = NOW()
		RETURNING id, tmdb_id
	`, strings.Join(valueStrings, ", "))

	rows, err := db.Pool().Query(ctx, query, valueArgs...)
	if err != nil {
		return nil, fmt.Errorf("failed to upsert movies batch: %w", err)
	}
	defer rows.Close()

	result := make(map[int]int)
	for rows.Next() {
		var movieDBID, tmdbID int
		if err := rows.Scan(&movieDBID, &tmdbID); err != nil {
			return nil, fmt.Errorf("failed to scan upsert result: %w", err)
		}
		result[tmdbID] = movieDBID
	}

	return result, rows.Err()
}

// UserMovieEntry represents an entry to add to user_movies
type UserMovieEntry struct {
	MovieDBID int
	TMDBID    int
}

// AddToUserMoviesBatch adds multiple movies to user's collection in a single query.
func (r *Repository) AddToUserMoviesBatch(ctx context.Context, userID int64, entries []UserMovieEntry, status Status) error {
	if len(entries) == 0 {
		return nil
	}

	valueStrings := make([]string, 0, len(entries))
	valueArgs := make([]interface{}, 0, len(entries)*3)
	argIndex := 1

	for _, e := range entries {
		valueStrings = append(valueStrings, fmt.Sprintf("($%d, $%d, $%d)", argIndex, argIndex+1, argIndex+2))
		valueArgs = append(valueArgs, userID, e.MovieDBID, status)
		argIndex += 3
	}

	query := fmt.Sprintf(`
		INSERT INTO user_movies (user_id, movie_id, status)
		VALUES %s
		ON CONFLICT (user_id, movie_id)
		DO UPDATE SET
			status = EXCLUDED.status,
			updated_at = NOW()
	`, strings.Join(valueStrings, ", "))

	_, err := db.Pool().Exec(ctx, query, valueArgs...)
	if err != nil {
		return fmt.Errorf("failed to add to user_movies batch: %w", err)
	}

	return nil
}

// AddToFavoritesBatch adds multiple movies to user's favorites in a single query.
func (r *Repository) AddToFavoritesBatch(ctx context.Context, userID int64, movieDBIDs []int64) error {
	if len(movieDBIDs) == 0 {
		return nil
	}

	valueStrings := make([]string, 0, len(movieDBIDs))
	valueArgs := make([]interface{}, 0, len(movieDBIDs)*3)
	argIndex := 1

	for _, movieDBID := range movieDBIDs {
		valueStrings = append(valueStrings, fmt.Sprintf("($%d, $%d, $%d)", argIndex, argIndex+1, argIndex+2))
		valueArgs = append(valueArgs, userID, movieDBID, true)
		argIndex += 3
	}

	query := fmt.Sprintf(`
		INSERT INTO user_movies (user_id, movie_id, is_favorite)
		VALUES %s
		ON CONFLICT (user_id, movie_id)
		DO UPDATE SET
			is_favorite = true,
			updated_at = NOW()
	`, strings.Join(valueStrings, ", "))

	_, err := db.Pool().Exec(ctx, query, valueArgs...)
	if err != nil {
		return fmt.Errorf("failed to add to favorites batch: %w", err)
	}

	return nil
}
