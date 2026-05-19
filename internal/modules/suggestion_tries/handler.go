package suggestion_tries

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/trynafindbhumik/cinematch-backend/internal/shared/middleware"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// GenerateSuggestions generates movie suggestions with 3 tries per week limit
//
//	@Summary		Generate movie suggestions with weekly tries
//	@Description	Returns 5 movie suggestions. User has 3 tries per week (resets every Sunday IST). Requires at least 5 favorites and 20 reactions.
//	@Tags			suggestion-tries
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Success		200	{object}	SuggestionTriesResponse
//	@Failure		401	{object}	ErrorResponse
//	@Failure		422	{object}	ErrorResponse	"Minimum favorites (5) or reactions (20) not met"
//	@Failure		429	{object}	ErrorResponse	"Weekly tries exhausted"
//	@Failure		500	{object}	ErrorResponse
//	@Router			/v1/suggestion-tries/generate [get]
func (h *Handler) GenerateSuggestions(c *gin.Context) {
	userID := middleware.GetUserID(c)
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, ErrorResponse{Error: "unauthorized"})
		return
	}

	result, err := h.svc.GenerateSuggestions(c.Request.Context(), userID)
	if err != nil {
		if errors.Is(err, errWeeklyLimitExhausted) {
			c.JSON(http.StatusTooManyRequests, ErrorResponse{Error: err.Error()})
			return
		}
		if err.Error() == fmt.Sprintf("at least %d movie reactions required", minReactions) {
			c.JSON(http.StatusUnprocessableEntity, ErrorResponse{Error: err.Error()})
			return
		}
		if err.Error() == "at least 5 favorite movies required" {
			c.JSON(http.StatusUnprocessableEntity, ErrorResponse{Error: err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, SuggestionTriesResponse{
		WeekStart:      result.WeekStart,
		TryNumber:      result.TryNumber,
		Suggestions:    result.Suggestions,
		GeneratedAt:    result.GeneratedAt,
		RemainingTries: result.RemainingTries,
	})
}