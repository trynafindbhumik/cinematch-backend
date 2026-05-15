package reviews

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/trynafindbhumik/cinematch-backend/internal/shared/tmdb"
)

type Repository struct {
	db         *pgxpool.Pool
	tmdbClient *tmdb.Client
}

// NewRepository creates a new reviews repository
func NewRepository(db *pgxpool.Pool, tmdbClient *tmdb.Client) *Repository {
	return &Repository{
		db:         db,
		tmdbClient: tmdbClient,
	}
}

// Create creates a new review in the database
func (r *Repository) Create(ctx context.Context, userID int64, tmdbID int, rating int, comment string) (*Review, error) {
	var review Review
	err := r.db.QueryRow(ctx, `
		INSERT INTO user_reviews (user_id, movie_id, rating, comment)
		VALUES ($1, $2, $3, $4)
		RETURNING id, user_id, movie_id, rating, comment, created_at
	`, userID, tmdbID, rating, comment).
		Scan(&review.ID, &review.UserID, &review.MovieID, &review.Rating, &review.Comment, &review.CreatedAt)

	if err != nil {
		return nil, fmt.Errorf("failed to create review: %w", err)
	}

	review.TMDBID = tmdbID
	return &review, nil
}

// Update updates an existing review
func (r *Repository) Update(ctx context.Context, userID int64, reviewID int64, req *UpdateReviewRequest) (*Review, error) {
	var review Review
	err := r.db.QueryRow(ctx, `
		UPDATE user_reviews
		SET rating = COALESCE(NULLIF($1, 0), rating),
		    comment = COALESCE(NULLIF($2, ''), comment)
		WHERE id = $3 AND user_id = $4
		RETURNING id, user_id, movie_id, rating, comment, created_at
	`, req.Rating, req.Comment, reviewID, userID).
		Scan(&review.ID, &review.UserID, &review.MovieID, &review.Rating, &review.Comment, &review.CreatedAt)

	if err == pgx.ErrNoRows {
		return nil, fmt.Errorf("review not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to update review: %w", err)
	}

	review.TMDBID = review.MovieID
	return &review, nil
}

// Delete deletes a review
func (r *Repository) Delete(ctx context.Context, userID int64, reviewID int64) error {
	result, err := r.db.Exec(ctx, `
		DELETE FROM user_reviews WHERE id = $1 AND user_id = $2
	`, reviewID, userID)
	if err != nil {
		return fmt.Errorf("failed to delete review: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("review not found")
	}

	return nil
}

// GetByUserID retrieves reviews for a user with pagination and optional date filtering
func (r *Repository) GetByUserID(ctx context.Context, userID int64, cursor string, limit int, fromDate, toDate *time.Time) ([]Review, string, error) {
	fmt.Printf("[DEBUG GetByUserID] Start | Cursor: %s | Limit: %d\n", cursor, limit)
	var reviews []Review
	var nextCursor string

	// Parse cursor for created_at timestamp (descending order)
	var lastCreatedAt time.Time
	var cursorErr error
	if cursor != "" {
		decoded, err := DecodeCursor(cursor)
		if err != nil {
			cursorErr = fmt.Errorf("failed to decode cursor: %w", err)
		} else if decoded.Source == "db" && decoded.DBCursor != "" {
			fmt.Printf("[DEBUG GetByUserID] Decoded cursor - Source: %s, DBCursor: %s\n", decoded.Source, decoded.DBCursor)
			// Handle both RFC3339 (with timezone) and RFC3339Nano formats
			lastCreatedAt, cursorErr = time.Parse(time.RFC3339, decoded.DBCursor)
			if cursorErr != nil {
				fmt.Printf("[DEBUG GetByUserID] RFC3339 parse failed: %v, trying RFC3339Nano...\n", cursorErr)
				// Try RFC3339Nano as fallback
				lastCreatedAt, cursorErr = time.Parse(time.RFC3339Nano, decoded.DBCursor)
				if cursorErr != nil {
					fmt.Printf("[DEBUG GetByUserID] RFC3339Nano parse also failed: %v\n", cursorErr)
				}
			} else {
				fmt.Printf("[DEBUG GetByUserID] Successfully parsed with RFC3339: %v\n", lastCreatedAt)
			}
		}
	}
	if cursorErr != nil {
		return nil, "", cursorErr
	}

	// Build date filter conditions
	var dateConditions []string
	var dateArgs []interface{}
	argIdx := 2

	if fromDate != nil {
		dateConditions = append(dateConditions, fmt.Sprintf("r.created_at >= $%d", argIdx))
		dateArgs = append(dateArgs, *fromDate)
		argIdx++
	}
	if toDate != nil {
		dateConditions = append(dateConditions, fmt.Sprintf("r.created_at <= $%d", argIdx))
		dateArgs = append(dateArgs, *toDate)
		argIdx++
	}

	// Build query with optional date filtering
	query := `
		SELECT r.id, r.user_id, r.movie_id, r.rating, r.comment, r.created_at,
		       m.title, m.poster_url, u.profile_url
		FROM user_reviews r
		LEFT JOIN movies m ON r.movie_id = m.tmdb_id
		JOIN users u ON r.user_id = u.id
		WHERE r.user_id = $1`

	// Append date conditions if any
	if len(dateConditions) > 0 {
		query += " AND " + strings.Join(dateConditions, " AND ")
	}

	// Cursor-based pagination using created_at DESC
	if !lastCreatedAt.IsZero() {
		query += fmt.Sprintf(" AND r.created_at < $%d", argIdx)
		argIdx++
	}

	// Order by created_at DESC for chronological order (newest first)
	query += " ORDER BY r.created_at DESC, r.id DESC LIMIT $" + fmt.Sprintf("%d", argIdx)

	args := []interface{}{userID}
	args = append(args, dateArgs...)
	if !lastCreatedAt.IsZero() {
		args = append(args, lastCreatedAt)
	}
	args = append(args, limit)

	fmt.Printf("[DEBUG GetByUserID] Final Query: %s\n", query)
	fmt.Printf("[DEBUG GetByUserID] Args: %v\n", args)

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		fmt.Printf("[DEBUG GetByUserID] Query error: %v\n", err)
		return nil, "", fmt.Errorf("failed to get reviews: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var review Review
		var title, posterURL *string
		var profileURL *string
		err := rows.Scan(&review.ID, &review.UserID, &review.MovieID, &review.Rating, &review.Comment, &review.CreatedAt, &title, &posterURL, &profileURL)
		if err != nil {
			continue
		}
		review.TMDBID = review.MovieID
		review.Title = title     // Already *string, can be nil
		review.PosterURL = posterURL // Already *string, can be nil
		review.AuthorPicture = profileURL
		reviews = append(reviews, review)
	}

	if len(reviews) == limit {
		lastReview := reviews[len(reviews)-1]
		cursor := &Cursor{Source: "db", DBCursor: lastReview.CreatedAt.Format(time.RFC3339)}
		nextCursor = EncodeCursor(cursor)
	}

	return reviews, nextCursor, nil
}

// GetByMovieID retrieves reviews for a specific movie with user info
func (r *Repository) GetByMovieID(ctx context.Context, movieID int, cursor string, limit int) ([]Review, string, error) {
	var reviews []Review
	var nextCursor string

	// Parse cursor for created_at timestamp (descending order)
	var lastCreatedAt time.Time
	var cursorErr error
	if cursor != "" {
		decoded, err := DecodeCursor(cursor)
		if err != nil {
			cursorErr = fmt.Errorf("failed to decode cursor: %w", err)
		} else if decoded.Source == "db" && decoded.DBCursor != "" {
			// Handle both RFC3339 (with timezone) and RFC3339Nano formats
			lastCreatedAt, cursorErr = time.Parse(time.RFC3339, decoded.DBCursor)
			if cursorErr != nil {
				// Try RFC3339Nano as fallback
				lastCreatedAt, cursorErr = time.Parse(time.RFC3339Nano, decoded.DBCursor)
			}
		}
	}
	if cursorErr != nil {
		return nil, "", cursorErr
	}

	// Build query with cursor-based pagination
	query := `
		SELECT r.id, r.user_id, r.movie_id, r.rating, r.comment, r.created_at,
		       u.name, u.profile_url
		FROM user_reviews r
		JOIN users u ON r.user_id = u.id
		WHERE r.movie_id = $1`

	args := []interface{}{movieID}

	// Cursor-based pagination using created_at DESC
	if !lastCreatedAt.IsZero() {
		query += " AND r.created_at < $2"
		args = append(args, lastCreatedAt)
		query += " ORDER BY r.created_at DESC, r.id DESC LIMIT $3"
		args = append(args, limit)
	} else {
		query += " ORDER BY r.created_at DESC, r.id DESC LIMIT $2"
		args = append(args, limit)
	}

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, "", fmt.Errorf("failed to get reviews: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var review Review
		var authorName *string
		var authorPicture *string
		err := rows.Scan(&review.ID, &review.UserID, &review.MovieID, &review.Rating, &review.Comment, &review.CreatedAt, &authorName, &authorPicture)
		if err != nil {
			continue
		}
		review.TMDBID = review.MovieID
		if authorName != nil {
			review.AuthorName = *authorName
		}
		review.AuthorPicture = authorPicture
		reviews = append(reviews, review)
	}

	if len(reviews) == limit {
		lastReview := reviews[len(reviews)-1]
		cursor := &Cursor{Source: "db", DBCursor: lastReview.CreatedAt.Format(time.RFC3339)}
		nextCursor = EncodeCursor(cursor)
	}

	return reviews, nextCursor, nil
}

// GetByID retrieves a single review by ID
func (r *Repository) GetByID(ctx context.Context, reviewID int64) (*Review, error) {
	var review Review
	err := r.db.QueryRow(ctx, `
		SELECT r.id, r.user_id, r.movie_id, r.rating, r.comment, r.created_at
		FROM user_reviews r
		WHERE r.id = $1
	`, reviewID).Scan(&review.ID, &review.UserID, &review.MovieID, &review.Rating, &review.Comment, &review.CreatedAt)

	if err == pgx.ErrNoRows {
		return nil, fmt.Errorf("review not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get review: %w", err)
	}

	review.TMDBID = review.MovieID
	return &review, nil
}

// GetTMDBReviews fetches reviews from TMDB for a movie
func (r *Repository) GetTMDBReviews(ctx context.Context, tmdbID int, page int) ([]TMDBReviewResponse, int, error) {
	result, err := r.tmdbClient.GetMovieReviews(tmdbID, page)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to get TMDB reviews: %w", err)
	}

	reviews := make([]TMDBReviewResponse, 0, len(result.Results))
	for _, r := range result.Results {
		reviews = append(reviews, TMDBReviewResponse{
			ID:         r.ID,
			Author:     r.AuthorDetails.Username,
			Content:    r.Content,
			CreatedAt:  r.CreatedAt,
			AvatarPath: r.AuthorDetails.AvatarPath,
			Rating:     r.AuthorDetails.Rating,
		})
	}

	return reviews, result.TotalPages, nil
}

// GetReviewCountForMovie returns the count of DB reviews for a movie
func (r *Repository) GetReviewCountForMovie(ctx context.Context, movieID int) (int, error) {
	var count int
	err := r.db.QueryRow(ctx, `
		SELECT COUNT(*) FROM user_reviews WHERE movie_id = $1
	`, movieID).Scan(&count)
	if err != nil {
		return 0, err
	}
	return count, nil
}