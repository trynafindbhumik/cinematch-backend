package genres

// Genre module handling movie genres for user preferences.

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/trynafindbhumik/cinematch-backend/internal/shared/middleware"
)

// Handler handles genre HTTP endpoints
type Handler struct {
	service *Service
}

// NewHandler creates a new genres handler
func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// GetAllGenres returns all available genres
//
//	@Summary		Get all genres
//	@Description	Returns all available movie genres
//	@Tags			genres
//	@Produce		json
//	@Security		BearerAuth
//	@Success		200	{object}	GenresListResponse
//	@Failure		401	{object}	ErrorResponse
//	@Failure		500	{object}	ErrorResponse
//	@Router			/v1/genres [get]
func (h *Handler) GetAllGenres(c *gin.Context) {
	userID := middleware.GetUserID(c)
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, ErrorResponse{Error: "unauthorized"})
		return
	}

	result, err := h.service.GetAllGenres(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "fetch_failed"})
		return
	}

	c.JSON(http.StatusOK, result)
}

// GetUserGenres returns user's preferred genres
//
//	@Summary		Get user's preferred genres
//	@Description	Returns genres selected by the current user
//	@Tags			genres
//	@Produce		json
//	@Security		BearerAuth
//	@Success		200	{object}	UserGenresListResponse
//	@Failure		401	{object}	ErrorResponse
//	@Failure		500	{object}	ErrorResponse
//	@Router			/v1/genres/mine [get]
func (h *Handler) GetUserGenres(c *gin.Context) {
	userID := middleware.GetUserID(c)
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, ErrorResponse{Error: "unauthorized"})
		return
	}

	result, err := h.service.GetUserGenres(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "fetch_failed"})
		return
	}

	c.JSON(http.StatusOK, result)
}

// AddUserGenre adds a single genre to user's preferences
//
//	@Summary		Add a genre to user's preferences
//	@Description	Adds a single genre to user's selected genres
//	@Tags			genres
//	@Produce		json
//	@Security		BearerAuth
//	@Param			genreId	path		int	true	"Genre ID"
//	@Success		200		{object}	map[string]string
//	@Failure		400		{object}	ErrorResponse
//	@Failure		401		{object}	ErrorResponse
//	@Failure		500		{object}	ErrorResponse
//	@Router			/v1/genres/{genreId} [post]
func (h *Handler) AddUserGenre(c *gin.Context) {
	userID := middleware.GetUserID(c)
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, ErrorResponse{Error: "unauthorized"})
		return
	}

	genreIDStr := c.Param("genreId")
	var genreID int16
	if _, err := fmt.Sscanf(genreIDStr, "%d", &genreID); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid_request"})
		return
	}

	err := h.service.AddUserGenre(c.Request.Context(), userID, genreID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "add_failed"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Genre added successfully"})
}

// RemoveUserGenre removes a single genre from user's preferences
//
//	@Summary		Remove a genre from user's preferences
//	@Description	Removes a single genre from user's selected genres
//	@Tags			genres
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			genreId	path		int	true	"Genre ID"
//	@Success		200		{object}	map[string]string
//	@Failure		400		{object}	ErrorResponse
//	@Failure		401		{object}	ErrorResponse
//	@Failure		500		{object}	ErrorResponse
//	@Router			/v1/genres/{genreId} [delete]
func (h *Handler) RemoveUserGenre(c *gin.Context) {
	userID := middleware.GetUserID(c)
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, ErrorResponse{Error: "unauthorized"})
		return
	}

	genreIDStr := c.Param("genreId")
	var genreID int16
	if _, err := fmt.Sscanf(genreIDStr, "%d", &genreID); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid_request"})
		return
	}

	err := h.service.RemoveUserGenre(c.Request.Context(), userID, genreID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "remove_failed"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Genre removed successfully"})
}
