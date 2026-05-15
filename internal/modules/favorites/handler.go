package favorites

// Favorites module for managing user's favorite movies.

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/trynafindbhumik/cinematch-backend/internal/shared/middleware"
)

// Response types for favorites module
type GetFavoritesResponse struct {
	Favorites  []FavoriteMovie `json:"favorites"`
	NextCursor string          `json:"next_cursor,omitempty"`
	Limit      int             `json:"limit"`
	TotalCount int             `json:"total_count"`
}

type GetFavoriteIDsResponse struct {
	TMDBIDs []int `json:"tmdb_ids"`
}

type AddFavoritesRequest struct {
	TMDBIDs []int `json:"tmdb_ids" binding:"required,min=1"`
}

type AddFavoritesResponse struct {
	Message   string         `json:"message"`
	Added     []int          `json:"added"`
	Failed    map[int]string `json:"failed,omitempty"`
	TotalAdd  int            `json:"total_added"`
	TotalFail int            `json:"total_failed"`
}

type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message,omitempty"`
}

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// GetFavorites handles GET /v1/favorites
//
//	@Summary		Get user's favorites
//	@Description	Returns paginated list of user's favorite movies
//	@Tags			favorites
//	@Produce		json
//	@Param			cursor	query	string	false	"Cursor for pagination"
//	@Param			limit	query	int		false	"Number of items"	default(20)
//	@Param			genre	query	string	false	"Filter by genre"
//	@Security		BearerAuth
//	@Success		200	{object}	GetFavoritesResponse
//	@Failure		401	{object}	ErrorResponse
//	@Failure		500	{object}	ErrorResponse
//	@Router			/v1/favorites [get]
func (h *Handler) GetFavorites(c *gin.Context) {
	userID := middleware.GetUserID(c)
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, ErrorResponse{Error: "unauthorized"})
		return
	}

	cursor := c.Query("cursor")
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	if limit < 1 || limit > 100 {
		limit = 20
	}
	genre := c.Query("genre")

	result, err := h.service.GetFavorites(c.Request.Context(), userID, cursor, limit, genre)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "fetch_failed"})
		return
	}

	c.JSON(http.StatusOK, result)
}

// GetFavoriteIDs handles GET /v1/favorites/ids
//
//	@Summary		Get all favorite TMDB IDs
//	@Description	Returns all TMDB IDs that user has favorited
//	@Tags			favorites
//	@Produce		json
//	@Security		BearerAuth
//	@Success		200	{object}	GetFavoriteIDsResponse
//	@Failure		401	{object}	ErrorResponse
//	@Failure		500	{object}	ErrorResponse
//	@Router			/v1/favorites/ids [get]
func (h *Handler) GetFavoriteIDs(c *gin.Context) {
	userID := middleware.GetUserID(c)
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, ErrorResponse{Error: "unauthorized"})
		return
	}

	result, err := h.service.GetFavoriteIDs(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "fetch_failed"})
		return
	}

	c.JSON(http.StatusOK, result)
}

// SearchFavorites handles GET /v1/favorites/search
//
//	@Summary		Search within favorites
//	@Description	Searches within user's favorite movies
//	@Tags			favorites
//	@Produce		json
//	@Param			q		query	string	true	"Search query"
//	@Param			cursor	query	string	false	"Cursor for pagination"
//	@Param			limit	query	int		false	"Items per page"	default(20)
//	@Security		BearerAuth
//	@Success		200	{object}	GetFavoritesResponse
//	@Failure		400	{object}	ErrorResponse
//	@Failure		401	{object}	ErrorResponse
//	@Failure		500	{object}	ErrorResponse
//	@Router			/v1/favorites/search [get]
func (h *Handler) SearchFavorites(c *gin.Context) {
	userID := middleware.GetUserID(c)
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, ErrorResponse{Error: "unauthorized"})
		return
	}

	query := c.Query("q")
	if query == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "missing_query"})
		return
	}

	cursor := c.Query("cursor")

	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	if limit < 1 || limit > 100 {
		limit = 20
	}

	result, err := h.service.SearchFavorites(c.Request.Context(), userID, query, cursor, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "search_failed"})
		return
	}

	c.JSON(http.StatusOK, result)
}

// AddFavorites handles POST /v1/favorites
//
//	@Summary		Add movies to favorites
//	@Description	Adds movies to user's favorites
//	@Tags			favorites
//	@Accept			json
//	@Produce		json
//	@Param			request	body	AddFavoritesRequest	true	"TMDB IDs to add"
//	@Security		BearerAuth
//	@Success		200	{object}	AddFavoritesResponse
//	@Failure		400	{object}	ErrorResponse
//	@Failure		401	{object}	ErrorResponse
//	@Failure		500	{object}	ErrorResponse
//	@Router			/v1/favorites [post]
func (h *Handler) AddFavorites(c *gin.Context) {
	userID := middleware.GetUserID(c)
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, ErrorResponse{Error: "unauthorized"})
		return
	}

	var req AddFavoritesRequest
	if err := c.ShouldBindJSON(&req); err != nil || len(req.TMDBIDs) == 0 {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid_request"})
		return
	}

	result, err := h.service.AddFavorites(c.Request.Context(), userID, req.TMDBIDs)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "add_failed"})
		return
	}

	c.JSON(http.StatusOK, AddFavoritesResponse{
		Message:  "Movies added to favorites",
		Added:    result.Added,
		Failed:   result.Failed,
		TotalAdd: len(result.Added),
		TotalFail: len(result.Failed),
	})
}

// DeleteFavorite handles DELETE /v1/favorites/:id
//
//	@Summary		Remove movie from favorites
//	@Description	Removes a movie from user's favorites
//	@Tags			favorites
//	@Produce		json
//	@Param			id	path	int	true	"User Movies ID"
//	@Security		BearerAuth
//	@Success		200	{object}	map[string]string
//	@Failure		400	{object}	ErrorResponse
//	@Failure		401	{object}	ErrorResponse
//	@Failure		404	{object}	ErrorResponse
//	@Failure		500	{object}	ErrorResponse
//	@Router			/v1/favorites/{id} [delete]
func (h *Handler) DeleteFavorite(c *gin.Context) {
	userID := middleware.GetUserID(c)
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, ErrorResponse{Error: "unauthorized"})
		return
	}

	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid_id"})
		return
	}

	err = h.service.DeleteFavorite(c.Request.Context(), userID, id)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			c.JSON(http.StatusNotFound, ErrorResponse{Error: "not_found"})
			return
		}
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "delete_failed"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Movie removed from favorites"})
}
