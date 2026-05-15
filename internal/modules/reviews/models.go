package reviews

import (
	"time"
)

// Review represents a user review in the database
type Review struct {
	ID            int64     `json:"id"`
	UserID        int64     `json:"user_id"`
	MovieID       int       `json:"movie_id"`
	TMDBID        int       `json:"tmdb_id"`
	Title         *string   `json:"title"`
	PosterURL     *string   `json:"poster_url"`
	Rating        int       `json:"rating"`
	Comment       string    `json:"comment"`
	CreatedAt     time.Time `json:"created_at"`
	AuthorName    string    `json:"author_name,omitempty"`
	AuthorPicture *string   `json:"author_picture,omitempty"`
}