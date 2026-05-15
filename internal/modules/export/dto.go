package export

import "time"

// ExportRequest specifies which data to include in the export
type ExportRequest struct {
	ProfileInfo bool `json:"profile_info"`
	Preferences bool `json:"preferences"`
	Watchlist   bool `json:"watchlist"`
	Watched     bool `json:"watched"`
	Favorites   bool `json:"favorites"`
	Reviews     bool `json:"reviews"`
}

// ExportResponse indicates export email has been sent
type ExportResponse struct {
	Message string `json:"message"`
}

// ProfileInfoCSV represents profile data for export
type ProfileInfoCSV struct {
	Email      string    `csv:"email"`
	Name       string    `csv:"name"`
	Tag        string    `csv:"tag"`
	IsVerified bool      `csv:"is_verified"`
	CreatedAt  time.Time `csv:"created_at"`
}

// PreferenceCSV represents preference data for export
type PreferenceCSV struct {
	PreferenceType string `csv:"preference_type"`
	Value          string `csv:"value"`
}

// WatchlistMovieCSV represents watchlist movie for export
type WatchlistMovieCSV struct {
	MovieTitle  string    `csv:"movie_title"`
	TMDBID      int       `csv:"tmdb_id"`
	PosterURL   string    `csv:"poster_url"`
	ReleaseYear int       `csv:"release_year"`
	Genres      string    `csv:"genres"`
	AddedAt     time.Time `csv:"added_at"`
}

// WatchedMovieCSV represents watched movie for export
type WatchedMovieCSV struct {
	MovieTitle  string    `csv:"movie_title"`
	TMDBID      int       `csv:"tmdb_id"`
	PosterURL   string    `csv:"poster_url"`
	ReleaseYear int       `csv:"release_year"`
	Genres      string    `csv:"genres"`
	IsFavorite  bool      `csv:"is_favorite"`
	AddedAt     time.Time `csv:"added_at"`
}

// FavoriteMovieCSV represents favorite movie for export
type FavoriteMovieCSV struct {
	MovieTitle  string    `csv:"movie_title"`
	TMDBID      int       `csv:"tmdb_id"`
	PosterURL   string    `csv:"poster_url"`
	ReleaseYear int       `csv:"release_year"`
	Genres      string    `csv:"genres"`
	AddedAt     time.Time `csv:"added_at"`
}

// ReviewCSV represents review for export
type ReviewCSV struct {
	MovieTitle string    `csv:"movie_title"`
	TMDBID     int       `csv:"tmdb_id"`
	Rating     int       `csv:"rating"`
	Comment    string    `csv:"comment"`
	CreatedAt  time.Time `csv:"created_at"`
}
