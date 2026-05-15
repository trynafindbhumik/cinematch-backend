package watched

// Watched module for managing user's watched movies.

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/trynafindbhumik/cinematch-backend/internal/shared/middleware"
)

// WatchedMovie represents a movie in user's watched list
type WatchedMovie struct {
	ID          int64    `json:"id"`
	MovieDBID   int      `json:"movie_db_id"`
	TMDBID      int      `json:"tmdb_id"`
	Title       string   `json:"title"`
	PosterURL   string   `json:"poster_url"`
	ReleaseYear int      `json:"release_year"`
	TMDBRating  int      `json:"tmdb_rating"`
	Genres      []string `json:"genres"`
	AddedAt     string   `json:"added_at"`
}

// GetWatchedResponse is the response for get watched
type GetWatchedResponse struct {
	Movies     []WatchedMovie `json:"movies"`
	NextCursor string         `json:"next_cursor,omitempty"`
	Limit      int            `json:"limit"`
	TotalCount int            `json:"total_count"`
}

// GetWatchedIDsResponse is the response for get watched IDs
type GetWatchedIDsResponse struct {
	TMDBIDs []int `json:"tmdb_ids"`
}

// AddToWatchedRequest is the request body for adding to watched
type AddToWatchedRequest struct {
	TMDBIDs []int `json:"tmdb_ids" binding:"required,min=1"`
}

// AddToWatchedResponse is the response for adding to watched
type AddToWatchedResponse struct {
	Message    string         `json:"message"`
	Added      []int          `json:"added"`
	Failed     map[int]string `json:"failed,omitempty"`
	TotalAdd   int            `json:"total_added"`
	TotalFail  int            `json:"total_failed"`
}

// ErrorResponse represents an API error
type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`
}

// Handler handles watched HTTP endpoints
type Handler struct {
	service *Service
}

// NewHandler creates a new watched handler
func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// GetWatched handles GET /v1/watched
//
//	@Summary		Get user's watched movies
//	@Description	Returns paginated list of user's watched movies
//	@Tags			watched
//	@Produce		json
//	@Param			cursor	query	string	false	"Cursor for pagination"
//	@Param			limit	query	int		false	"Number of items"	default(20)
//	@Param			genre	query	string	false	"Filter by genre"
//	@Security		BearerAuth
//	@Success		200	{object}	GetWatchedResponse
//	@Failure		401	{object}	ErrorResponse
//	@Failure		500	{object}	ErrorResponse
//	@Router			/v1/watched [get]
func (h *Handler) GetWatched(c *gin.Context) {
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

	result, err := h.service.GetWatched(c.Request.Context(), userID, cursor, limit, genre)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "fetch_failed"})
		return
	}

	c.JSON(http.StatusOK, result)
}

// GetWatchedIDs handles GET /v1/watched/ids
//
//	@Summary		Get all watched TMDB IDs
//	@Description	Returns all TMDB IDs that user has watched
//	@Tags			watched
//	@Produce		json
//	@Security		BearerAuth
//	@Success		200	{object}	GetWatchedIDsResponse
//	@Failure		401	{object}	ErrorResponse
//	@Failure		500	{object}	ErrorResponse
//	@Router			/v1/watched/ids [get]
func (h *Handler) GetWatchedIDs(c *gin.Context) {
	userID := middleware.GetUserID(c)
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, ErrorResponse{Error: "unauthorized"})
		return
	}

	result, err := h.service.GetWatchedIDs(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "fetch_failed"})
		return
	}

	c.JSON(http.StatusOK, result)
}

// SearchWatched handles GET /v1/watched/search
//
//	@Summary		Search user's watched list
//	@Description	Searches within user's watched list by title
//	@Tags			watched
//	@Produce		json
//	@Param			q		query	string	true	"Search query"
//	@Param			cursor	query	string	false	"Cursor for pagination"
//	@Param			limit	query	int		false	"Items per page"	default(20)
//	@Security		BearerAuth
//	@Success		200	{object}	GetWatchedResponse
//	@Failure		401	{object}	ErrorResponse
//	@Failure		500	{object}	ErrorResponse
//	@Router			/v1/watched/search [get]
func (h *Handler) SearchWatched(c *gin.Context) {
	userID := middleware.GetUserID(c)
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, ErrorResponse{Error: "unauthorized"})
		return
	}

	query := c.Query("q")
	if query == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid_request"})
		return
	}

	cursor := c.Query("cursor")

	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	if limit < 1 || limit > 100 {
		limit = 20
	}

	result, err := h.service.SearchWatched(c.Request.Context(), userID, query, cursor, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "search_failed"})
		return
	}

	c.JSON(http.StatusOK, result)
}

// AddToWatched handles POST /v1/watched
//
//	@Summary		Add movies to watched
//	@Description	Adds one or more movies to user's watched list
//	@Tags			watched
//	@Accept			json
//	@Produce		json
//	@Param			request	body	AddToWatchedRequest	true	"Movies to add"
//	@Security		BearerAuth
//	@Success		201	{object}	map[string]string
//	@Failure		400	{object}	ErrorResponse
//	@Failure		401	{object}	ErrorResponse
//	@Failure		500	{object}	ErrorResponse
//	@Router			/v1/watched [post]
func (h *Handler) AddToWatched(c *gin.Context) {
	userID := middleware.GetUserID(c)
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, ErrorResponse{Error: "unauthorized"})
		return
	}

	var req AddToWatchedRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid_request"})
		return
	}

	result, err := h.service.AddToWatched(c.Request.Context(), userID, &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "add_failed"})
		return
	}

	c.JSON(http.StatusOK, AddToWatchedResponse{
		Message:   "Movies added to watched",
		Added:     result.Added,
		Failed:    result.Failed,
		TotalAdd:  len(result.Added),
		TotalFail: len(result.Failed),
	})
}

// DeleteFromWatched handles DELETE /v1/watched/:id
//
//	@Summary		Remove movie from watched
//	@Description	Removes a movie from user's watched list
//	@Tags			watched
//	@Produce		json
//	@Param			id	path	int	true	"Watched item ID"
//	@Security		BearerAuth
//	@Success		204	"No content"
//	@Failure		401	{object}	ErrorResponse
//	@Failure		404	{object}	ErrorResponse
//	@Failure		500	{object}	ErrorResponse
//	@Router			/v1/watched/{id} [delete]
func (h *Handler) DeleteFromWatched(c *gin.Context) {
	userID := middleware.GetUserID(c)
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, ErrorResponse{Error: "unauthorized"})
		return
	}

	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid_id"})
		return
	}

	err = h.service.DeleteFromWatched(c.Request.Context(), userID, id)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			c.JSON(http.StatusNotFound, ErrorResponse{Error: "not_found"})
			return
		}
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "delete_failed"})
		return
	}

	c.Status(http.StatusNoContent)
}
