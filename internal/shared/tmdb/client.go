package tmdb

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/joho/godotenv"
)

const (
	baseURL       = "https://api.themoviedb.org/3"
	ImageBaseURL  = "https://image.tmdb.org/t/p"
	maxRetries    = 3
	retryBaseDelay = 500 * time.Millisecond
)

var apiKey string

func Load() error {
	godotenv.Load()
	apiKey = os.Getenv("TMDB_API_KEY")
	if apiKey == "" {
		return fmt.Errorf("TMDB_API_KEY environment variable not set")
	}
	return nil
}

// Client is the TMDB API client
type Client struct {
	httpClient *http.Client
}

// NewClient creates a new TMDB client
func NewClient() *Client {
	return &Client{
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// doRequest performs an HTTP GET with retry logic
func (c *Client) doRequest(url string) (*http.Response, error) {
	var lastErr error
	for attempt := 1; attempt <= maxRetries; attempt++ {
		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}

		resp, err := c.httpClient.Do(req)
		if err != nil {
			lastErr = err
			if attempt < maxRetries {
				time.Sleep(retryBaseDelay * time.Duration(attempt))
				continue
			}
			return nil, fmt.Errorf("failed to execute request after %d attempts: %w", maxRetries, lastErr)
		}

		if resp.StatusCode == http.StatusOK {
			return resp, nil
		}

		// Close response body before retry
		resp.Body.Close()

		// Don't retry on client errors (400, 401, 404, etc.)
		if resp.StatusCode >= 400 && resp.StatusCode < 500 {
			return nil, fmt.Errorf("TMDB returned status %d", resp.StatusCode)
		}

		// Retry on server errors (500, 502, 503, etc.)
		lastErr = fmt.Errorf("TMDB returned status %d", resp.StatusCode)
		if attempt < maxRetries {
			time.Sleep(retryBaseDelay * time.Duration(attempt))
			continue
		}
	}
	return nil, lastErr
}

// readAndClose reads body and closes the response
func readAndClose(body io.ReadCloser) ([]byte, error) {
	defer body.Close()
	return io.ReadAll(body)
}

// Movie represents a movie from TMDB API
type Movie struct {
	ID           int     `json:"id"`
	Title        string  `json:"title"`
	Overview     string  `json:"overview"`
	PosterPath   string  `json:"poster_path"`
	BackdropPath string  `json:"backdrop_path"`
	ReleaseDate  string  `json:"release_date"`
	VoteAverage  float64 `json:"vote_average"`
	VoteCount    int     `json:"vote_count"`
	GenreIDs     []int   `json:"genre_ids"`
	Adult        bool    `json:"adult"`
	OriginalLang string  `json:"original_language"`
}

// Genre represents a genre from TMDB
type Genre struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// SearchMoviesResponse is the response from TMDB search
type SearchMoviesResponse struct {
	Page         int     `json:"page"`
	Results      []Movie `json:"results"`
	TotalPages   int     `json:"total_pages"`
	TotalResults int     `json:"total_results"`
}

// TrendingMoviesResponse is the response from TMDB trending
type TrendingMoviesResponse struct {
	Page         int     `json:"page"`
	Results      []Movie `json:"results"`
	TotalPages   int     `json:"total_pages"`
	TotalResults int     `json:"total_results"`
}

// MovieDetails is the detailed movie info from TMDB
type MovieDetails struct {
	ID           int     `json:"id"`
	Title        string  `json:"title"`
	Overview     string  `json:"overview"`
	PosterPath   string  `json:"poster_path"`
	BackdropPath string  `json:"backdrop_path"`
	ReleaseDate  string  `json:"release_date"`
	VoteAverage  float64 `json:"vote_average"`
	VoteCount    int     `json:"vote_count"`
	Genres       []Genre `json:"genres"`
	Runtime      int     `json:"runtime"`
	Tagline      string  `json:"tagline"`
	Adult        bool    `json:"adult"`
	OriginalLang string  `json:"original_language"`
	Budget       int64   `json:"budget"`
	Revenue      int64   `json:"revenue"`
	Status       string  `json:"status"`
}

// GenreListResponse is the response from TMDB genres
type GenreListResponse struct {
	Genres []Genre `json:"genres"`
}

// MovieVideosResponse is the response from TMDB movie videos endpoint
type MovieVideosResponse struct {
	ID      int      `json:"id"`
	Results []Video  `json:"results"`
}

// Video represents a movie video (trailer, teaser, etc.)
type Video struct {
	ID       string `json:"id"`
	Key      string `json:"key"`
Name     string `json:"name"`
Site     string `json:"site"`
Type     string `json:"type"`
Official bool   `json:"official"`
}

// MovieReviewsResponse is the response from TMDB movie reviews endpoint
type MovieReviewsResponse struct {
	ID       int       `json:"id"`
	Page     int       `json:"page"`
	Results  []Review  `json:"results"`
	TotalPages int     `json:"total_pages"`
	TotalResults int   `json:"total_results"`
}

// Review represents a TMDB movie review
type Review struct {
	ID           string `json:"id"`
	Author       string `json:"author"`
	Content      string `json:"content"`
	CreatedAt    string `json:"created_at"`
	UpdatedAt    string `json:"updated_at"`
	URL          string `json:"url"`
	AuthorDetails AuthorDetails `json:"author_details"`
}

// AuthorDetails contains author metadata from TMDB
type AuthorDetails struct {
	Name       string   `json:"name"`
	Username   string   `json:"username"`
	AvatarPath *string  `json:"avatar_path"`
	Rating     *float64 `json:"rating"`
}

// WatchProvidersResponse is the response from TMDB watch providers endpoint
type WatchProvidersResponse struct {
	Results map[string]RegionProviders `json:"results"`
}

// RegionProviders contains watch provider info for a region
type RegionProviders struct {
	Link      string     `json:"link"`
	Flatrate []Provider `json:"flatrate,omitempty"`
	Rent     []Provider `json:"rent,omitempty"`
	Buy      []Provider `json:"buy,omitempty"`
}

// Provider represents a streaming/watching provider
type Provider struct {
	ProviderID      int    `json:"provider_id"`
	ProviderName    string `json:"provider_name"`
	LogoPath        string `json:"logo_path"`
	DisplayPriority int    `json:"display_priority"`
}

// CreditsResponse is the response from TMDB movie credits endpoint
type CreditsResponse struct {
	ID      *int     `json:"id,omitempty"`
	Cast    []Cast   `json:"cast"`
	Crew    []Crew   `json:"crew"`
}

// Cast represents a cast member
type Cast struct {
	ID           int    `json:"id"`
Name         string `json:"name"`
Character    string `json:"character"`
ProfilePath  string `json:"profile_path"`
Order        int    `json:"order"`
}

// Crew represents a crew member
type Crew struct {
ID           int    `json:"id"`
Name         string `json:"name"`
Job          string `json:"job"`
Department   string `json:"department"`
ProfilePath  string `json:"profile_path"`
}

// SearchMovies calls TMDB search/movie endpoint
func (c *Client) SearchMovies(query string, page int) (*SearchMoviesResponse, error) {
	url := fmt.Sprintf("%s/search/movie?api_key=%s&query=%s&page=%d", baseURL, apiKey, query, page)

	resp, err := c.doRequest(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result SearchMoviesResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// GetTrending calls TMDB trending/movie endpoint
func (c *Client) GetTrending(page int) (*TrendingMoviesResponse, error) {
	url := fmt.Sprintf("%s/trending/movie/week?api_key=%s&page=%d", baseURL, apiKey, page)

	resp, err := c.doRequest(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result TrendingMoviesResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// GetMovieDetails calls TMDB movie/{id} endpoint
func (c *Client) GetMovieDetails(tmdbID int) (*MovieDetails, error) {
	url := fmt.Sprintf("%s/movie/%d?api_key=%s", baseURL, tmdbID, apiKey)

	resp, err := c.doRequest(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result MovieDetails
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// AppendedDetailsResponse holds movie details with credits and watch providers
type AppendedDetailsResponse struct {
	MovieDetails
	Credits      CreditsResponse      `json:"credits"`
	WatchProviders WatchProvidersResponse `json:"watch/providers"`
}

// GetMovieDetailsWithAppend calls TMDB movie/{id}?append_to_response=credits,watch/providers
func (c *Client) GetMovieDetailsWithAppend(tmdbID int) (*AppendedDetailsResponse, error) {
	url := fmt.Sprintf("%s/movie/%d?api_key=%s&append_to_response=credits,watch/providers", baseURL, tmdbID, apiKey)

	resp, err := c.doRequest(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result AppendedDetailsResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// GetMovieVideos calls TMDB movie/{id}/videos endpoint
func (c *Client) GetMovieVideos(tmdbID int) (*MovieVideosResponse, error) {
	url := fmt.Sprintf("%s/movie/%d/videos?api_key=%s", baseURL, tmdbID, apiKey)

	resp, err := c.doRequest(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result MovieVideosResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// GetMovieReviews calls TMDB movie/{id}/reviews endpoint
func (c *Client) GetMovieReviews(tmdbID int, page int) (*MovieReviewsResponse, error) {
	url := fmt.Sprintf("%s/movie/%d/reviews?api_key=%s&page=%d", baseURL, tmdbID, apiKey, page)

	resp, err := c.doRequest(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result MovieReviewsResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// GetGenres calls TMDB genre/movie/list endpoint
func (c *Client) GetGenres() (*GenreListResponse, error) {
	url := fmt.Sprintf("%s/genre/movie/list?api_key=%s", baseURL, apiKey)

	resp, err := c.doRequest(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result GenreListResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// PosterURL returns the full poster path URL
func PosterURL(path string) string {
	if path == "" {
		return ""
	}
	return fmt.Sprintf("%s/w500%s", ImageBaseURL, path)
}

// BackdropURL returns the full backdrop path URL
func BackdropURL(path string) string {
	if path == "" {
		return ""
	}
	return fmt.Sprintf("%s/w1280%s", ImageBaseURL, path)
}

// ProfileURL returns the full profile path URL
func ProfileURL(path string) string {
	if path == "" {
		return ""
	}
	return fmt.Sprintf("%s/w185%s", ImageBaseURL, path)
}

// LogoURL returns the full logo path URL for streaming providers
func LogoURL(path string) string {
	if path == "" {
		return ""
	}
	return fmt.Sprintf("%s/w92%s", ImageBaseURL, path)
}
