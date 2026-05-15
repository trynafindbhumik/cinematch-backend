package suggestions

type Reaction struct {
	MovieID   int    `json:"movie_id"`
	TMDBID    int    `json:"tmdb_id"`
	Title     string `json:"title"`
	Reaction  string `json:"reaction"`
	CreatedAt string `json:"created_at"`
}

type FavoriteMovie struct {
	ID          int64  `json:"id"`
	TMDBID      int    `json:"tmdb_id"`
	Title       string `json:"title"`
	PosterURL   string `json:"poster_url"`
	ReleaseYear int    `json:"release_year"`
	AddedAt     string `json:"added_at"`
}