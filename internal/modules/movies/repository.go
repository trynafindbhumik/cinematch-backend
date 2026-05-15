package movies

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/trynafindbhumik/cinematch-backend/internal/db"
	"github.com/trynafindbhumik/cinematch-backend/internal/modules/genres"
	"github.com/trynafindbhumik/cinematch-backend/internal/shared/logger"
	"github.com/trynafindbhumik/cinematch-backend/internal/shared/redis"
	"github.com/trynafindbhumik/cinematch-backend/internal/shared/tmdb"
)

const genreCacheKey = "genres:map"

// MovieVotes contains vote counts from our database
type MovieVotes struct {
	TotalCount   int
	LikeCount    int
	LoveCount    int
	DislikeCount int
	HateCount    int
	SkipCount    int
}

// Repository handles movie data operations
type Repository struct {
	tmdbClient *tmdb.Client
	genresRepo *genres.Repository
}

// NewRepository creates a new movies repository
func NewRepository(tmdbClient *tmdb.Client, genresRepo *genres.Repository) *Repository {
	repo := &Repository{
		tmdbClient: tmdbClient,
		genresRepo: genresRepo,
	}
	// Ensure genres are synced to DB
	repo.ensureGenresSynced()
	return repo
}

// ensureGenresSynced ensures genres are synced from TMDB to DB
func (r *Repository) ensureGenresSynced() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Check if genres exist in DB
	genreMap, err := r.genresRepo.GetGenreMap(ctx)
	if err != nil || len(genreMap) == 0 {
		logger.Info("genres table empty, syncing from TMDB")
		r.syncGenresFromTMDB()
		return
	}

	// Cache in Redis
	r.cacheGenresInRedis(genreMap)
	logger.Info("genres loaded from DB", logger.Int("count", len(genreMap)))
}

// syncGenresFromTMDB fetches genres from TMDB and saves to DB
func (r *Repository) syncGenresFromTMDB() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	tmdbGenres, err := r.tmdbClient.GetGenres()
	if err != nil {
		logger.Error("failed to fetch genres from TMDB", logger.Err(err))
		return
	}

	// Save to DB
	for _, g := range tmdbGenres.Genres {
		_, err := db.Pool().Exec(ctx, `
			INSERT INTO genres (tmdb_id, name)
			VALUES ($1, $2)
			ON CONFLICT (tmdb_id) DO UPDATE SET name = EXCLUDED.name
		`, g.ID, g.Name)
		if err != nil {
			logger.Error("failed to save genre", logger.Err(err), logger.Int("tmdb_id", g.ID))
		}
	}

	// Reload from DB and cache in Redis
	genreMap, err := r.genresRepo.GetGenreMap(ctx)
	if err != nil {
		logger.Error("failed to load genre map after sync", logger.Err(err))
		return
	}

	r.cacheGenresInRedis(genreMap)
	logger.Info("genres synced from TMDB to DB", logger.Int("count", len(tmdbGenres.Genres)))
}

// cacheGenresInRedis saves genre map to Redis
func (r *Repository) cacheGenresInRedis(genreMap map[int]string) {
	rdb := redis.Client()
	if rdb == nil {
		return
	}

	data, err := json.Marshal(genreMap)
	if err != nil {
		logger.Error("failed to marshal genre map", logger.Err(err))
		return
	}

	// Cache for 24 hours
	rdb.Set(context.Background(), genreCacheKey, data, 24*time.Hour)
}

// getGenresFromCache retrieves genres from Redis and converts IDs to names
func (r *Repository) getGenresFromCache(genreIDs []int) []string {
	if len(genreIDs) == 0 {
		return nil
	}

	rdb := redis.Client()
	if rdb == nil {
		return nil
	}

	data, err := rdb.Get(context.Background(), genreCacheKey).Bytes()
	if err != nil {
		logger.Warn("failed to get genres from Redis", logger.Err(err))
		return nil
	}

	var genreMap map[int]string
	if err := json.Unmarshal(data, &genreMap); err != nil {
		logger.Warn("failed to unmarshal genre map", logger.Err(err))
		return nil
	}

	genres := make([]string, 0, len(genreIDs))
	for _, id := range genreIDs {
		if name, ok := genreMap[id]; ok {
			genres = append(genres, name)
		}
	}
	return genres
}

// GetMovieVotes retrieves vote counts for a movie from our DB
func (r *Repository) GetMovieVotes(ctx context.Context, tmdbID int) (*MovieVotes, error) {
	var votes MovieVotes
	var totalCount, likeCount, loveCount, dislikeCount, hateCount, skipCount *int

	err := db.Pool().QueryRow(ctx, `
		SELECT total_count, like_count, love_count, dislike_count, hate_count, skip_count
		FROM movies WHERE tmdb_id = $1
	`, tmdbID).Scan(&totalCount, &likeCount, &loveCount, &dislikeCount, &hateCount, &skipCount)

	if err == nil {
		if totalCount != nil { votes.TotalCount = *totalCount }
		if likeCount != nil { votes.LikeCount = *likeCount }
		if loveCount != nil { votes.LoveCount = *loveCount }
		if dislikeCount != nil { votes.DislikeCount = *dislikeCount }
		if hateCount != nil { votes.HateCount = *hateCount }
		if skipCount != nil { votes.SkipCount = *skipCount }
		return &votes, nil
	}
	// Movie not in DB - return zeros
	return &MovieVotes{}, nil
}

// GetUserReaction retrieves a user's reaction for a specific movie
func (r *Repository) GetUserReaction(ctx context.Context, tmdbID int, userID int64) (string, error) {
	var reaction string
	err := db.Pool().QueryRow(ctx, `
		SELECT ur.reaction FROM user_reaction ur
		JOIN movies m ON ur.movie_id = m.id
		WHERE m.tmdb_id = $1 AND ur.user_id = $2
	`, tmdbID, userID).Scan(&reaction)
	if err != nil {
		return "", nil // No reaction found, not an error
	}
	return reaction, nil
}

// EnsureMovieExists inserts a movie into our DB if it doesn't exist
func (r *Repository) EnsureMovieExists(ctx context.Context, tmdbID int, title, posterURL, backdropURL string, releaseYear, tmdbRating int, genres []string) error {
	_, err := db.Pool().Exec(ctx, `
		INSERT INTO movies (tmdb_id, title, poster_url, backdrop_url, release_year, tmdb_rating, genres)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (tmdb_id) DO NOTHING
	`, tmdbID, title, posterURL, backdropURL, releaseYear, tmdbRating, genres)
	return err
}

// SearchMovies searches movies via TMDB API
func (r *Repository) SearchMovies(ctx context.Context, query string, page int) (*SearchResponse, error) {
	result, err := r.tmdbClient.SearchMovies(query, page)
	if err != nil {
		return nil, fmt.Errorf("failed to search movies: %w", err)
	}

	movies := make([]MovieResponse, 0, len(result.Results))
	for _, m := range result.Results {
		year := extractYear(m.ReleaseDate)
		rating := int(m.VoteAverage * 10) // Convert 0-10 to 0-100 scale like in DB schema
		movies = append(movies, MovieResponse{
			TMDBID:      m.ID,
			Title:       m.Title,
			PosterURL:   tmdb.PosterURL(m.PosterPath),
			BackdropURL: tmdb.BackdropURL(m.BackdropPath),
			ReleaseYear: year,
			TMDBRating:  rating,
			Genres:      r.getGenresFromCache(m.GenreIDs),
		})
	}

	return &SearchResponse{
		Movies:       movies,
		Page:         result.Page,
		TotalPages:   result.TotalPages,
		TotalResults: result.TotalResults,
	}, nil
}

// GetTrending gets trending movies from TMDB API
func (r *Repository) GetTrending(ctx context.Context, page int) (*TrendingResponse, error) {
	result, err := r.tmdbClient.GetTrending(page)
	if err != nil {
		return nil, fmt.Errorf("failed to get trending: %w", err)
	}

	movies := make([]MovieResponse, 0, len(result.Results))
	for _, m := range result.Results {
		year := extractYear(m.ReleaseDate)
		rating := int(m.VoteAverage * 10)
		movies = append(movies, MovieResponse{
			TMDBID:      m.ID,
			Title:       m.Title,
			PosterURL:   tmdb.PosterURL(m.PosterPath),
			BackdropURL: tmdb.BackdropURL(m.BackdropPath),
			ReleaseYear: year,
			TMDBRating:  rating,
			Genres:      r.getGenresFromCache(m.GenreIDs),
		})
	}

	return &TrendingResponse{
		Movies:       movies,
		Page:         result.Page,
		TotalPages:   result.TotalPages,
		TotalResults: result.TotalResults,
	}, nil
}

// GetMovieByID gets detailed movie info from TMDB (now uses append_to_response)
func (r *Repository) GetMovieByID(ctx context.Context, tmdbID int) (*MovieDetailsResponse, error) {
	result, err := r.tmdbClient.GetMovieDetailsWithAppend(tmdbID)
	if err != nil {
		return nil, fmt.Errorf("failed to get movie details: %w", err)
	}

	// DEBUG: Check what we got from TMDB
	logger.Info("TMDB response", 
		logger.Int("tmdb_id", tmdbID),
		logger.Int("credits_cast_len", len(result.Credits.Cast)),
		logger.Int("watch_providers_regions", len(result.WatchProviders.Results)))

	year := extractYear(result.ReleaseDate)
	rating := int(result.VoteAverage * 10)
	genres := make([]string, 0, len(result.Genres))
	for _, g := range result.Genres {
		genres = append(genres, g.Name)
	}

	// Transform credits (cast only, no crew)
	var credits *CreditsResponse
	if len(result.Credits.Cast) > 0 {
		cast := make([]CastMember, 0, len(result.Credits.Cast))
		for _, c := range result.Credits.Cast {
			cast = append(cast, CastMember{
				ID:         c.ID,
				Name:       c.Name,
				Character:  c.Character,
				ProfileURL: tmdb.ProfileURL(c.ProfilePath),
				Order:      c.Order,
			})
		}
		credits = &CreditsResponse{Cast: cast}
	}

	// Transform watch providers
	var watchProviders *WatchProvidersDTO
	if len(result.WatchProviders.Results) > 0 {
		wp := &WatchProvidersDTO{Results: make(map[string][]ProviderInfo)}
		for region, regionData := range result.WatchProviders.Results {
			providerList := make([]ProviderInfo, 0)
			// Flatten flatrate, rent, and buy into a single list
			for _, p := range regionData.Flatrate {
				providerList = append(providerList, ProviderInfo{
					ProviderID:      p.ProviderID,
					ProviderName:   p.ProviderName,
					LogoURL:        tmdb.LogoURL(p.LogoPath),
					DisplayPriority: p.DisplayPriority,
				})
			}
			for _, p := range regionData.Rent {
				providerList = append(providerList, ProviderInfo{
					ProviderID:      p.ProviderID,
					ProviderName:   p.ProviderName,
					LogoURL:        tmdb.LogoURL(p.LogoPath),
					DisplayPriority: p.DisplayPriority,
				})
			}
			for _, p := range regionData.Buy {
				providerList = append(providerList, ProviderInfo{
					ProviderID:      p.ProviderID,
					ProviderName:   p.ProviderName,
					LogoURL:        tmdb.LogoURL(p.LogoPath),
					DisplayPriority: p.DisplayPriority,
				})
			}
			if len(providerList) > 0 {
				wp.Results[region] = providerList
			}
		}
		watchProviders = wp
	}

	return &MovieDetailsResponse{
		MovieResponse: MovieResponse{
			TMDBID:      result.ID,
			Title:       result.Title,
			PosterURL:   tmdb.PosterURL(result.PosterPath),
			BackdropURL: tmdb.BackdropURL(result.BackdropPath),
			ReleaseYear: year,
			TMDBRating:  rating,
			Genres:      genres,
			Runtime:     result.Runtime,
			Tagline:     result.Tagline,
			Status:      result.Status,
		},
		Budget:         result.Budget,
		Revenue:        result.Revenue,
		Adult:          result.Adult,
		Credits:        credits,
		WatchProviders: watchProviders,
	}, nil
}

// GetMovieVideos gets movie videos from TMDB
func (r *Repository) GetMovieVideos(ctx context.Context, tmdbID int) (*VideosResponse, error) {
	result, err := r.tmdbClient.GetMovieVideos(tmdbID)
	if err != nil {
		return nil, fmt.Errorf("failed to get movie videos: %w", err)
	}

	videos := make([]VideoDetail, 0, len(result.Results))
	for _, v := range result.Results {
		videos = append(videos, VideoDetail{
			ID:       v.ID,
			Key:      v.Key,
			Name:     v.Name,
			Site:     v.Site,
			Type:     v.Type,
			Official: v.Official,
		})
	}

	return &VideosResponse{
		ID:     result.ID,
		Videos: videos,
	}, nil
}

// extractYear extracts year from date string (YYYY-MM-DD)
func extractYear(dateStr string) int {
	if dateStr == "" {
		return 0
	}
	// Date format from TMDB: "2024-01-15"
	t, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		return 0
	}
	return t.Year()
}