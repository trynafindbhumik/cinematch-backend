package export

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/trynafindbhumik/cinematch-backend/internal/shared/middleware"
)

// Handler handles export endpoints
type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// ExportData handles POST /v1/export
//
//	@Summary		Request data export
//	@Description	Generates and sends a data export via email
//	@Tags			export
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			request	body		ExportRequest	true	"Data to include in export"
//	@Success		200		{object}	ExportResponse
//	@Failure		401		{object}	ErrorResponse	"Unauthorized"
//	@Failure		500		{object}	ErrorResponse	"Server error"
//	@Router			/v1/export [post]
func (h *Handler) ExportData(c *gin.Context) {
	userID := middleware.GetUserID(c)
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, ErrorResponse{Error: "unauthorized"})
		return
	}

	var req ExportRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid request"})
		return
	}

	// At least one option must be selected
	if !req.ProfileInfo && !req.Preferences && !req.Watchlist && !req.Watched && !req.Favorites && !req.Reviews {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "at least one export option must be selected"})
		return
	}

	if err := h.service.ExportData(c.Request.Context(), userID, &req); err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "failed to process export request"})
		return
	}

	c.JSON(http.StatusOK, ExportResponse{Message: "export email has been sent"})
}

// ErrorResponse represents an API error
type ErrorResponse struct {
	Error string `json:"error"`
}
