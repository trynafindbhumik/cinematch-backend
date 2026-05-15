package genres

// Genre represents a genre in the database
type Genre struct {
	ID     int16
	Name   string
	TMDBID *int32
}

// UserGenre represents a user's selected genre
type UserGenre struct {
	UserID    int64
	GenreID   int16
	GenreName string // For convenience when returning to frontend
}
