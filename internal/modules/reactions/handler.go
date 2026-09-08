package reactions

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

// RemoveReaction removes a user's reaction from a movie
//
//	@Summary		Remove reaction from movie
//	@Description	Remove user's reaction from a movie
//	@Tags			reactions
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			tmdb_id	path		int	true	"TMDB Movie ID"
//	@Success		200		{object}	map[string]interface{}
//	@Failure		400		{object}	ErrorResponse
//	@Failure		401		{object}	ErrorResponse
//	@Failure		404		{object}	ErrorResponse
//	@Failure		500		{object}	ErrorResponse
//	@Router			/v1/reactions/{tmdb_id} [delete]
func (h *Handler) RemoveReaction(c *gin.Context) {
	userID := middleware.GetUserID(c)
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, ErrorResponse{Error: "unauthorized"})
		return
	}

	tmdbIDStr := c.Param("tmdb_id")
	tmdbID, err := strconv.Atoi(tmdbIDStr)
	if err != nil || tmdbID <= 0 {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "valid tmdb_id is required"})
		return
	}

	if err := h.svc.RemoveReaction(c.Request.Context(), userID, tmdbID); err != nil {
		if strings.Contains(err.Error(), "movie not found") {
			c.JSON(http.StatusNotFound, ErrorResponse{Error: err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "failed to remove reaction"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "reaction removed successfully",
	})
}