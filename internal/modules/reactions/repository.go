package reactions

import (
	"context"
	"fmt"

	"github.com/trynafindbhumik/cinematch-backend/internal/db"
)

type Repository struct{}

func NewRepository() *Repository {
	return &Repository{}
}

// GetMovieIDByTMDBID returns internal movie ID from TMDB ID
func (r *Repository) GetMovieIDByTMDBID(ctx context.Context, tmdbID int) (int, error) {
	var movieID int
	err := db.Pool().QueryRow(ctx, `SELECT id FROM movies WHERE tmdb_id = $1`, tmdbID).Scan(&movieID)
	if err != nil {
		return 0, fmt.Errorf("movie not found: %w", err)
	}
	return movieID, nil
}

// GetPreviousReaction returns the user's previous reaction for a movie (if any)
func (r *Repository) GetPreviousReaction(ctx context.Context, userID int64, movieID int) (string, error) {
	var reaction string
	err := db.Pool().QueryRow(ctx, `
		SELECT reaction FROM user_reaction
		WHERE user_id = $1 AND movie_id = $2
	`, userID, movieID).Scan(&reaction)
	if err != nil {
		// No previous reaction - not an error, just return empty
		return "", nil
	}
	return reaction, nil
}

// AddReaction adds or updates a user's reaction for a movie
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

func isValidReaction(reaction string) bool {
	switch reaction {
	case "like", "love", "dislike", "hate", "skip":
		return true
	default:
		return false
	}
}

// UpsertMovie saves a movie into the movies table if it doesn't exist yet
func (r *Repository) UpsertMovie(ctx context.Context, tmdbID int, title, posterURL, backdropURL string, releaseYear, tmdbRating int, genres []string) (int, error) {
	var movieID int
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
	`, tmdbID, title, posterURL, backdropURL, releaseYear, tmdbRating, genres).Scan(&movieID)
	if err != nil {
		return 0, fmt.Errorf("failed to upsert movie: %w", err)
	}
	return movieID, nil
}

// UpdateMovieReactionCounts updates the reaction counts for a movie
func (r *Repository) UpdateMovieReactionCounts(ctx context.Context, movieID int, previousReaction string, newReaction string) error {
	// If previous reaction exists, decrement that count
	if previousReaction != "" && previousReaction != newReaction {
		if !isValidReaction(previousReaction) {
			return fmt.Errorf("invalid previous reaction: %s", previousReaction)
		}
		_, err := db.Pool().Exec(ctx, fmt.Sprintf(`
			UPDATE movies
			SET %s_count = %s_count - 1, total_count = total_count - 1
			WHERE id = $1 AND %s_count > 0
		`, previousReaction, previousReaction, previousReaction), movieID)
		if err != nil {
			return fmt.Errorf("failed to decrement previous reaction count: %w", err)
		}
	}

	// If updating reaction or adding new one
	if newReaction != "" {
		// If same reaction was already set, this is an update - we don't increment twice
		if previousReaction == newReaction {
			return nil
		}
		if !isValidReaction(newReaction) {
			return fmt.Errorf("invalid reaction: %s", newReaction)
		}
		_, err := db.Pool().Exec(ctx, fmt.Sprintf(`
			UPDATE movies
			SET %s_count = %s_count + 1, total_count = total_count + 1
			WHERE id = $1
		`, newReaction, newReaction), movieID)
		if err != nil {
			return fmt.Errorf("failed to increment reaction count: %w", err)
		}
	}

	return nil
}

// RemoveReaction removes a user's reaction for a movie
func (r *Repository) RemoveReaction(ctx context.Context, userID int64, movieID int) error {
	_, err := db.Pool().Exec(ctx, `
		DELETE FROM user_reaction WHERE user_id = $1 AND movie_id = $2
	`, userID, movieID)
	if err != nil {
		return fmt.Errorf("failed to remove reaction: %w", err)
	}
	return nil
}

// RemoveFromDailyGenerationLog removes a tmdbID from user's daily generation log movie_ids array
func (r *Repository) RemoveFromDailyGenerationLog(ctx context.Context, userID int64, tmdbID int) error {
	_, err := db.Pool().Exec(ctx, `
		UPDATE user_daily_generation_log
		SET movie_ids = array_remove(movie_ids, $2::int)
		WHERE user_id = $1
	`, userID, tmdbID)
	if err != nil {
		return fmt.Errorf("failed to remove from generation log: %w", err)
	}
	return nil
}
