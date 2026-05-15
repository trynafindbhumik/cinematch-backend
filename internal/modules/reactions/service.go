package reactions

import (
	"context"
	"fmt"

	"github.com/trynafindbhumik/cinematch-backend/internal/shared/logger"
	"go.uber.org/zap"
)

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) AddReaction(ctx context.Context, userID int64, tmdbID int, reaction string) error {
	log := logger.WithUserID(userID).With(zap.String("operation", "AddReaction"))

	movieID, err := s.repo.GetMovieIDByTMDBID(ctx, tmdbID)
	if err != nil {
		log.Error("movie not found", logger.Err(err), logger.Int("tmdb_id", tmdbID))
		return fmt.Errorf("movie not found: %w", err)
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
