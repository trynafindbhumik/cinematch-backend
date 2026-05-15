// Package movie_collection provides shared functionality for user's movie collections
// (favorites, watchlist, watched). It follows clean architecture principles with
// proper dependency injection and interface-based design.
package movie_collection

import (
	"time"
)

// Status represents the status of a movie in user's collection
type Status string

const (
	StatusWatchlist Status = "watchlist"
	StatusWatched   Status = "watched"
	StatusNone      Status = "none"
)

// MovieInput represents movie data for adding to collection
type MovieInput struct {
	TMDBID      int
	Title       string
	PosterURL   string
	BackdropURL string
	ReleaseYear int
	TMDBRating  int
	Genres      []string
}

// MovieExists represents a movie stored in the movies table
type MovieExists struct {
	ID          int
	TMDBID      int
	Title       string
	PosterURL   string
	BackdropURL string
	ReleaseYear int
	TMDBRating  int
	Genres      []string
}

// UserMovie represents a movie in user's collection
type UserMovie struct {
	ID          int64
	MovieDBID   int
	TMDBID      int
	Title       string
	PosterURL   string
	ReleaseYear int
	TMDBRating  int
	Genres      []string
	AddedAt     time.Time
}

// CollectionFilter contains filter options for querying collections
type CollectionFilter struct {
	Cursor string
	Limit  int
	Genre  string
	Query  string
	Page   int
	Offset int
}

// PaginatedResult contains paginated query results
type PaginatedResult struct {
	Movies     []UserMovie
	TotalCount int
	Page       int
	Limit      int
}
