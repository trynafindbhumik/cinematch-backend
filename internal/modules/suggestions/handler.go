package suggestions

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/trynafindbhumik/cinematch-backend/internal/shared/middleware"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

type ErrorResponse struct {
	Error string `json:"error"`
}

// GenerateSuggestions generates or resumes movie suggestions
//
//	@Summary		Generate or resume movie suggestions
//	@Description	Returns 1-2 movies with full details. Checks old logs first, then today's log.
//	@Tags			suggestions
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Success		200	{object}	GenerateSuggestionsResponse
//	@Failure		401	{object}	ErrorResponse
//	@Failure		422	{object}	ErrorResponse	"Minimum favorites not met"
//	@Failure		500	{object}	ErrorResponse
//	@Router			/v1/suggestions/generate [get]
func (h *Handler) GenerateSuggestions(c *gin.Context) {
	userID := middleware.GetUserID(c)
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, ErrorResponse{Error: "unauthorized"})
		return
	}

	result, err := h.svc.GenerateSuggestions(c.Request.Context(), userID)
	if err != nil {
		if strings.Contains(err.Error(), "at least") && strings.Contains(err.Error(), "favorite movies required") {
			c.JSON(http.StatusUnprocessableEntity, ErrorResponse{Error: err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, GenerateSuggestionsResponse{
		Suggestions:    result.Suggestions,
		GenerationDate:  result.GenerationDate,
		Regeneration:   result.Regeneration,
		Finished:       result.Finished,
		Message:        result.Message,
	})
}

// GetNext returns the next movie after the given tmdb_id
//
//	@Summary		Get next movie suggestion
//	@Description	Returns next movie with full details based on current tmdb_id
//	@Tags			suggestions
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			tmdb_id	query	int	true	"Current movie TMDB ID"
//	@Success		200	{object}	NextResponse
//	@Failure		400	{object}	ErrorResponse
//	@Failure		401	{object}	ErrorResponse
//	@Failure		404	{object}	ErrorResponse
//	@Failure		500	{object}	ErrorResponse
//	@Router			/v1/suggestions/next [get]
func (h *Handler) GetNext(c *gin.Context) {
	userID := middleware.GetUserID(c)
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, ErrorResponse{Error: "unauthorized"})
		return
	}

	tmdbIDStr := c.Query("tmdb_id")
	if tmdbIDStr == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "tmdb_id is required"})
		return
	}

	tmdbID, err := strconv.Atoi(tmdbIDStr)
	if err != nil || tmdbID <= 0 {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "valid tmdb_id is required"})
		return
	}

	result, err := h.svc.GetNextMovie(c.Request.Context(), userID, tmdbID)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			c.JSON(http.StatusNotFound, ErrorResponse{Error: "movie not found in active suggestions"})
			return
		}
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "failed to get next suggestion"})
		return
	}

	c.JSON(http.StatusOK, NextResponse{
		Suggestion:   result.Suggestion,
		NextTMDBID:   result.NextTMDBID,
		HasMore:      result.HasMore,
		Regeneration: result.Regeneration,
		Finished:     result.Finished,
		Message:      result.Message,
	})
}
