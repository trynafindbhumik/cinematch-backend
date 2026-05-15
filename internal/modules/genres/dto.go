package genres

// GenreResponse is returned when getting a single genre
type GenreResponse struct {
	ID   int16  `json:"id"`
	Name string `json:"name"`
}

// GenresListResponse is returned when getting all genres
type GenresListResponse struct {
	Genres []GenreResponse `json:"genres"`
}

// UserGenreResponse is returned when getting user's selected genres
type UserGenreResponse struct {
	ID      int16  `json:"id"`
	Name    string `json:"name"`
	GenreID int16  `json:"genreId"` // Foreign key to genres table
}

// UserGenresListResponse is returned when getting user's selected genres
type UserGenresListResponse struct {
	Genres []UserGenreResponse `json:"genres"`
}

// ErrorResponse is the standard error response
type ErrorResponse struct {
	Error string `json:"error"`
}
