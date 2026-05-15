package streaming_services

import (
	"context"
	"errors"
	"strings"

	"github.com/trynafindbhumik/cinematch-backend/internal/db"
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

// GetAllStreamingServices returns all available streaming services with pagination
func (s *Service) GetAllStreamingServices(ctx context.Context, cursor string, limit int) ([]StreamingServiceResponse, string, int, error) {
	services, nextCursor, totalCount, err := s.repo.GetAllStreamingServices(ctx, cursor, limit)
	if err != nil {
		return nil, "", 0, err
	}

	response := make([]StreamingServiceResponse, len(services))
	for i, svc := range services {
		iconURL := ""
		if svc.IconURL != nil {
			iconURL = *svc.IconURL
		}
		response[i] = StreamingServiceResponse{
			ID:      svc.ID,
			Name:    svc.Name,
			IconURL: iconURL,
		}
	}

	return response, nextCursor, totalCount, nil
}

// SearchStreamingServices searches streaming services by name with pagination
func (s *Service) SearchStreamingServices(ctx context.Context, query string, cursor string, limit int) ([]StreamingServiceResponse, string, int, error) {
	if strings.TrimSpace(query) == "" {
		return []StreamingServiceResponse{}, "", 0, nil
	}

	services, nextCursor, totalCount, err := s.repo.SearchStreamingServices(ctx, query, cursor, limit)
	if err != nil {
		return nil, "", 0, err
	}

	response := make([]StreamingServiceResponse, len(services))
	for i, svc := range services {
		iconURL := ""
		if svc.IconURL != nil {
			iconURL = *svc.IconURL
		}
		response[i] = StreamingServiceResponse{
			ID:      svc.ID,
			Name:    svc.Name,
			IconURL: iconURL,
		}
	}

	return response, nextCursor, totalCount, nil
}

// GetUserStreamingServices returns streaming services selected by the user
func (s *Service) GetUserStreamingServices(ctx context.Context, userID int64) ([]UserStreamingServiceResponse, error) {
	userServices, err := s.repo.GetUserStreamingServices(ctx, userID)
	if err != nil {
		return nil, err
	}

	response := make([]UserStreamingServiceResponse, len(userServices))
	for i, us := range userServices {
		iconURL := ""
		if us.IconURL != nil {
			iconURL = *us.IconURL
		}
		response[i] = UserStreamingServiceResponse{
			ID:       us.ServiceID,
			Name:     us.SourceName,
			IconURL:  iconURL,
			SourceID: us.ServiceID,
		}
	}

	return response, nil
}

// UpdateUserStreamingServices replaces user's streaming services with new list
func (s *Service) UpdateUserStreamingServices(ctx context.Context, userID int64, serviceIDs []int16) error {
	if len(serviceIDs) == 0 {
		// Clear all user streaming services
		_, err := db.Pool().Exec(ctx, `DELETE FROM user_streaming_services WHERE user_id = $1`, userID)
		return err
	}

	// Validate all service IDs exist
	valid, err := s.repo.ValidateServiceIDs(ctx, serviceIDs)
	if err != nil {
		return err
	}
	if !valid {
		return ErrInvalidInput
	}

	return s.repo.UpdateUserStreamingServices(ctx, userID, serviceIDs)
}

// RemoveUserStreamingServices removes multiple streaming services from user's preferences
func (s *Service) RemoveUserStreamingServices(ctx context.Context, userID int64, serviceIDs []int16) error {
	return s.repo.RemoveUserStreamingServices(ctx, userID, serviceIDs)
}

// RemoveUserStreamingService removes a single streaming service from user's preferences
func (s *Service) RemoveUserStreamingService(ctx context.Context, userID int64, serviceID int16) error {
	return s.repo.RemoveUserStreamingService(ctx, userID, serviceID)
}
