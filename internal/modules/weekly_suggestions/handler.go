package weekly_suggestions

import (
	"net/http"
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

// GetWeeklySuggestions returns weekly suggestions for current week
//
//	@Summary		Get weekly movie suggestions
//	@Description	Returns 5 movie suggestions for the current week (Sunday to Saturday). Requires at least 5 favorites and 20 reactions.
//	@Tags			weekly-suggestions
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Success		200	{object}	WeeklySuggestionResponse
//	@Failure		401	{object}	ErrorResponse
//	@Failure		422	{object}	ErrorResponse	"Minimum favorites (5) or reactions (20) not met"
//	@Failure		500	{object}	ErrorResponse
//	@Router			/v1/weekly-suggestions [get]
func (h *Handler) GetWeeklySuggestions(c *gin.Context) {
	userID := middleware.GetUserID(c)
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, ErrorResponse{Error: "unauthorized"})
		return
	}

	result, err := h.svc.GetWeeklySuggestions(c.Request.Context(), userID)
	if err != nil {
		if strings.Contains(err.Error(), "at least") && strings.Contains(err.Error(), "favorite movies required") {
			c.JSON(http.StatusUnprocessableEntity, ErrorResponse{Error: err.Error()})
			return
		}
		if strings.Contains(err.Error(), "at least") && strings.Contains(err.Error(), "movie reactions required") {
			c.JSON(http.StatusUnprocessableEntity, ErrorResponse{Error: err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "failed to get weekly suggestions"})
		return
	}

	c.JSON(http.StatusOK, WeeklySuggestionResponse{
		WeekStart:        result.WeekStart,
		Suggestions:      result.Suggestions,
		GeneratedAt:      result.GeneratedAt,
		AlreadyGenerated: result.AlreadyGenerated,
	})
}