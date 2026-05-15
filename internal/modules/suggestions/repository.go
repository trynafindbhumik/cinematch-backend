package suggestions

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/lib/pq"
	"github.com/trynafindbhumik/cinematch-backend/internal/db"
)

const (
	maxFavorites  = 60
	maxWatchlist  = 60
	maxReactions  = 70
)

type Repository struct{}

func NewRepository() *Repository {
	return &Repository{}
}

func (r *Repository) GetFavoriteMovies(ctx context.Context, userID int64) ([]FavoriteMovie, error) {
	rows, err := db.Pool().Query(ctx, `
		SELECT um.id, m.tmdb_id, m.title, m.poster_url, m.release_year, um.added_at
		FROM user_movies um
		JOIN movies m ON um.movie_id = m.id
		WHERE um.user_id = $1 AND um.is_favorite = true
		ORDER BY um.added_at DESC
		LIMIT $2
	`, userID, maxFavorites)
	if err != nil {
		return nil, fmt.Errorf("failed to get favorite movies: %w", err)
	}
	defer rows.Close()

	var movies []FavoriteMovie
	for rows.Next() {
		var m FavoriteMovie
		var addedAt string
		if err := rows.Scan(&m.ID, &m.TMDBID, &m.Title, &m.PosterURL, &m.ReleaseYear, &addedAt); err != nil {
			return nil, fmt.Errorf("failed to scan favorite: %w", err)
		}
		m.AddedAt = addedAt
		movies = append(movies, m)
	}
	return movies, rows.Err()
}

func (r *Repository) GetFavoriteCount(ctx context.Context, userID int64) (int, error) {
	var count int
	err := db.Pool().QueryRow(ctx, `
		SELECT COUNT(*) FROM user_movies
		WHERE user_id = $1 AND is_favorite = true
	`, userID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count favorites: %w", err)
	}
	return count, nil
}

func (r *Repository) GetWatchlistMovies(ctx context.Context, userID int64) ([]FavoriteMovie, error) {
	rows, err := db.Pool().Query(ctx, `
		SELECT um.id, m.tmdb_id, m.title, m.poster_url, m.release_year, um.added_at
		FROM user_movies um
		JOIN movies m ON um.movie_id = m.id
		WHERE um.user_id = $1 AND um.status = 'watchlist'
		ORDER BY um.added_at DESC
		LIMIT $2
	`, userID, maxWatchlist)
	if err != nil {
		return nil, fmt.Errorf("failed to get watchlist movies: %w", err)
	}
	defer rows.Close()

	var movies []FavoriteMovie
	for rows.Next() {
		var m FavoriteMovie
		var addedAt string
		if err := rows.Scan(&m.ID, &m.TMDBID, &m.Title, &m.PosterURL, &m.ReleaseYear, &addedAt); err != nil {
			return nil, fmt.Errorf("failed to scan watchlist: %w", err)
		}
		m.AddedAt = addedAt
		movies = append(movies, m)
	}
	return movies, rows.Err()
}

func (r *Repository) GetReactions(ctx context.Context, userID int64) ([]Reaction, error) {
	rows, err := db.Pool().Query(ctx, `
		SELECT ur.movie_id, m.tmdb_id, m.title, ur.reaction, ur.created_at
		FROM user_reaction ur
		JOIN movies m ON ur.movie_id = m.id
		WHERE ur.user_id = $1
		ORDER BY ur.created_at DESC
		LIMIT $2
	`, userID, maxReactions)
	if err != nil {
		return nil, fmt.Errorf("failed to get reactions: %w", err)
	}
	defer rows.Close()

	var reactions []Reaction
	for rows.Next() {
		var rx Reaction
		var createdAt string
		if err := rows.Scan(&rx.MovieID, &rx.TMDBID, &rx.Title, &rx.Reaction, &createdAt); err != nil {
			return nil, fmt.Errorf("failed to scan reaction: %w", err)
		}
		rx.CreatedAt = createdAt
		reactions = append(reactions, rx)
	}
	return reactions, rows.Err()
}

func (r *Repository) GetMovieTMDBIDsByStatus(ctx context.Context, userID int64, status string) ([]int, error) {
	rows, err := db.Pool().Query(ctx, `
		SELECT m.tmdb_id
		FROM user_movies um
		JOIN movies m ON um.movie_id = m.id
		WHERE um.user_id = $1 AND um.status = $2
	`, userID, status)
	if err != nil {
		return nil, fmt.Errorf("failed to get TMDB IDs by status: %w", err)
	}
	defer rows.Close()

	var ids []int
	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func (r *Repository) GetFavoriteTMDBIDs(ctx context.Context, userID int64) ([]int, error) {
	rows, err := db.Pool().Query(ctx, `
		SELECT m.tmdb_id
		FROM user_movies um
		JOIN movies m ON um.movie_id = m.id
		WHERE um.user_id = $1 AND um.is_favorite = true
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get favorite TMDB IDs: %w", err)
	}
	defer rows.Close()

	var ids []int
	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func (r *Repository) GetReactedTMDBIDs(ctx context.Context, userID int64) ([]int, error) {
	rows, err := db.Pool().Query(ctx, `
		SELECT m.tmdb_id
		FROM user_reaction ur
		JOIN movies m ON ur.movie_id = m.id
		WHERE ur.user_id = $1
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get reacted TMDB IDs: %w", err)
	}
	defer rows.Close()

	var ids []int
	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func (r *Repository) UpsertMovie(ctx context.Context, tmdbID int, title string, posterURL string, releaseYear int) (int, error) {
	var movieDBID int
	err := db.Pool().QueryRow(ctx, `
		INSERT INTO movies (tmdb_id, title, poster_url, release_year)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (tmdb_id) DO UPDATE SET
			title = EXCLUDED.title,
			poster_url = EXCLUDED.poster_url,
			release_year = EXCLUDED.release_year,
			updated_at = NOW()
		RETURNING id
	`, tmdbID, title, posterURL, releaseYear).Scan(&movieDBID)
	if err != nil {
		return 0, fmt.Errorf("failed to upsert movie: %w", err)
	}
	return movieDBID, nil
}

func (r *Repository) UpdateGenerationLog(ctx context.Context, userID int64, movieIDs []int) error {
	_, err := db.Pool().Exec(ctx, `
		INSERT INTO user_daily_generation_log (user_id, date, movie_ids)
		VALUES ($1, (NOW() AT TIME ZONE 'Asia/Kolkata')::date, $2)
		ON CONFLICT (user_id, date) DO UPDATE SET
			movie_ids = $2,
			created_at = NOW()
	`, userID, movieIDs)
	if err != nil {
		return fmt.Errorf("failed to update generation log: %w", err)
	}
	return nil
}

func (r *Repository) GetMovieByTMDBID(ctx context.Context, tmdbID int) (*FavoriteMovie, error) {
	var m FavoriteMovie
	err := db.Pool().QueryRow(ctx, `
		SELECT id, tmdb_id, title, poster_url, release_year
		FROM movies WHERE tmdb_id = $1
	`, tmdbID).Scan(&m.ID, &m.TMDBID, &m.Title, &m.PosterURL, &m.ReleaseYear)
	if err != nil {
		return nil, err
	}
	return &m, nil
}

func (r *Repository) HasGeneratedToday(ctx context.Context, userID int64) (bool, error) {
	var exists bool
	err := db.Pool().QueryRow(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM user_daily_generation_log
			WHERE user_id = $1 AND date = (NOW() AT TIME ZONE 'Asia/Kolkata')::date
		)
	`, userID).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("failed to check generation status: %w", err)
	}
	return exists, nil
}

func (r *Repository) GetMovieIDByTMDBID(ctx context.Context, tmdbID int) (int, error) {
	var movieID int
	err := db.Pool().QueryRow(ctx, `SELECT id FROM movies WHERE tmdb_id = $1`, tmdbID).Scan(&movieID)
	if err != nil {
		return 0, fmt.Errorf("movie not found: %w", err)
	}
	return movieID, nil
}

func (r *Repository) AddReaction(ctx context.Context, userID int64, movieID int, reaction string) error {
	_, err := db.Pool().Exec(ctx, `
		INSERT INTO user_reaction (user_id, movie_id, reaction)
		VALUES ($1, $2, $3::suggestion_reaction)
		ON CONFLICT (user_id, movie_id) DO UPDATE SET reaction = $3::suggestion_reaction
	`, userID, movieID, reaction)
	if err != nil {
		return fmt.Errorf("failed to add reaction: %w", err)
	}
	return nil
}

func (r *Repository) RemoveFromDailyGenerationLog(ctx context.Context, userID int64, tmdbID int) error {
	_, err := db.Pool().Exec(ctx, `
		UPDATE user_daily_generation_log
		SET movie_ids = array_remove(movie_ids, $2::int)
		WHERE user_id = $1 AND date = (NOW() AT TIME ZONE 'Asia/Kolkata')::date
	`, userID, tmdbID)
	if err != nil {
		return fmt.Errorf("failed to remove from generation log: %w", err)
	}
	return nil
}

func (r *Repository) BuildExcludedTMDBIDs(ctx context.Context, userID int64) ([]int, error) {
	favIDs, err := r.GetFavoriteTMDBIDs(ctx, userID)
	if err != nil {
		return nil, err
	}
	watchlistIDs, err := r.GetMovieTMDBIDsByStatus(ctx, userID, "watchlist")
	if err != nil {
		return nil, err
	}
	watchedIDs, err := r.GetMovieTMDBIDsByStatus(ctx, userID, "watched")
	if err != nil {
		return nil, err
	}
	reactionIDs, err := r.GetReactedTMDBIDs(ctx, userID)
	if err != nil {
		return nil, err
	}

	seen := make(map[int]bool)
	var excluded []int
	for _, ids := range [][]int{favIDs, watchlistIDs, watchedIDs, reactionIDs} {
		for _, id := range ids {
			if !seen[id] {
				seen[id] = true
				excluded = append(excluded, id)
			}
		}
	}
	return excluded, nil
}

func titlesFromMovies(movies []FavoriteMovie) string {
	var titles []string
	for _, m := range movies {
		titles = append(titles, m.Title)
	}
	return strings.Join(titles, ", ")
}

func reactionsString(reactions []Reaction) string {
	var parts []string
	for _, r := range reactions {
		parts = append(parts, fmt.Sprintf("%s: %s", r.Title, r.Reaction))
	}
	return strings.Join(parts, ", ")
}

// GenerationLog represents a row from user_daily_generation_log
type GenerationLog struct {
	ID        int64
	UserID    int64
	Date      string
	MovieIDs  []int
	CreatedAt string
}

// FindOldLogWithMovieIDs finds the most recent log older than today with non-empty movie_ids
func (r *Repository) FindOldLogWithMovieIDs(ctx context.Context, userID int64) (*GenerationLog, error) {
	row := db.Pool().QueryRow(ctx, `
		SELECT id, user_id, date::text, movie_ids, created_at::text
		FROM user_daily_generation_log
		WHERE user_id = $1
			AND date < (NOW() AT TIME ZONE 'Asia/Kolkata')::date
			AND array_length(movie_ids, 1) > 0
		ORDER BY date DESC
		LIMIT 1
	`, userID)

	var log GenerationLog
	err := row.Scan(&log.ID, &log.UserID, &log.Date, pq.Array(&log.MovieIDs), &log.CreatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to find old log: %w", err)
	}
	return &log, nil
}

// GetLogByDate gets a specific date's log
func (r *Repository) GetLogByDate(ctx context.Context, userID int64, date string) (*GenerationLog, error) {
	row := db.Pool().QueryRow(ctx, `
		SELECT id, user_id, date::text, movie_ids, created_at::text
		FROM user_daily_generation_log
		WHERE user_id = $1 AND date = $2
	`, userID, date)

	var log GenerationLog
	err := row.Scan(&log.ID, &log.UserID, &log.Date, pq.Array(&log.MovieIDs), &log.CreatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get log: %w", err)
	}
	return &log, nil
}

// GetTodayLog gets today's log if it exists
func (r *Repository) GetTodayLog(ctx context.Context, userID int64) (*GenerationLog, error) {
	row := db.Pool().QueryRow(ctx, `
		SELECT id, user_id, date::text, movie_ids, created_at::text
		FROM user_daily_generation_log
		WHERE user_id = $1 AND date = (NOW() AT TIME ZONE 'Asia/Kolkata')::date
	`, userID)

	var log GenerationLog
	err := row.Scan(&log.ID, &log.UserID, &log.Date, pq.Array(&log.MovieIDs), &log.CreatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get today's log: %w", err)
	}
	return &log, nil
}

// CreateTodayLog creates or updates today's log with movie IDs
func (r *Repository) CreateTodayLog(ctx context.Context, userID int64, movieIDs []int) error {
	_, err := db.Pool().Exec(ctx, `
		INSERT INTO user_daily_generation_log (user_id, date, movie_ids)
		VALUES ($1, (NOW() AT TIME ZONE 'Asia/Kolkata')::date, $2)
		ON CONFLICT (user_id, date) DO UPDATE SET
			movie_ids = $2,
			created_at = NOW()
	`, userID, movieIDs)
	if err != nil {
		return fmt.Errorf("failed to create today's log: %w", err)
	}
	return nil
}

// DeleteLogByDate deletes a log by date
func (r *Repository) DeleteLogByDate(ctx context.Context, userID int64, date string) error {
	_, err := db.Pool().Exec(ctx, `
		DELETE FROM user_daily_generation_log
		WHERE user_id = $1 AND date = $2
	`, userID, date)
	if err != nil {
		return fmt.Errorf("failed to delete log: %w", err)
	}
	return nil
}

// GetFirstMovieIDs returns first N movie IDs from a specific log
func (r *Repository) GetFirstMovieIDs(ctx context.Context, userID int64, date string, count int) ([]int, error) {
	row := db.Pool().QueryRow(ctx, `
		SELECT movie_ids
		FROM user_daily_generation_log
		WHERE user_id = $1 AND date = $2
	`, userID, date)

	var movieIDs []int
	err := row.Scan(pq.Array(&movieIDs))
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get movie ids: %w", err)
	}

	if len(movieIDs) > count {
		return movieIDs[:count], nil
	}
	return movieIDs, nil
}

// GetNextTMDBID finds the next tmdb_id after current in a specific log
// Returns nil if current is the last movie
func (r *Repository) GetNextTMDBID(ctx context.Context, userID int64, date string, currentTMDBID int) (*int, error) {
	row := db.Pool().QueryRow(ctx, `
		SELECT movie_ids
		FROM user_daily_generation_log
		WHERE user_id = $1 AND date = $2
	`, userID, date)

	var movieIDs []int
	err := row.Scan(pq.Array(&movieIDs))
	if err != nil {
		return nil, fmt.Errorf("failed to get movie ids: %w", err)
	}

	// Find current index
	for i, id := range movieIDs {
		if id == currentTMDBID {
			// If there's a next one, return it
			if i+1 < len(movieIDs) {
				return &movieIDs[i+1], nil
			}
			return nil, nil // no more movies
		}
	}

	return nil, fmt.Errorf("current tmdb_id not found in log")
}