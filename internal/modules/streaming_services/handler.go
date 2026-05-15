package streaming_services

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/trynafindbhumik/cinematch-backend/internal/shared/middleware"
)

// Handler handles HTTP requests for streaming services
type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// GetAllStreamingServices handles GET /v1/streaming-services
//
//	@Summary		Get all streaming services
//	@Description	Returns paginated list of streaming services from the master table
//	@Tags			streaming-services
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			cursor	query	string	false	"Cursor for pagination"
//	@Param			limit	query	int		false	"Number of items"	default(20)	maximum(100)
//	@Success		200	{object}	StreamingServicesListResponse
//	@Failure		401	{object}	ErrorResponse	"Unauthorized"
//	@Failure		500	{object}	ErrorResponse	"Server error"
//	@Router			/v1/streaming-services [get]
func (h *Handler) GetAllStreamingServices(c *gin.Context) {
	cursor := c.Query("cursor")
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	if limit < 1 || limit > 100 {
		limit = 20
	}

	services, nextCursor, totalCount, err := h.service.GetAllStreamingServices(c.Request.Context(), cursor, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "failed to fetch streaming services"})
		return
	}

	c.JSON(http.StatusOK, StreamingServicesListResponse{
		StreamingServices: services,
		NextCursor:        nextCursor,
		Limit:             limit,
		TotalCount:        totalCount,
	})
}

// SearchStreamingServices handles GET /v1/streaming-services/search?q={query}
//
//	@Summary		Search streaming services
//	@Description	Searches streaming services by name with pagination
//	@Tags			streaming-services
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			q		query	string	true	"Search query"
//	@Param			cursor	query	string	false	"Cursor for pagination"
//	@Param			limit	query	int		false	"Number of items"	default(20)	maximum(100)
//	@Success		200	{object}	SearchStreamingServicesResponse
//	@Failure		400	{object}	ErrorResponse	"Bad request"
//	@Failure		401	{object}	ErrorResponse	"Unauthorized"
//	@Failure		500	{object}	ErrorResponse	"Server error"
//	@Router			/v1/streaming-services/search [get]
func (h *Handler) SearchStreamingServices(c *gin.Context) {
	query := c.Query("q")
	if query == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "query parameter 'q' is required"})
		return
	}

	cursor := c.Query("cursor")
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	if limit < 1 || limit > 100 {
		limit = 20
	}

	services, nextCursor, totalCount, err := h.service.SearchStreamingServices(c.Request.Context(), query, cursor, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "failed to search streaming services"})
		return
	}

	c.JSON(http.StatusOK, SearchStreamingServicesResponse{
		StreamingServices: services,
		NextCursor:         nextCursor,
		Limit:              limit,
		TotalCount:         totalCount,
	})
}

// GetUserStreamingServices handles GET /v1/streaming-services/mine
//
//	@Summary		Get user's selected streaming services
//	@Description	Returns the streaming services selected by the authenticated user
//	@Tags			streaming-services
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Success		200	{object}	UserStreamingServicesListResponse
//	@Failure		401	{object}	ErrorResponse	"Unauthorized"
//	@Failure		500	{object}	ErrorResponse	"Server error"
//	@Router			/v1/streaming-services/mine [get]
func (h *Handler) GetUserStreamingServices(c *gin.Context) {
	userID := middleware.GetUserID(c)
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, ErrorResponse{Error: "unauthorized"})
		return
	}

	services, err := h.service.GetUserStreamingServices(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "failed to fetch user streaming services"})
		return
	}

	c.JSON(http.StatusOK, UserStreamingServicesListResponse{StreamingServices: services})
}

// UpdateUserStreamingServices handles PUT /v1/streaming-services
//
//	@Summary		Update user's streaming services (bulk)
//	@Description	Replaces user's streaming services with the provided list
//	@Tags			streaming-services
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			request	body		UpdateStreamingServicesRequest	true	"List of streaming service IDs"
//	@Success		200		{object}	map[string]string
//	@Failure		400		{object}	ErrorResponse	"Bad request"
//	@Failure		401		{object}	ErrorResponse	"Unauthorized"
//	@Failure		500		{object}	ErrorResponse	"Server error"
//	@Router			/v1/streaming-services [put]
func (h *Handler) UpdateUserStreamingServices(c *gin.Context) {
	userID := middleware.GetUserID(c)
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, ErrorResponse{Error: "unauthorized"})
		return
	}

	var req UpdateStreamingServicesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid_request"})
		return
	}

	if err := h.service.UpdateUserStreamingServices(c.Request.Context(), userID, req.ServiceIDs); err != nil {
		if errors.Is(err, ErrInvalidInput) {
			c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid_streaming_service_id"})
			return
		}
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "update_failed"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Streaming services updated successfully"})
}

// RemoveUserStreamingServicesBulk handles DELETE /v1/streaming-services/bulk
//
//	@Summary		Remove multiple streaming services from user's profile
//	@Description	Removes multiple streaming services from user's selected services
//	@Tags			streaming-services
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			request	body		UpdateStreamingServicesRequest	true	"List of streaming service IDs to remove"
//	@Success		200		{object}	map[string]string
//	@Failure		400		{object}	ErrorResponse	"Bad request"
//	@Failure		401		{object}	ErrorResponse	"Unauthorized"
//	@Failure		500		{object}	ErrorResponse	"Server error"
//	@Router			/v1/streaming-services/bulk [delete]
func (h *Handler) RemoveUserStreamingServicesBulk(c *gin.Context) {
	userID := middleware.GetUserID(c)
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, ErrorResponse{Error: "unauthorized"})
		return
	}

	var req UpdateStreamingServicesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid_request"})
		return
	}

	if len(req.ServiceIDs) == 0 {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "service_ids_required"})
		return
	}

	if err := h.service.RemoveUserStreamingServices(c.Request.Context(), userID, req.ServiceIDs); err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "remove_failed"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Streaming services removed successfully"})
}

// RemoveUserStreamingService handles DELETE /v1/streaming-services/{serviceId}
//
//	@Summary		Remove a streaming service from user's profile
//	@Description	Removes a single streaming service from user's selected services
//	@Tags			streaming-services
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			serviceId	path		int	true	"Streaming service ID to remove"
//	@Success		200			{object}	map[string]string
//	@Failure		400			{object}	ErrorResponse	"Bad request"
//	@Failure		401			{object}	ErrorResponse	"Unauthorized"
//	@Failure		500			{object}	ErrorResponse	"Server error"
//	@Router			/v1/streaming-services/{serviceId} [delete]
func (h *Handler) RemoveUserStreamingService(c *gin.Context) {
	userID := middleware.GetUserID(c)
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, ErrorResponse{Error: "unauthorized"})
		return
	}

	serviceIDStr := c.Param("serviceId")
	serviceID, err := strconv.ParseInt(serviceIDStr, 10, 16)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid_service_id"})
		return
	}

	if err := h.service.RemoveUserStreamingService(c.Request.Context(), userID, int16(serviceID)); err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "remove_failed"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Streaming service removed successfully"})
}
