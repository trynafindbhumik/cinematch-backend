package weekly_suggestions

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/trynafindbhumik/cinematch-backend/internal/db"
)

const (
	maxFavorites = 60
	maxWatchlist = 60
	maxReactions = 70
)

type Repository struct {
}

func NewRepository() *Repository {
	return &Repository{}
}

// GetWeekStart returns the Sunday of the current week in Asia/Kolkata timezone
func GetWeekStart(t time.Time) string {
	ist := t.In(time.FixedZone("IST", 5*60*60+30*60))
	weekday := int(ist.Weekday())
	// Adjust so Sunday (0) is start of week
	daysSinceSunday := weekday
	sunday := ist.AddDate(0, 0, -daysSinceSunday)
	return sunday.Format("2006-01-02")
}

// FavoriteMovie is reused from suggestions module
type FavoriteMovie struct {
	ID          int64  `json:"id"`
	TMDBID      int    `json:"tmdb_id"`
	Title       string `json:"title"`
	PosterURL   string `json:"poster_url"`
	ReleaseYear int    `json:"release_year"`
	AddedAt     string `json:"added_at"`
}

// Reaction is reused from suggestions module
type Reaction struct {
	MovieID   int    `json:"movie_id"`
	TMDBID    int    `json:"tmdb_id"`
	Title     string `json:"title"`
	Reaction  string `json:"reaction"`
	CreatedAt string `json:"created_at"`
}

// HasGeneratedForWeek checks if weekly suggestions exist for user/week
func (r *Repository) HasGeneratedForWeek(ctx context.Context, userID int64, weekStart string) (bool, error) {
	var exists bool
	err := db.Pool().QueryRow(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM user_weekly_suggestions
			WHERE user_id = $1 AND week_start = $2
		)
	`, userID, weekStart).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("failed to check generation status: %w", err)
	}
	return exists, nil
}

// GetWeeklySuggestions retrieves all movies for a user's week
func (r *Repository) GetWeeklySuggestions(ctx context.Context, userID int64, weekStart string) ([]Movie, error) {
	rows, err := db.Pool().Query(ctx, `
		SELECT m.tmdb_id, m.title, m.poster_url, m.genres, m.release_year,
		       m.tmdb_rating, uws.created_at
		FROM user_weekly_suggestion_movies uwm
		JOIN user_weekly_suggestions uws ON uwm.suggestion_id = uws.id
		JOIN movies m ON uwm.movie_id = m.id
		WHERE uws.user_id = $1 AND uws.week_start = $2
		ORDER BY uwm.position
	`, userID, weekStart)
	if err != nil {
		return nil, fmt.Errorf("failed to get weekly suggestions: %w", err)
	}
	defer rows.Close()

	var movies []Movie
	for rows.Next() {
		var m Movie
		var genres interface{}
		var createdAt string
		if err := rows.Scan(&m.TMDBID, &m.Title, &m.PosterURL, &genres, &m.ReleaseYear, &m.TMDBRating, &createdAt); err != nil {
			return nil, fmt.Errorf("failed to scan movie: %w", err)
		}
		if genres != nil {
			m.Genres = parsePostgresArray(genres.(string))
		}
		movies = append(movies, m)
	}
	return movies, rows.Err()
}

// CreateWeeklySuggestion creates a suggestion entry and returns its ID
func (r *Repository) CreateWeeklySuggestion(ctx context.Context, userID int64, weekStart string) (int64, error) {
	var id int64
	err := db.Pool().QueryRow(ctx, `
		INSERT INTO user_weekly_suggestions (user_id, week_start)
		VALUES ($1, $2)
		ON CONFLICT (user_id, week_start) DO UPDATE SET created_at = NOW()
		RETURNING id
	`, userID, weekStart).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("failed to create weekly suggestion: %w", err)
	}
	return id, nil
}

// CreateWeeklySuggestionMovie inserts a movie into a weekly suggestion
func (r *Repository) CreateWeeklySuggestionMovie(ctx context.Context, suggestionID int64, movieID int, position int) error {
	_, err := db.Pool().Exec(ctx, `
		INSERT INTO user_weekly_suggestion_movies (suggestion_id, movie_id, position)
		VALUES ($1, $2, $3)
	`, suggestionID, movieID, position)
	if err != nil {
		return fmt.Errorf("failed to create weekly suggestion movie: %w", err)
	}
	return nil
}

// UpsertMovie saves movie to movies table and returns the movie ID
func (r *Repository) UpsertMovie(ctx context.Context, tmdbID int, title string, posterURL string, backdropURL string, genres []string, releaseYear int, tmdbRating int) (int, error) {
	var movieID int
	err := db.Pool().QueryRow(ctx, `
		INSERT INTO movies (tmdb_id, title, poster_url, backdrop_url, genres, release_year, tmdb_rating)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (tmdb_id) DO UPDATE SET
			title = EXCLUDED.title,
			poster_url = EXCLUDED.poster_url,
			backdrop_url = EXCLUDED.backdrop_url,
			genres = EXCLUDED.genres,
			release_year = EXCLUDED.release_year,
			tmdb_rating = EXCLUDED.tmdb_rating,
			updated_at = NOW()
		RETURNING id
	`, tmdbID, title, posterURL, backdropURL, genres, releaseYear, tmdbRating).Scan(&movieID)
	if err != nil {
		return 0, fmt.Errorf("failed to upsert movie: %w", err)
	}
	return movieID, nil
}

// GetFavoriteMovies returns user's favorite movies
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

// GetFavoriteCount returns count of user's favorites
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

// GetWatchlistMovies returns user's watchlist movies
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

// GetReactions returns user's past reactions
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

// GetMovieTMDBIDsByStatus returns TMDB IDs for movies in a given status
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

// GetFavoriteTMDBIDs returns TMDB IDs for favorite movies
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

// GetReactedTMDBIDs returns TMDB IDs for movies user has reacted to
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

// BuildExcludedTMDBIDs builds list of already seen/reacted movie IDs
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

func parsePostgresArray(s string) []string {
	s = strings.Trim(s, "{}")
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	for i := range parts {
		parts[i] = strings.Trim(parts[i], `"`)
	}
	return parts
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