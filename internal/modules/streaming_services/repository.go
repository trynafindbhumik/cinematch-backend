package streaming_services

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/trynafindbhumik/cinematch-backend/internal/db"
)

var (
	ErrStreamingServiceNotFound      = errors.New("streaming service not found")
	ErrStreamingServiceAlreadyExists = errors.New("streaming service already exists")
	ErrInvalidStreamingServiceID     = errors.New("invalid streaming service ID")
)

type Repository struct{}

func NewRepository() *Repository {
	return &Repository{}
}

// GetAllStreamingServices retrieves all streaming services from the master table (tmdb_id excluded)
func (r *Repository) GetAllStreamingServices(ctx context.Context, cursor string, limit int) ([]StreamingService, string, int, error) {
	var totalCount int
	err := db.Pool().QueryRow(ctx, `SELECT COUNT(*) FROM streaming_services`).Scan(&totalCount)
	if err != nil {
		return nil, "", 0, err
	}

	var query string
	var args []interface{}

	cursorID, _ := strconv.ParseInt(cursor, 10, 16)

	if cursor == "" || cursorID <= 0 {
		query = `
			SELECT id, name, icon_url
			FROM streaming_services
			ORDER BY id ASC
			LIMIT $1
		`
		args = []interface{}{limit + 1} // Fetch one extra to check if there's a next page
	} else {
		query = `
			SELECT id, name, icon_url
			FROM streaming_services
			WHERE id > $1
			ORDER BY id ASC
			LIMIT $2
		`
		args = []interface{}{cursorID, limit + 1}
	}

	rows, err := db.Pool().Query(ctx, query, args...)
	if err != nil {
		return nil, "", 0, err
	}
	defer rows.Close()

	var services []StreamingService
	for rows.Next() {
		var s StreamingService
		if err := rows.Scan(&s.ID, &s.Name, &s.IconURL); err != nil {
			return nil, "", 0, err
		}
		services = append(services, s)
	}

	nextCursor := ""
	if len(services) > limit {
		services = services[:limit]
		nextCursor = fmt.Sprintf("%d", services[limit-1].ID)
	}

	return services, nextCursor, totalCount, rows.Err()
}

// SearchStreamingServices searches streaming services by name
func (r *Repository) SearchStreamingServices(ctx context.Context, query string, cursor string, limit int) ([]StreamingService, string, int, error) {
	searchQuery := "%" + strings.ToLower(query) + "%"

	var totalCount int
	countQuery := `SELECT COUNT(*) FROM streaming_services WHERE LOWER(name) LIKE $1`
	if err := db.Pool().QueryRow(ctx, countQuery, searchQuery).Scan(&totalCount); err != nil {
		return nil, "", 0, err
	}

	var q string
	var args []interface{}

	cursorID, _ := strconv.ParseInt(cursor, 10, 16)

	if cursor == "" || cursorID <= 0 {
		q = `
			SELECT id, name, icon_url
			FROM streaming_services
			WHERE LOWER(name) LIKE $1
			ORDER BY name ASC
			LIMIT $2
		`
		args = []interface{}{searchQuery, limit + 1}
	} else {
		q = `
			SELECT id, name, icon_url
			FROM streaming_services
			WHERE LOWER(name) LIKE $1 AND name > (
				SELECT name FROM streaming_services WHERE id = $2
			)
			ORDER BY name ASC
			LIMIT $3
		`
		args = []interface{}{searchQuery, cursorID, limit + 1}
	}

	rows, err := db.Pool().Query(ctx, q, args...)
	if err != nil {
		return nil, "", 0, err
	}
	defer rows.Close()

	var services []StreamingService
	for rows.Next() {
		var s StreamingService
		if err := rows.Scan(&s.ID, &s.Name, &s.IconURL); err != nil {
			return nil, "", 0, err
		}
		services = append(services, s)
	}

	nextCursor := ""
	if len(services) > limit {
		services = services[:limit]
		nextCursor = fmt.Sprintf("%d", services[limit-1].ID)
	}

	return services, nextCursor, totalCount, rows.Err()
}

// GetUserStreamingServices retrieves all streaming services selected by a user
func (r *Repository) GetUserStreamingServices(ctx context.Context, userID int64) ([]UserStreamingService, error) {
	rows, err := db.Pool().Query(ctx, `
		SELECT uss.user_id, uss.service_id, ss.name, ss.icon_url
		FROM user_streaming_services uss
		JOIN streaming_services ss ON ss.id = uss.service_id
		WHERE uss.user_id = $1
		ORDER BY ss.name ASC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var userServices []UserStreamingService
	for rows.Next() {
		var us UserStreamingService
		if err := rows.Scan(&us.UserID, &us.ServiceID, &us.SourceName, &us.IconURL); err != nil {
			return nil, err
		}
		userServices = append(userServices, us)
	}

	return userServices, rows.Err()
}

// ValidateServiceIDs checks if all provided service IDs exist
func (r *Repository) ValidateServiceIDs(ctx context.Context, serviceIDs []int16) (bool, error) {
	if len(serviceIDs) == 0 {
		return true, nil
	}

	row := db.Pool().QueryRow(ctx, `
		SELECT COUNT(*) FROM streaming_services WHERE id = ANY($1)
	`, serviceIDs)

	var count int
	if err := row.Scan(&count); err != nil {
		return false, err
	}

	return count == len(serviceIDs), nil
}

// AddUserStreamingService adds a single streaming service to a user's profile
func (r *Repository) AddUserStreamingService(ctx context.Context, userID int64, serviceID int16) error {
	_, err := db.Pool().Exec(ctx, `
		INSERT INTO user_streaming_services (user_id, service_id) VALUES ($1, $2)
		ON CONFLICT (user_id, service_id) DO NOTHING
	`, userID, serviceID)
	return err
}

// RemoveUserStreamingService removes a single streaming service from a user's profile
func (r *Repository) RemoveUserStreamingService(ctx context.Context, userID int64, serviceID int16) error {
	_, err := db.Pool().Exec(ctx, `
		DELETE FROM user_streaming_services WHERE user_id = $1 AND service_id = $2
	`, userID, serviceID)
	return err
}

// RemoveUserStreamingServices removes multiple streaming services from a user's profile
func (r *Repository) RemoveUserStreamingServices(ctx context.Context, userID int64, serviceIDs []int16) error {
	if len(serviceIDs) == 0 {
		return nil
	}

	_, err := db.Pool().Exec(ctx, `
		DELETE FROM user_streaming_services WHERE user_id = $1 AND service_id = ANY($2)
	`, userID, serviceIDs)
	return err
}

// UpdateUserStreamingServices replaces user's streaming services with new list
func (r *Repository) UpdateUserStreamingServices(ctx context.Context, userID int64, serviceIDs []int16) error {
	tx, err := db.Pool().Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	// Delete existing user streaming services
	_, err = tx.Exec(ctx, `DELETE FROM user_streaming_services WHERE user_id = $1`, userID)
	if err != nil {
		return err
	}

	// Insert new user streaming services
	for _, serviceID := range serviceIDs {
		_, err = tx.Exec(ctx, `
			INSERT INTO user_streaming_services (user_id, service_id) VALUES ($1, $2)
		`, userID, serviceID)
		if err != nil {
			return err
		}
	}

	return tx.Commit(ctx)
}
