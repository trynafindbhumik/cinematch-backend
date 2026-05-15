package genres

import (
	"context"
	"errors"

	"github.com/trynafindbhumik/cinematch-backend/internal/db"
)

var (
	ErrGenreNotFound      = errors.New("genre not found")
	ErrGenreAlreadyExists = errors.New("genre already exists")
	ErrInvalidGenreID     = errors.New("invalid genre ID")
)

type Repository struct{}

func NewRepository() *Repository {
	return &Repository{}
}

// GetAllGenres retrieves all genres from the master table ( tmdb_id excluded)
func (r *Repository) GetAllGenres(ctx context.Context) ([]Genre, error) {
	rows, err := db.Pool().Query(ctx, `
		SELECT id, name
		FROM genres
		ORDER BY name ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var genres []Genre
	for rows.Next() {
		var g Genre
		if err := rows.Scan(&g.ID, &g.Name); err != nil {
			return nil, err
		}
		genres = append(genres, g)
	}

	return genres, rows.Err()
}

// GetUserGenres retrieves all genres selected by a user
func (r *Repository) GetUserGenres(ctx context.Context, userID int64) ([]UserGenre, error) {
	rows, err := db.Pool().Query(ctx, `
		SELECT ug.user_id, ug.genre_id, g.name
		FROM user_genres ug
		JOIN genres g ON g.id = ug.genre_id
		WHERE ug.user_id = $1
		ORDER BY g.name ASC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var userGenres []UserGenre
	for rows.Next() {
		var ug UserGenre
		if err := rows.Scan(&ug.UserID, &ug.GenreID, &ug.GenreName); err != nil {
			return nil, err
		}
		userGenres = append(userGenres, ug)
	}

	return userGenres, rows.Err()
}

// ValidateGenreIDs checks if all provided genre IDs exist
func (r *Repository) ValidateGenreIDs(ctx context.Context, genreIDs []int16) (bool, error) {
	if len(genreIDs) == 0 {
		return true, nil
	}

	row := db.Pool().QueryRow(ctx, `
		SELECT COUNT(*) FROM genres WHERE id = ANY($1)
	`, genreIDs)

	var count int
	if err := row.Scan(&count); err != nil {
		return false, err
	}

	return count == len(genreIDs), nil
}

// AddUserGenre adds a single genre to a user's profile
func (r *Repository) AddUserGenre(ctx context.Context, userID int64, genreID int16) error {
	_, err := db.Pool().Exec(ctx, `
		INSERT INTO user_genres (user_id, genre_id) VALUES ($1, $2)
		ON CONFLICT (user_id, genre_id) DO NOTHING
	`, userID, genreID)
	return err
}

// RemoveUserGenre removes a single genre from a user's profile
func (r *Repository) RemoveUserGenre(ctx context.Context, userID int64, genreID int16) error {
	_, err := db.Pool().Exec(ctx, `
		DELETE FROM user_genres WHERE user_id = $1 AND genre_id = $2
	`, userID, genreID)
	return err
}

// GetGenreMap retrieves all genres as a map of tmdb_id -> name
func (r *Repository) GetGenreMap(ctx context.Context) (map[int]string, error) {
	rows, err := db.Pool().Query(ctx, `
		SELECT tmdb_id, name FROM genres WHERE tmdb_id IS NOT NULL
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	genreMap := make(map[int]string)
	for rows.Next() {
		var tmdbID int
		var name string
		if err := rows.Scan(&tmdbID, &name); err != nil {
			return nil, err
		}
		genreMap[tmdbID] = name
	}
	return genreMap, rows.Err()
}
