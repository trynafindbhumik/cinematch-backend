package export

import (
	"context"
	"fmt"
	"strings"

	"github.com/trynafindbhumik/cinematch-backend/internal/db"
)

// Repository handles data fetching for exports
type Repository struct{}

func NewRepository() *Repository {
	return &Repository{}
}

// GetUserProfileInfo fetches user profile info
func (r *Repository) GetUserProfileInfo(ctx context.Context, userID int64) (*ProfileInfoCSV, error) {
	var p ProfileInfoCSV
	err := db.Pool().QueryRow(ctx, `
		SELECT email, name, tag, is_verified, created_at
		FROM users WHERE id = $1
	`, userID).Scan(&p.Email, &p.Name, &p.Tag, &p.IsVerified, &p.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("failed to get profile: %w", err)
	}
	return &p, nil
}

// GetUserPreferences fetches user genre and streaming preferences
func (r *Repository) GetUserPreferences(ctx context.Context, userID int64) ([]PreferenceCSV, error) {
	var prefs []PreferenceCSV

	// Get genre preferences
	rows, err := db.Pool().Query(ctx, `
		SELECT g.name FROM genres g
		JOIN user_genres ug ON g.id = ug.genre_id
		WHERE ug.user_id = $1
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get genres: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		prefs = append(prefs, PreferenceCSV{PreferenceType: "genre", Value: name})
	}

	// Get streaming service preferences
	rows2, err := db.Pool().Query(ctx, `
		SELECT s.name FROM streaming_services s
		JOIN user_streaming_services uss ON s.id = uss.service_id
		WHERE uss.user_id = $1
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get streaming services: %w", err)
	}
	defer rows2.Close()

	for rows2.Next() {
		var name string
		if err := rows2.Scan(&name); err != nil {
			return nil, err
		}
		prefs = append(prefs, PreferenceCSV{PreferenceType: "streaming_service", Value: name})
	}

	return prefs, nil
}

// GetWatchlist fetches user's watchlist movies
func (r *Repository) GetWatchlist(ctx context.Context, userID int64) ([]WatchlistMovieCSV, error) {
	rows, err := db.Pool().Query(ctx, `
		SELECT m.title, m.tmdb_id, m.poster_url, m.release_year, 
			   COALESCE(m.genres, '{}'), um.added_at
		FROM user_movies um
		JOIN movies m ON um.movie_id = m.id
		WHERE um.user_id = $1 AND um.status = 'watchlist'
		ORDER BY um.added_at DESC
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get watchlist: %w", err)
	}
	defer rows.Close()

	var movies []WatchlistMovieCSV
	for rows.Next() {
		var m WatchlistMovieCSV
		var genres []string
		if err := rows.Scan(&m.MovieTitle, &m.TMDBID, &m.PosterURL, &m.ReleaseYear, &genres, &m.AddedAt); err != nil {
			return nil, err
		}
		m.Genres = strings.Join(genres, ",")
		movies = append(movies, m)
	}

	return movies, rows.Err()
}

// GetWatched fetches user's watched movies
func (r *Repository) GetWatched(ctx context.Context, userID int64) ([]WatchedMovieCSV, error) {
	rows, err := db.Pool().Query(ctx, `
		SELECT m.title, m.tmdb_id, m.poster_url, m.release_year,
			   COALESCE(m.genres, '{}'), um.is_favorite, um.added_at
		FROM user_movies um
		JOIN movies m ON um.movie_id = m.id
		WHERE um.user_id = $1 AND um.status = 'watched'
		ORDER BY um.added_at DESC
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get watched: %w", err)
	}
	defer rows.Close()

	var movies []WatchedMovieCSV
	for rows.Next() {
		var m WatchedMovieCSV
		var genres []string
		if err := rows.Scan(&m.MovieTitle, &m.TMDBID, &m.PosterURL, &m.ReleaseYear, &genres, &m.IsFavorite, &m.AddedAt); err != nil {
			return nil, err
		}
		m.Genres = strings.Join(genres, ",")
		movies = append(movies, m)
	}

	return movies, rows.Err()
}

// GetFavorites fetches user's favorite movies (is_favorite = true)
func (r *Repository) GetFavorites(ctx context.Context, userID int64) ([]FavoriteMovieCSV, error) {
	rows, err := db.Pool().Query(ctx, `
		SELECT m.title, m.tmdb_id, m.poster_url, m.release_year,
			   COALESCE(m.genres, '{}'), um.added_at
		FROM user_movies um
		JOIN movies m ON um.movie_id = m.id
		WHERE um.user_id = $1 AND um.is_favorite = true
		ORDER BY um.added_at DESC
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get favorites: %w", err)
	}
	defer rows.Close()

	var movies []FavoriteMovieCSV
	for rows.Next() {
		var m FavoriteMovieCSV
		var genres []string
		if err := rows.Scan(&m.MovieTitle, &m.TMDBID, &m.PosterURL, &m.ReleaseYear, &genres, &m.AddedAt); err != nil {
			return nil, err
		}
		m.Genres = strings.Join(genres, ",")
		movies = append(movies, m)
	}

	return movies, rows.Err()
}

// GetReviews fetches user's reviews
func (r *Repository) GetReviews(ctx context.Context, userID int64) ([]ReviewCSV, error) {
	rows, err := db.Pool().Query(ctx, `
		SELECT m.title, ur.movie_id, ur.rating, ur.comment, ur.created_at
		FROM user_reviews ur
		JOIN movies m ON ur.movie_id = m.tmdb_id
		WHERE ur.user_id = $1
		ORDER BY ur.created_at DESC
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get reviews: %w", err)
	}
	defer rows.Close()

	var reviews []ReviewCSV
	for rows.Next() {
		var r ReviewCSV
		var comment *string
		if err := rows.Scan(&r.MovieTitle, &r.TMDBID, &r.Rating, &comment, &r.CreatedAt); err != nil {
			return nil, err
		}
		if comment != nil {
			r.Comment = *comment
		}
		reviews = append(reviews, r)
	}

	return reviews, rows.Err()
}
