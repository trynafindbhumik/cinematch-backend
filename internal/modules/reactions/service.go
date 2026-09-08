package reactions

import (
	"context"
	"fmt"
	"strconv"

	"github.com/trynafindbhumik/cinematch-backend/internal/shared/logger"
	"github.com/trynafindbhumik/cinematch-backend/internal/shared/tmdb"
	"go.uber.org/zap"
)

type Service struct {
	repo       *Repository
	tmdbClient *tmdb.Client
}

func NewService(repo *Repository, tmdbClient *tmdb.Client) *Service {
	return &Service{
		repo:       repo,
		tmdbClient: tmdbClient,
	}
}

func (s *Service) AddReaction(ctx context.Context, userID int64, tmdbID int, reaction string) error {
	log := logger.WithUserID(userID).With(zap.String("operation", "AddReaction"))

	movieID, err := s.repo.GetMovieIDByTMDBID(ctx, tmdbID)
	if err != nil {
		// Movie not in local DB - try fetching from TMDB and upserting
		if s.tmdbClient != nil {
			details, fetchErr := s.tmdbClient.GetMovieDetails(tmdbID)
			if fetchErr == nil {
				genres := make([]string, 0, len(details.Genres))
				for _, g := range details.Genres {
					genres = append(genres, g.Name)
				}
				year := 0
				if len(details.ReleaseDate) >= 4 {
					year, _ = strconv.Atoi(details.ReleaseDate[:4])
				}
				rating := int(details.VoteAverage * 10)
				upsertedID, upsertErr := s.repo.UpsertMovie(ctx, tmdbID, details.Title, tmdb.PosterURL(details.PosterPath), tmdb.BackdropURL(details.BackdropPath), year, rating, genres)
				if upsertErr == nil {
					movieID = upsertedID
					err = nil
				}
			}
		}
		if err != nil {
			log.Error("movie not found", logger.Err(err), logger.Int("tmdb_id", tmdbID))
			return fmt.Errorf("movie not found: %w", err)
		}
	}

	previousReaction, err := s.repo.GetPreviousReaction(ctx, userID, movieID)
	if err != nil {
		log.Warn("failed to get previous reaction, assuming none", logger.Err(err))
	}

	if err := s.repo.AddReaction(ctx, userID, movieID, reaction); err != nil {
		log.Error("failed to add reaction", logger.Err(err))
		return fmt.Errorf("failed to add reaction: %w", err)
	}

	if err := s.repo.UpdateMovieReactionCounts(ctx, movieID, previousReaction, reaction); err != nil {
		log.Error("failed to update movie reaction counts", logger.Err(err))
		return fmt.Errorf("failed to update movie reaction counts: %w", err)
	}

	if err := s.repo.RemoveFromDailyGenerationLog(ctx, userID, tmdbID); err != nil {
		log.Error("failed to remove tmdbid from generation log", logger.Err(err), logger.Int("tmdb_id", tmdbID))
	} else {
		log.Info("removed tmdbid from generation log", logger.Int("tmdb_id", tmdbID))
	}

	return nil
}

func (s *Service) RemoveReaction(ctx context.Context, userID int64, tmdbID int) error {
	log := logger.WithUserID(userID).With(zap.String("operation", "RemoveReaction"))

	movieID, err := s.repo.GetMovieIDByTMDBID(ctx, tmdbID)
	if err != nil {
		log.Error("movie not found", logger.Err(err), logger.Int("tmdb_id", tmdbID))
		return fmt.Errorf("movie not found: %w", err)
	}

	previousReaction, err := s.repo.GetPreviousReaction(ctx, userID, movieID)
	if err != nil {
		log.Warn("failed to get previous reaction", logger.Err(err))
	}

	if err := s.repo.RemoveReaction(ctx, userID, movieID); err != nil {
		log.Error("failed to remove reaction", logger.Err(err))
		return fmt.Errorf("failed to remove reaction: %w", err)
	}

	if previousReaction != "" {
		if err := s.repo.UpdateMovieReactionCounts(ctx, movieID, previousReaction, ""); err != nil {
			log.Error("failed to update movie reaction counts", logger.Err(err))
		}
	}

	return nil
}
