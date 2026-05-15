package genres

import (
	"context"
	"errors"
)

var (
	ErrUserNotFound = errors.New("user not found")
	ErrInvalidInput = errors.New("invalid input")
)

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

// GetAllGenres returns all available genres
func (s *Service) GetAllGenres(ctx context.Context) ([]GenreResponse, error) {
	genres, err := s.repo.GetAllGenres(ctx)
	if err != nil {
		return nil, err
	}

	response := make([]GenreResponse, len(genres))
	for i, g := range genres {
		response[i] = GenreResponse{
			ID:   g.ID,
			Name: g.Name,
		}
	}

	return response, nil
}

// GetUserGenres returns genres selected by the user
func (s *Service) GetUserGenres(ctx context.Context, userID int64) ([]UserGenreResponse, error) {
	userGenres, err := s.repo.GetUserGenres(ctx, userID)
	if err != nil {
		return nil, err
	}

	response := make([]UserGenreResponse, len(userGenres))
	for i, ug := range userGenres {
		response[i] = UserGenreResponse{
			ID:      ug.GenreID,
			Name:    ug.GenreName,
			GenreID: ug.GenreID,
		}
	}

	return response, nil
}

// AddUserGenre adds a single genre to user's preferences
func (s *Service) AddUserGenre(ctx context.Context, userID int64, genreID int16) error {
	// Validate genre ID exists
	valid, err := s.repo.ValidateGenreIDs(ctx, []int16{genreID})
	if err != nil {
		return err
	}
	if !valid {
		return ErrInvalidInput
	}

	return s.repo.AddUserGenre(ctx, userID, genreID)
}

// RemoveUserGenre removes a single genre from user's preferences
func (s *Service) RemoveUserGenre(ctx context.Context, userID int64, genreID int16) error {
	return s.repo.RemoveUserGenre(ctx, userID, genreID)
}
