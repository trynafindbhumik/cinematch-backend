package watchlist

// Watchlist module for managing user's movies to watch.

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/trynafindbhumik/cinematch-backend/internal/shared/middleware"
)

// WatchlistMovie represents a movie in user's watchlist
type WatchlistMovie struct {
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

// GetWatchlistResponse is the response for get watchlist
type GetWatchlistResponse struct {
	Movies     []WatchlistMovie `json:"movies"`
	NextCursor string           `json:"next_cursor,omitempty"`
	Limit      int              `json:"limit"`
	TotalCount int              `json:"total_count"`
}

// GetWatchlistIDsResponse is the response for get watchlist IDs
type GetWatchlistIDsResponse struct {
	TMDBIDs []int `json:"tmdb_ids"`
}

// AddToWatchlistRequest is the request body for adding to watchlist
type AddToWatchlistRequest struct {
	TMDBIDs []int `json:"tmdb_ids" binding:"required,min=1"`
}

// AddToWatchlistResponse is the response for adding to watchlist
type AddToWatchlistResponse struct {
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

// Handler handles watchlist HTTP endpoints
type Handler struct {
	service *Service
}

// NewHandler creates a new watchlist handler
func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// GetWatchlist handles GET /v1/watchlist
//
//	@Summary		Get user's watchlist
//	@Description	Returns paginated list of user's watchlist movies
//	@Tags			watchlist
//	@Produce		json
//	@Param			cursor	query	string	false	"Cursor for pagination"
//	@Param			limit	query	int		false	"Number of items"	default(20)
//	@Param			genre	query	string	false	"Filter by genre"
//	@Security		BearerAuth
//	@Success		200	{object}	GetWatchlistResponse
//	@Failure		401	{object}	ErrorResponse
//	@Failure		500	{object}	ErrorResponse
//	@Router			/v1/watchlist [get]
func (h *Handler) GetWatchlist(c *gin.Context) {
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

	result, err := h.service.GetWatchlist(c.Request.Context(), userID, cursor, limit, genre)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "fetch_failed"})
		return
	}

	c.JSON(http.StatusOK, result)
}

// GetWatchlistIDs handles GET /v1/watchlist/ids
//
//	@Summary		Get all watchlist TMDB IDs
//	@Description	Returns all TMDB IDs that user has in watchlist
//	@Tags			watchlist
//	@Produce		json
//	@Security		BearerAuth
//	@Success		200	{object}	GetWatchlistIDsResponse
//	@Failure		401	{object}	ErrorResponse
//	@Failure		500	{object}	ErrorResponse
//	@Router			/v1/watchlist/ids [get]
func (h *Handler) GetWatchlistIDs(c *gin.Context) {
	userID := middleware.GetUserID(c)
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, ErrorResponse{Error: "unauthorized"})
		return
	}

	result, err := h.service.GetWatchlistIDs(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "fetch_failed"})
		return
	}

	c.JSON(http.StatusOK, result)
}

// SearchWatchlist handles GET /v1/watchlist/search
//
//	@Summary		Search user's watchlist
//	@Description	Searches within user's watchlist by title
//	@Tags			watchlist
//	@Produce		json
//	@Param			q		query	string	true	"Search query"
//	@Param			cursor	query	string	false	"Cursor for pagination"
//	@Param			limit	query	int		false	"Items per page"	default(20)
//	@Security		BearerAuth
//	@Success		200	{object}	GetWatchlistResponse
//	@Failure		401	{object}	ErrorResponse
//	@Failure		500	{object}	ErrorResponse
//	@Router			/v1/watchlist/search [get]
func (h *Handler) SearchWatchlist(c *gin.Context) {
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

	result, err := h.service.SearchWatchlist(c.Request.Context(), userID, query, cursor, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "search_failed"})
		return
	}

	c.JSON(http.StatusOK, result)
}

// AddToWatchlist handles POST /v1/watchlist
//
//	@Summary		Add movies to watchlist
//	@Description	Adds one or more movies to user's watchlist
//	@Tags			watchlist
//	@Accept			json
//	@Produce		json
//	@Param			request	body	AddToWatchlistRequest	true	"Movies to add"
//	@Security		BearerAuth
//	@Success		201	{object}	map[string]string
//	@Failure		400	{object}	ErrorResponse
//	@Failure		401	{object}	ErrorResponse
//	@Failure		500	{object}	ErrorResponse
//	@Router			/v1/watchlist [post]
func (h *Handler) AddToWatchlist(c *gin.Context) {
	userID := middleware.GetUserID(c)
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, ErrorResponse{Error: "unauthorized"})
		return
	}

	var req AddToWatchlistRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid_request"})
		return
	}

	result, err := h.service.AddToWatchlist(c.Request.Context(), userID, &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "add_failed"})
		return
	}

	c.JSON(http.StatusOK, AddToWatchlistResponse{
		Message:   "Movies added to watchlist",
		Added:     result.Added,
		Failed:    result.Failed,
		TotalAdd:  len(result.Added),
		TotalFail: len(result.Failed),
	})
}

// DeleteFromWatchlist handles DELETE /v1/watchlist/:id
//
//	@Summary		Remove movie from watchlist
//	@Description	Removes a movie from user's watchlist
//	@Tags			watchlist
//	@Produce		json
//	@Param			id	path	int	true	"Watchlist item ID"
//	@Security		BearerAuth
//	@Success		204	"No content"
//	@Failure		401	{object}	ErrorResponse
//	@Failure		404	{object}	ErrorResponse
//	@Failure		500	{object}	ErrorResponse
//	@Router			/v1/watchlist/{id} [delete]
func (h *Handler) DeleteFromWatchlist(c *gin.Context) {
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

	err = h.service.DeleteFromWatchlist(c.Request.Context(), userID, id)
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
