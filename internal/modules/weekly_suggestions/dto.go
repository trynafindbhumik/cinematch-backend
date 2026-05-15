package weekly_suggestions

type WeeklySuggestionResponse struct {
	WeekStart        string   `json:"week_start"`
	Suggestions      []Movie  `json:"suggestions"`
	GeneratedAt      string   `json:"generated_at"`
	AlreadyGenerated bool     `json:"already_generated"`
}

type Movie struct {
	TMDBID       int      `json:"tmdb_id"`
	Title        string   `json:"title"`
	PosterURL    string   `json:"poster_url"`
	Genres       []string `json:"genres"`
	ReleaseYear  int      `json:"release_year"`
	TMDBRating   int      `json:"tmdb_rating"`
	MatchReason  string   `json:"match_reason,omitempty"`
}

type ErrorResponse struct {
	Error string `json:"error"`
}