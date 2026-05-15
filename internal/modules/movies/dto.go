package movies

// MovieResponse is the response struct for a movie
type MovieResponse struct {
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
	// Vote counts from our DB (always present, 0 if movie not in DB)
	TotalCount   int    `json:"total_count"`
	LikeCount    int    `json:"like_count"`
	LoveCount    int    `json:"love_count"`
	DislikeCount int    `json:"dislike_count"`
	HateCount    int    `json:"hate_count"`
	SkipCount    int    `json:"skip_count"`
	// User's reaction to this movie (empty if not reacted)
	UserReaction string `json:"user_reaction,omitempty"`
}

// SearchResponse is the response for search endpoint
type SearchResponse struct {
	Movies       []MovieResponse `json:"movies"`
	Page         int             `json:"page"`
	TotalPages   int             `json:"total_pages"`
	TotalResults int             `json:"total_results"`
}

// TrendingResponse is the response for trending endpoint
type TrendingResponse struct {
	Movies       []MovieResponse `json:"movies"`
	Page         int             `json:"page"`
	TotalPages   int             `json:"total_pages"`
	TotalResults int             `json:"total_results"`
}

// MovieDetailsResponse is the response for get by ID endpoint
type MovieDetailsResponse struct {
	MovieResponse
	Budget         int64              `json:"budget,omitempty"`
	Revenue        int64              `json:"revenue,omitempty"`
	Adult          bool               `json:"adult,omitempty"`
	Credits        *CreditsResponse   `json:"credits,omitempty"`
	WatchProviders *WatchProvidersDTO `json:"watch_providers,omitempty"`
}

// DeepCopy creates a deep copy of MovieDetailsResponse
func (m *MovieDetailsResponse) DeepCopy() *MovieDetailsResponse {
	if m == nil {
		return nil
	}
	cp := *m
	cp.TMDBID = m.TMDBID
	cp.Title = m.Title
	cp.PosterURL = m.PosterURL
	cp.BackdropURL = m.BackdropURL
	cp.ReleaseYear = m.ReleaseYear
	cp.TMDBRating = m.TMDBRating
	if m.Genres != nil {
		cp.Genres = make([]string, len(m.Genres))
		copy(cp.Genres, m.Genres)
	}
	cp.Runtime = m.Runtime
	cp.Tagline = m.Tagline
	cp.Status = m.Status
	cp.TotalCount = m.TotalCount
	cp.LikeCount = m.LikeCount
	cp.LoveCount = m.LoveCount
	cp.DislikeCount = m.DislikeCount
	cp.HateCount = m.HateCount
	cp.SkipCount = m.SkipCount
	cp.UserReaction = m.UserReaction
	if m.Credits != nil {
		cp.Credits = &CreditsResponse{
			Cast: make([]CastMember, len(m.Credits.Cast)),
		}
		for i, c := range m.Credits.Cast {
			cp.Credits.Cast[i] = c
		}
	}
	if m.WatchProviders != nil {
		cp.WatchProviders = &WatchProvidersDTO{
			Results: make(map[string][]ProviderInfo),
		}
		for region, providers := range m.WatchProviders.Results {
			cp.WatchProviders.Results[region] = make([]ProviderInfo, len(providers))
			copy(cp.WatchProviders.Results[region], providers)
		}
	}
	return &cp
}

// CreditsResponse contains cast only
type CreditsResponse struct {
	Cast []CastMember `json:"cast"`
}

// CastMember represents a cast member
type CastMember struct {
	ID         int    `json:"id"`
	Name       string `json:"name"`
	Character  string `json:"character"`
	ProfileURL string `json:"profile_url"`
	Order      int    `json:"order"`
}

// WatchProvidersDTO maps region -> providers
type WatchProvidersDTO struct {
	Results map[string][]ProviderInfo `json:"results"`
}

// ProviderInfo represents a streaming provider
type ProviderInfo struct {
	ProviderID       int    `json:"provider_id"`
	ProviderName     string `json:"provider_name"`
	LogoURL          string `json:"logo_url"`
	DisplayPriority  int    `json:"display_priority"`
}

// VideosResponse is the response for videos endpoint
type VideosResponse struct {
	ID       int            `json:"id"`
	Videos   []VideoDetail  `json:"videos"`
}

// VideoDetail represents a movie video
type VideoDetail struct {
	ID       string `json:"id"`
	Key      string `json:"key"`
	Name     string `json:"name"`
Site     string `json:"site"`
Type     string `json:"type"`
Official bool   `json:"official"`
}

// ErrorResponse represents an API error
type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`
}

// PaginationParams holds pagination parameters
type PaginationParams struct {
	Page  int
	Limit int
}