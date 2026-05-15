package export

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/trynafindbhumik/cinematch-backend/internal/shared/email"
)

// Service handles export business logic
type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

// ExportData generates a ZIP containing CSV files based on the request and sends via email
func (s *Service) ExportData(ctx context.Context, userID int64, req *ExportRequest) error {
	// Get user email for sending
	profile, err := s.repo.GetUserProfileInfo(ctx, userID)
	if err != nil {
		return fmt.Errorf("failed to get profile: %w", err)
	}

	// Create ZIP buffer
	buf := new(bytes.Buffer)
	zipWriter := zip.NewWriter(buf)

	// Generate CSV files based on request
	if req.ProfileInfo {
		if err := s.addProfileInfoCSV(ctx, zipWriter, userID); err != nil {
			return fmt.Errorf("failed to add profile info: %w", err)
		}
	}

	if req.Preferences {
		if err := s.addPreferencesCSV(ctx, zipWriter, userID); err != nil {
			return fmt.Errorf("failed to add preferences: %w", err)
		}
	}

	if req.Watchlist {
		if err := s.addWatchlistCSV(ctx, zipWriter, userID); err != nil {
			return fmt.Errorf("failed to add watchlist: %w", err)
		}
	}

	if req.Watched {
		if err := s.addWatchedCSV(ctx, zipWriter, userID); err != nil {
			return fmt.Errorf("failed to add watched: %w", err)
		}
	}

	if req.Favorites {
		if err := s.addFavoritesCSV(ctx, zipWriter, userID); err != nil {
			return fmt.Errorf("failed to add favorites: %w", err)
		}
	}

	if req.Reviews {
		if err := s.addReviewsCSV(ctx, zipWriter, userID); err != nil {
			return fmt.Errorf("failed to add reviews: %w", err)
		}
	}

	// Close zip writer
	if err := zipWriter.Close(); err != nil {
		return fmt.Errorf("failed to close zip: %w", err)
	}

	// Send email with ZIP attachment
	emailBody := fmt.Sprintf(`
		<h1>Your CineMatch Data Export</h1>
		<p>Hello %s,</p>
		<p>Your data export is attached as a ZIP file containing CSV files.</p>
		<p>If you did not request this export, please contact our support team immediately.</p>
		<p>This email was sent to: %s</p>
		<p>- The CineMatch Team</p>
	`, profile.Name, profile.Email)

	email.SendEmailAsync(email.EmailData{
		To:      profile.Email,
		Subject: "Your CineMatch Data Export",
		Body:    emailBody,
		Attachment: &email.Attachment{
			Filename: "cinematch_data_export.zip",
			MimeType: "application/zip",
			Data:     buf.Bytes(),
		},
	})

	return nil
}

func (s *Service) addProfileInfoCSV(ctx context.Context, zipWriter *zip.Writer, userID int64) error {
	profile, err := s.repo.GetUserProfileInfo(ctx, userID)
	if err != nil {
		return err
	}

	var buf bytes.Buffer
	buf.WriteString("email,name,tag,is_verified,created_at\n")
	buf.WriteString(fmt.Sprintf("%s,%s,%s,%t,%s\n",
		escapeCSV(profile.Email),
		escapeCSV(profile.Name),
		profile.Tag,
		profile.IsVerified,
		profile.CreatedAt.Format(time.RFC3339),
	))

	return s.addFileToZip(zipWriter, "profile_info.csv", buf.Bytes())
}

func (s *Service) addPreferencesCSV(ctx context.Context, zipWriter *zip.Writer, userID int64) error {
	prefs, err := s.repo.GetUserPreferences(ctx, userID)
	if err != nil {
		return err
	}

	var buf bytes.Buffer
	buf.WriteString("preference_type,value\n")
	for _, p := range prefs {
		buf.WriteString(fmt.Sprintf("%s,%s\n",
			escapeCSV(p.PreferenceType),
			escapeCSV(p.Value),
		))
	}

	return s.addFileToZip(zipWriter, "preferences.csv", buf.Bytes())
}

func (s *Service) addWatchlistCSV(ctx context.Context, zipWriter *zip.Writer, userID int64) error {
	movies, err := s.repo.GetWatchlist(ctx, userID)
	if err != nil {
		return err
	}

	var buf bytes.Buffer
	buf.WriteString("movie_title,tmdb_id,poster_url,release_year,genres,added_at\n")
	for _, m := range movies {
		buf.WriteString(fmt.Sprintf("%s,%d,%s,%d,%s,%s\n",
			escapeCSV(m.MovieTitle),
			m.TMDBID,
			escapeCSV(m.PosterURL),
			m.ReleaseYear,
			escapeCSV(m.Genres),
			m.AddedAt.Format(time.RFC3339),
		))
	}

	return s.addFileToZip(zipWriter, "watchlist.csv", buf.Bytes())
}

func (s *Service) addWatchedCSV(ctx context.Context, zipWriter *zip.Writer, userID int64) error {
	movies, err := s.repo.GetWatched(ctx, userID)
	if err != nil {
		return err
	}

	var buf bytes.Buffer
	buf.WriteString("movie_title,tmdb_id,poster_url,release_year,genres,is_favorite,added_at\n")
	for _, m := range movies {
		buf.WriteString(fmt.Sprintf("%s,%d,%s,%d,%s,%t,%s\n",
			escapeCSV(m.MovieTitle),
			m.TMDBID,
			escapeCSV(m.PosterURL),
			m.ReleaseYear,
			escapeCSV(m.Genres),
			m.IsFavorite,
			m.AddedAt.Format(time.RFC3339),
		))
	}

	return s.addFileToZip(zipWriter, "watched.csv", buf.Bytes())
}

func (s *Service) addFavoritesCSV(ctx context.Context, zipWriter *zip.Writer, userID int64) error {
	movies, err := s.repo.GetFavorites(ctx, userID)
	if err != nil {
		return err
	}

	var buf bytes.Buffer
	buf.WriteString("movie_title,tmdb_id,poster_url,release_year,genres,added_at\n")
	for _, m := range movies {
		buf.WriteString(fmt.Sprintf("%s,%d,%s,%d,%s,%s\n",
			escapeCSV(m.MovieTitle),
			m.TMDBID,
			escapeCSV(m.PosterURL),
			m.ReleaseYear,
			escapeCSV(m.Genres),
			m.AddedAt.Format(time.RFC3339),
		))
	}

	return s.addFileToZip(zipWriter, "favorites.csv", buf.Bytes())
}

func (s *Service) addReviewsCSV(ctx context.Context, zipWriter *zip.Writer, userID int64) error {
	reviews, err := s.repo.GetReviews(ctx, userID)
	if err != nil {
		return err
	}

	var buf bytes.Buffer
	buf.WriteString("movie_title,tmdb_id,rating,comment,created_at\n")
	for _, r := range reviews {
		buf.WriteString(fmt.Sprintf("%s,%d,%d,%s,%s\n",
			escapeCSV(r.MovieTitle),
			r.TMDBID,
			r.Rating,
			escapeCSV(r.Comment),
			r.CreatedAt.Format(time.RFC3339),
		))
	}

	return s.addFileToZip(zipWriter, "reviews.csv", buf.Bytes())
}

func (s *Service) addFileToZip(zipWriter *zip.Writer, filename string, data []byte) error {
	f, err := zipWriter.Create(filename)
	if err != nil {
		return err
	}
	_, err = f.Write(data)
	return err
}

// escapeCSV escapes a string for CSV format
func escapeCSV(s string) string {
	if strings.ContainsAny(s, ",\"\n\r") {
		return "\"" + strings.ReplaceAll(s, "\"", "\"\"") + "\""
	}
	return s
}
