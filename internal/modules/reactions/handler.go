package reactions

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

// AddReaction records a user's reaction to any movie
//
//	@Summary		Add reaction to movie
//	@Description	Record user's reaction (like/dislike/hate/love/skip) to any movie and update reaction counts
//	@Tags			reactions
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			request	body		AddReactionRequest	true	"Reaction details"
//	@Success		200	{object}	AddReactionResponse
//	@Failure		400	{object}	ErrorResponse
//	@Failure		401	{object}	ErrorResponse
//	@Failure		404	{object}	ErrorResponse
//	@Failure		500	{object}	ErrorResponse
//	@Router			/v1/reactions [post]
func (h *Handler) AddReaction(c *gin.Context) {
	userID := middleware.GetUserID(c)
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, ErrorResponse{Error: "unauthorized"})
		return
	}

	var req AddReactionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid request: " + err.Error()})
		return
	}

	if err := h.svc.AddReaction(c.Request.Context(), userID, req.TMDBID, req.Reaction); err != nil {
		if strings.Contains(err.Error(), "movie not found") {
			c.JSON(http.StatusNotFound, ErrorResponse{Error: err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "failed to add reaction"})
		return
	}

	c.JSON(http.StatusOK, AddReactionResponse{
		Success: true,
		Message: "reaction recorded successfully",
	})
}