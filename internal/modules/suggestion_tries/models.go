package suggestion_tries

import "time"

type UserSuggestion struct {
	ID              int64     `json:"id"`
	UserID          int64     `json:"user_id"`
	WeekStart       string    `json:"week_start"`
	SuggestionIndex int       `json:"suggestion_index"`
	CreatedAt       time.Time `json:"created_at"`
}

type UserSuggestionMovie struct {
	ID           int64     `json:"id"`
	SuggestionID int64     `json:"suggestion_id"`
	MovieID      int       `json:"movie_id"`
	CreatedAt    time.Time `json:"created_at"`
}

type Movie struct {
	TMDBID      int    `json:"tmdb_id"`
	Title       string `json:"title"`
	PosterURL   string `json:"poster_url"`
	BackdropURL string `json:"backdrop_url,omitempty"`
	Genres      []string `json:"genres"`
	ReleaseYear int    `json:"release_year"`
	TMDBRating  int    `json:"tmdb_rating"`
	MatchReason string `json:"match_reason,omitempty"`
}

type FavoriteMovie struct {
	ID          int64  `json:"id"`
	TMDBID      int    `json:"tmdb_id"`
	Title       string `json:"title"`
	PosterURL   string `json:"poster_url"`
	ReleaseYear int    `json:"release_year"`
	AddedAt     string `json:"added_at"`
}

type Reaction struct {
	MovieID   int    `json:"movie_id"`
	TMDBID    int    `json:"tmdb_id"`
	Title     string `json:"title"`
	Reaction  string `json:"reaction"`
	CreatedAt string `json:"created_at"`
}