package reviews

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/trynafindbhumik/cinematch-backend/internal/shared/redis"
	"github.com/trynafindbhumik/cinematch-backend/internal/shared/tmdb"
)

const (
	defaultLimit = 10
	MovieCacheTTL = 60 * 60 // 1 hour
)

type Service struct {
	repo       *Repository
	db         *pgxpool.Pool
	tmdbClient *tmdb.Client
}

func NewService(repo *Repository, db *pgxpool.Pool, tmdbClient *tmdb.Client) *Service {
	return &Service{
		repo:       repo,
		db:         db,
		tmdbClient: tmdbClient,
	}
}

// Create creates a new review
func (s *Service) Create(ctx context.Context, userID int64, req *CreateReviewRequest) (*ReviewResponse, error) {
	// Ensure movie exists in DB (Redis -> DB -> TMDB)
	_, err := s.getMovieInfo(ctx, req.TMDBID)
	if err != nil {
		return nil, fmt.Errorf("failed to get movie info: %w", err)
	}

	review, err := s.repo.Create(ctx, userID, req.TMDBID, req.Rating, req.Comment)
	if err != nil {
		return nil, err
	}

	return &ReviewResponse{
		ID:        review.ID,
		UserID:    review.UserID,
		TMDBID:    review.TMDBID,
		Rating:    review.Rating,
		Comment:   review.Comment,
		CreatedAt: review.CreatedAt,
	}, nil
}

// MovieInfo holds movie data for reviews
type MovieInfo struct {
	Title     string
	PosterURL string
}

// getMovieInfo fetches movie info from Redis -> DB -> TMDB
func (s *Service) getMovieInfo(ctx context.Context, tmdbID int) (*MovieInfo, error) {
	// 1. Check Redis cache first
	cacheKey := fmt.Sprintf("movies:detail:%d", tmdbID)
	rdb := redis.Client()
	if rdb != nil {
		data, err := rdb.Get(ctx, cacheKey).Bytes()
		if err == nil {
			var movieDetails struct {
				Title     string `json:"title"`
				PosterURL string `json:"poster_url"`
			}
			if json.Unmarshal(data, &movieDetails) == nil {
				return &MovieInfo{
					Title:     movieDetails.Title,
					PosterURL: movieDetails.PosterURL,
				}, nil
			}
		}
	}

	// 2. Check database
	var title, posterURL string
	err := s.db.QueryRow(ctx, `
		SELECT title, poster_url FROM movies WHERE tmdb_id = $1
	`, tmdbID).Scan(&title, &posterURL)
	if err == nil {
		// Cache in Redis for next time
		if rdb != nil {
			cacheData, _ := json.Marshal(map[string]string{"title": title, "poster_url": posterURL})
			rdb.Set(ctx, cacheKey, cacheData, time.Duration(MovieCacheTTL)*time.Second)
		}
		return &MovieInfo{Title: title, PosterURL: posterURL}, nil
	}

	// 3. Fetch from TMDB and save to DB
	movieDetails, err := s.tmdbClient.GetMovieDetails(tmdbID)
	if err != nil {
		return nil, err
	}

	posterURL = tmdb.PosterURL(movieDetails.PosterPath)

	// Save to database
	year := extractYear(movieDetails.ReleaseDate)
	rating := int(movieDetails.VoteAverage * 10)
	genres := make([]string, 0, len(movieDetails.Genres))
	for _, g := range movieDetails.Genres {
		genres = append(genres, g.Name)
	}

	_, err = s.db.Exec(ctx, `
		INSERT INTO movies (tmdb_id, title, poster_url, genres, release_year, tmdb_rating, backdrop_url)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (tmdb_id) DO UPDATE SET
			title = EXCLUDED.title,
			poster_url = EXCLUDED.poster_url,
			backdrop_url = EXCLUDED.backdrop_url
	`, tmdbID, movieDetails.Title, posterURL, genres, year, rating, tmdb.BackdropURL(movieDetails.BackdropPath))
	if err != nil {
		// Log but continue - we have the data
		fmt.Printf("failed to save movie to DB: %v\n", err)
	}

	// Cache in Redis
	if rdb != nil {
		cacheData, _ := json.Marshal(map[string]string{"title": movieDetails.Title, "poster_url": posterURL})
		rdb.Set(ctx, cacheKey, cacheData, time.Duration(MovieCacheTTL)*time.Second)
	}

	return &MovieInfo{Title: movieDetails.Title, PosterURL: posterURL}, nil
}

func extractYear(dateStr string) int {
	if dateStr == "" {
		return 0
	}
	t, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		return 0
	}
	return t.Year()
}

// Update updates an existing review
func (s *Service) Update(ctx context.Context, userID int64, reviewID int64, req *UpdateReviewRequest) (*ReviewResponse, error) {
	review, err := s.repo.Update(ctx, userID, reviewID, req)
	if err != nil {
		return nil, err
	}

	return &ReviewResponse{
		ID:        review.ID,
		UserID:    review.UserID,
		TMDBID:    review.TMDBID,
		Rating:    review.Rating,
		Comment:   review.Comment,
		CreatedAt: review.CreatedAt,
	}, nil
}

// Delete deletes a review
func (s *Service) Delete(ctx context.Context, userID int64, reviewID int64) error {
	return s.repo.Delete(ctx, userID, reviewID)
}

// GetByID gets a single review by ID
func (s *Service) GetByID(ctx context.Context, reviewID int64) (*ReviewResponse, error) {
	review, err := s.repo.GetByID(ctx, reviewID)
	if err != nil {
		return nil, err
	}

	return &ReviewResponse{
		ID:        review.ID,
		UserID:    review.UserID,
		TMDBID:    review.TMDBID,
		Rating:    review.Rating,
		Comment:   review.Comment,
		CreatedAt: review.CreatedAt,
	}, nil
}

// GetUserReviews gets reviews for a user with cursor pagination and optional date filtering
func (s *Service) GetUserReviews(ctx context.Context, userID int64, cursor string, limit int, fromDate, toDate *time.Time) (*ReviewsListResponse, error) {
	fmt.Printf("[DEBUG Service GetUserReviews] UserID: %d | Cursor: %s | Limit: %d\n", userID, cursor, limit)
	if limit <= 0 {
		limit = defaultLimit
	}

	reviews, nextCursor, err := s.repo.GetByUserID(ctx, userID, cursor, limit, fromDate, toDate)
	if err != nil {
		fmt.Printf("[DEBUG Service GetUserReviews] Repo error: %v\n", err)
		return nil, err
	}

	items := make([]ReviewsListItem, 0, len(reviews))
	for _, r := range reviews {
		rating := float64(r.Rating)
		items = append(items, ReviewsListItem{
			ID:             fmt.Sprintf("%d", r.ID),
			TMDBID:         r.TMDBID,
			Title:          r.Title,
			PosterURL:      r.PosterURL,
			Rating:         &rating,
			Comment:        r.Comment,
			AuthorPicture:  r.AuthorPicture,
			CreatedAt:      r.CreatedAt,
			Source:         "db",
		})
	}

	return &ReviewsListResponse{
		Reviews:    items,
		NextCursor: nextCursor,
		HasMore:    nextCursor != "",
	}, nil
}

// GetMovieReviews gets reviews for a movie with hybrid cursor pagination (DB + TMDB)
func (s *Service) GetMovieReviews(ctx context.Context, tmdbID int, cursor string, limit int) (*ReviewsListResponse, error) {
	if limit <= 0 {
		limit = defaultLimit
	}

	// Decode cursor
	var source string = "db"
	var dbLastID int64 = 0
	var tmdbPage int = 1

	if cursor != "" {
	 decoded, err := DecodeCursor(cursor)
		if err == nil {
			source = decoded.Source
			if decoded.Source == "db" && decoded.DBCursor != "" {
				fmt.Sscanf(decoded.DBCursor, "%d", &dbLastID)
			} else if decoded.Source == "tmdb" {
				tmdbPage = decoded.TMDBPage
			}
		}
	}

	// If DB source or no cursor, fetch from DB first
	if source == "db" || cursor == "" {
		reviews, dbNextCursor, err := s.repo.GetByMovieID(ctx, tmdbID, cursor, limit)
		if err != nil {
			return nil, err
		}

		items := make([]ReviewsListItem, 0, len(reviews))

		for _, r := range reviews {
			rating := float64(r.Rating)
			items = append(items, ReviewsListItem{
				ID:            fmt.Sprintf("%d", r.ID),
				UserID:        r.UserID,
				TMDBID:        r.TMDBID,
				Title:         r.Title,
				PosterURL:      r.PosterURL,
				Rating:        &rating,
				Comment:       r.Comment,
				AuthorName:    r.AuthorName,
				AuthorPicture: r.AuthorPicture,
				CreatedAt:     r.CreatedAt,
				Source:        "db",
			})
		}

		// If we got full limit from DB, there's more in DB
		if len(reviews) >= limit && dbNextCursor != "" {
			return &ReviewsListResponse{
				Reviews:    items,
				NextCursor: dbNextCursor,
				HasMore:    true,
			}, nil
		}

		// DB exhausted, fetch from TMDB
		tmdbReviews, totalPages, err := s.repo.GetTMDBReviews(ctx, tmdbID, tmdbPage)
		if err == nil && len(tmdbReviews) > 0 {
			tmdbItems := make([]ReviewsListItem, 0, len(tmdbReviews))
			for _, r := range tmdbReviews {
				t, _ := time.Parse(time.RFC3339, r.CreatedAt)
				tmdbItems = append(tmdbItems, ReviewsListItem{
					ID:            r.ID,
					AuthorName:    r.Author,
					Content:       r.Content,
					AuthorPicture: buildAvatarURL(r.AvatarPath),
					Rating:        r.Rating,
					CreatedAt:     t,
					Source:        "tmdb",
				})
			}
			items = append(items, tmdbItems...)

			nextCursor := ""
			if tmdbPage < totalPages {
				c := &Cursor{Source: "tmdb", TMDBPage: tmdbPage + 1}
				nextCursor = EncodeCursor(c)
			}

			return &ReviewsListResponse{
				Reviews:    items,
				NextCursor: nextCursor,
				HasMore:    nextCursor != "",
			}, nil
		}

		return &ReviewsListResponse{
			Reviews:    items,
			NextCursor: dbNextCursor,
			HasMore:    dbNextCursor != "",
		}, nil
	}

	// TMDB source - fetch from TMDB directly
	tmdbReviews, totalPages, err := s.repo.GetTMDBReviews(ctx, tmdbID, tmdbPage)
	if err != nil {
		return nil, err
	}

	items := make([]ReviewsListItem, 0, len(tmdbReviews))
	for _, r := range tmdbReviews {
		t, _ := time.Parse(time.RFC3339, r.CreatedAt)
		items = append(items, ReviewsListItem{
			ID:            r.ID,
			AuthorName:    r.Author,
			Content:       r.Content,
			AuthorPicture: buildAvatarURL(r.AvatarPath),
			Rating:        r.Rating,
			CreatedAt:     t,
			Source:        "tmdb",
		})
	}

	nextCursor := ""
	if tmdbPage < totalPages {
		c := &Cursor{Source: "tmdb", TMDBPage: tmdbPage + 1}
		nextCursor = EncodeCursor(c)
	}

	return &ReviewsListResponse{
		Reviews:    items,
		NextCursor: nextCursor,
		HasMore:    nextCursor != "",
	}, nil
}

func pointerString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func buildAvatarURL(path *string) *string {
	if path == nil || *path == "" {
		return nil
	}
	fullURL := "https://image.tmdb.org/t/p/original" + *path
	return &fullURL
}