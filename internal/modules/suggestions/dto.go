package suggestions

// MovieDetails represents the full movie data returned by /movies/:id
type MovieDetails struct {
	TMDBID      int      `json:"tmdb_id"`
	Title       string   `json:"title"`
	PosterURL   string   `json:"poster_url"`
	BackdropURL string   `json:"backdrop_url,omitempty"`
	ReleaseYear int      `json:"release_year"`
	TMDBRating  int      `json:"tmdb_rating"`
	Genres      []string `json:"genres,omitempty"`
	Runtime     int      `json:"runtime,omitempty"`
	Tagline     string   `json:"tagline,omitempty"`
	Status      string   `json:"status,omitempty"`
	TotalCount  int      `json:"total_count"`
	LikeCount   int      `json:"like_count"`
	LoveCount   int      `json:"love_count"`
	HateCount   int      `json:"hate_count"`
	SkipCount   int      `json:"skip_count"`
	DislikeCount int     `json:"dislike_count"`
	UserReaction string  `json:"user_reaction,omitempty"`
}

type GenerateSuggestionsResponse struct {
	Suggestions     []MovieDetails `json:"suggestions"`
	GenerationDate  string         `json:"generation_date"`
	Regeneration    bool           `json:"regeneration"`
	Finished        bool           `json:"finished"`
	Message         string         `json:"message,omitempty"`
}

type NextResponse struct {
	Suggestion    *MovieDetails `json:"suggestion"`
	NextTMDBID    *int          `json:"next_tmdb_id"`
	HasMore       bool          `json:"has_more"`
	Regeneration  bool          `json:"regeneration"`
	Finished      bool          `json:"finished"`
	Message       string        `json:"message,omitempty"`
}

type AddReactionRequest struct {
	TMDBID   int    `json:"tmdb_id" binding:"required"`
	Reaction string `json:"reaction" binding:"required"`
}

type AddReactionResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}