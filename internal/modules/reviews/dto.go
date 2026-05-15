package reviews

import (
	"encoding/base64"
	"encoding/json"
	"time"
)

// Cursor represents the pagination cursor
type Cursor struct {
	Source   string `json:"source"`   // "db" or "tmdb"
	DBCursor string `json:"db_cursor,omitempty"`
	TMDBPage int    `json:"tmdb_page,omitempty"`
}

// EncodeCursor encodes cursor to base64 string
func EncodeCursor(c *Cursor) string {
	data, _ := json.Marshal(c)
	return base64.StdEncoding.EncodeToString(data)
}

// DecodeCursor decodes base64 string to cursor
func DecodeCursor(encoded string) (*Cursor, error) {
	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, err
	}
	var c Cursor
	if err := json.Unmarshal(data, &c); err != nil {
		return nil, err
	}
	return &c, nil
}

// CreateReviewRequest is the request body for creating a review
type CreateReviewRequest struct {
	TMDBID  int    `json:"tmdb_id" binding:"required"`
	Rating  int    `json:"rating" binding:"required,min=1,max=10"`
	Comment string `json:"comment"`
}

// UpdateReviewRequest is the request body for updating a review
type UpdateReviewRequest struct {
	Rating  int    `json:"rating" binding:"omitempty,min=1,max=10"`
	Comment string `json:"comment"`
}

// ReviewResponse is the response for a single review
type ReviewResponse struct {
	ID        int64     `json:"id"`
	UserID    int64     `json:"user_id"`
	TMDBID    int       `json:"tmdb_id"`
	Title     string    `json:"title"`
	PosterURL string    `json:"poster_url"`
	Rating    int       `json:"rating"`
	Comment   string    `json:"comment"`
	CreatedAt time.Time `json:"created_at"`
}

// TMDBReviewResponse represents a TMDB review
type TMDBReviewResponse struct {
	ID         string   `json:"id"`
	Author     string   `json:"author"`
	Content    string   `json:"content"`
	CreatedAt  string   `json:"created_at"`
	AvatarPath *string  `json:"avatar_path,omitempty"`
	Rating     *float64 `json:"rating,omitempty"`
}

// ReviewsListResponse is the response for listing reviews with cursor pagination
type ReviewsListResponse struct {
	Reviews     []ReviewsListItem `json:"reviews"`
	NextCursor  string            `json:"next_cursor,omitempty"`
	HasMore     bool              `json:"has_more"`
}

// ReviewsListItem is a generic review item (DB or TMDB)
type ReviewsListItem struct {
	ID            string    `json:"id,omitempty"`
	UserID        int64     `json:"user_id,omitempty"`
	TMDBID        int       `json:"tmdb_id,omitempty"`
	Title         *string   `json:"title"`
	PosterURL     *string   `json:"poster_url"`
	Rating        *float64  `json:"rating,omitempty"`
	Comment       string    `json:"comment,omitempty"`
	AuthorName    string    `json:"author_name,omitempty"`
	AuthorPicture *string   `json:"author_picture"`
	Content       string    `json:"content,omitempty"`
	CreatedAt     time.Time `json:"created_at,omitempty"`
	Source        string    `json:"source"` // "db" or "tmdb"
}

// ErrorResponse represents an API error
type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`
}