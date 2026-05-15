package movies

// Movies module for searching and fetching movie data from TMDB.

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/trynafindbhumik/cinematch-backend/internal/shared/logger"
	"github.com/trynafindbhumik/cinematch-backend/internal/shared/middleware"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// Search handles GET /v1/movies/search
//
//	@Summary		Search movies
//	@Description	Searches movies via TMDB, caches results
//	@Tags			movies
//	@Accept			json
//	@Produce		json
//	@Param			q		query	string	true	"Search query"
//	@Param			page	query	int		false	"Page number"	default(1)
//	@Security		BearerAuth
//	@Success		200	{object}	SearchResponse
//	@Failure		400	{object}	ErrorResponse
//	@Failure		401	{object}	ErrorResponse
//	@Failure		500	{object}	ErrorResponse
//	@Router			/v1/movies/search [get]
func (h *Handler) Search(c *gin.Context) {
	query := c.Query("q")
	if query == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "missing_query",
			Message: "Search query is required",
		})
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	if page < 1 {
		page = 1
	}

	result, err := h.service.SearchMovies(c.Request.Context(), query, page)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "search_failed",
			Message: "Failed to search movies",
		})
		return
	}

	c.JSON(http.StatusOK, result)
}

// GetTrending handles GET /v1/movies/trending
//
//	@Summary		Get trending movies
//	@Description	Fetches trending movies from TMDB, caches results
//	@Tags			movies
//	@Accept			json
//	@Produce		json
//	@Param			page	query	int	false	"Page number"	default(1)
//	@Security		BearerAuth
//	@Success		200	{object}	TrendingResponse
//	@Failure		401	{object}	ErrorResponse
//	@Failure		500	{object}	ErrorResponse
//	@Router			/v1/movies/trending [get]
func (h *Handler) GetTrending(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	if page < 1 {
		page = 1
	}

	result, err := h.service.GetTrending(c.Request.Context(), page)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "trending_failed",
			Message: "Failed to get trending movies",
		})
		return
	}

	c.JSON(http.StatusOK, result)
}

// GetByID handles GET /v1/movies/:tmdb_id
//
//	@Summary		Get movie details
//	@Description	Gets detailed movie info with credits and watch providers by TMDB ID
//	@Tags			movies
//	@Accept			json
//	@Produce		json
//	@Param			tmdb_id	path	int	true	"TMDB Movie ID"
//	@Security		BearerAuth
//	@Success		200	{object}	MovieDetailsResponse
//	@Failure		400	{object}	ErrorResponse
//	@Failure		401	{object}	ErrorResponse
//	@Failure		404	{object}	ErrorResponse
//	@Failure		500	{object}	ErrorResponse
//	@Router			/v1/movies/{tmdb_id} [get]
func (h *Handler) GetByID(c *gin.Context) {
	tmdbIDStr := c.Param("tmdb_id")
	tmdbID, err := strconv.Atoi(tmdbIDStr)
	if err != nil || tmdbID <= 0 {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "invalid_tmdb_id",
			Message: "Valid TMDB ID is required",
		})
		return
	}

	userID := middleware.GetUserID(c)

	result, err := h.service.GetMovieByID(c.Request.Context(), tmdbID, userID)
	if err != nil {
		logger.Error("Failed to get movie by ID", logger.Err(err), logger.Int("tmdb_id", tmdbID))
		c.JSON(http.StatusNotFound, ErrorResponse{
			Error:   "movie_not_found",
			Message: "Failed to get movie details",
		})
		return
	}

	c.JSON(http.StatusOK, result)
}

// GetVideos handles GET /v1/movies/:tmdb_id/videos
//
//	@Summary		Get movie videos
//	@Description	Gets movie videos (trailers, teasers, etc.) by TMDB ID
//	@Tags			movies
//	@Accept			json
//	@Produce		json
//	@Param			tmdb_id	path	int	true	"TMDB Movie ID"
//	@Security		BearerAuth
//	@Success		200	{object}	VideosResponse
//	@Failure		400	{object}	ErrorResponse
//	@Failure		401	{object}	ErrorResponse
//	@Failure		500	{object}	ErrorResponse
//	@Router			/v1/movies/{tmdb_id}/videos [get]
func (h *Handler) GetVideos(c *gin.Context) {
	tmdbIDStr := c.Param("tmdb_id")
	tmdbID, err := strconv.Atoi(tmdbIDStr)
	if err != nil || tmdbID <= 0 {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "invalid_tmdb_id",
			Message: "Valid TMDB ID is required",
		})
		return
	}

	result, err := h.service.GetMovieVideos(c.Request.Context(), tmdbID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "videos_not_found",
			Message: "Failed to get movie videos",
		})
		return
	}

	c.JSON(http.StatusOK, result)
}