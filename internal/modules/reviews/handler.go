package reviews

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/trynafindbhumik/cinematch-backend/internal/shared/middleware"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// CreateReview handles POST /v1/reviews
//
//	@Summary		Create a review
//	@Description	Creates a new review for a movie
//	@Tags			reviews
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			body	body		CreateReviewRequest	true	"Review data"
//	@Success		201		{object}	ReviewResponse
//	@Failure		400		{object}	ErrorResponse
//	@Failure		401		{object}	ErrorResponse
//	@Failure		500		{object}	ErrorResponse
//	@Router			/v1/reviews [post]
func (h *Handler) CreateReview(c *gin.Context) {
	var req CreateReviewRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "invalid_request",
			Message: err.Error(),
		})
		return
	}

	userID := middleware.GetUserID(c)
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Error:   "unauthorized",
			Message: "User not authenticated",
		})
		return
	}

	review, err := h.service.Create(c.Request.Context(), userID, &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "create_failed",
			Message: "Failed to create review",
		})
		return
	}

	c.JSON(http.StatusCreated, review)
}

// UpdateReview handles PATCH /v1/reviews/:id
//
//	@Summary		Update a review
//	@Description	Updates an existing review (rating and/or comment)
//	@Tags			reviews
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id		path		int64	true	"Review ID"
//	@Param			body	body		UpdateReviewRequest	true	"Update data"
//	@Success		200		{object}	ReviewResponse
//	@Failure		400		{object}	ErrorResponse
//	@Failure		401		{object}	ErrorResponse
//	@Failure		404		{object}	ErrorResponse
//	@Failure		500		{object}	ErrorResponse
//	@Router			/v1/reviews/{id} [patch]
func (h *Handler) UpdateReview(c *gin.Context) {
	reviewIDStr := c.Param("id")
	reviewID, err := strconv.ParseInt(reviewIDStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "invalid_id",
			Message: "Valid review ID is required",
		})
		return
	}

	var req UpdateReviewRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "invalid_request",
			Message: err.Error(),
		})
		return
	}

	userID := middleware.GetUserID(c)
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Error:   "unauthorized",
			Message: "User not authenticated",
		})
		return
	}

	review, err := h.service.Update(c.Request.Context(), userID, reviewID, &req)
	if err != nil {
		if err.Error() == "review not found" {
			c.JSON(http.StatusNotFound, ErrorResponse{
				Error:   "not_found",
				Message: "Review not found",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "update_failed",
			Message: "Failed to update review",
		})
		return
	}

	c.JSON(http.StatusOK, review)
}

// DeleteReview handles DELETE /v1/reviews/:id
//
//	@Summary		Delete a review
//	@Description	Deletes a review by ID
//	@Tags			reviews
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id	path		int64	true	"Review ID"
//	@Success		204	{object}	nil
//	@Failure		400	{object}	ErrorResponse
//	@Failure		401	{object}	ErrorResponse
//	@Failure		404	{object}	ErrorResponse
//	@Failure		500	{object}	ErrorResponse
//	@Router			/v1/reviews/{id} [delete]
func (h *Handler) DeleteReview(c *gin.Context) {
	reviewIDStr := c.Param("id")
	reviewID, err := strconv.ParseInt(reviewIDStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "invalid_id",
			Message: "Valid review ID is required",
		})
		return
	}

	userID := middleware.GetUserID(c)
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Error:   "unauthorized",
			Message: "User not authenticated",
		})
		return
	}

	err = h.service.Delete(c.Request.Context(), userID, reviewID)
	if err != nil {
		if err.Error() == "review not found" {
			c.JSON(http.StatusNotFound, ErrorResponse{
				Error:   "not_found",
				Message: "Review not found",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "delete_failed",
			Message: "Failed to delete review",
		})
		return
	}

	c.Status(http.StatusNoContent)
}

// GetUserReviews handles GET /v1/reviews
//
//	@Summary		Get user's reviews
//	@Description	Gets reviews for the authenticated user with cursor pagination and optional date filtering
//	@Tags			reviews
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			cursor	query	string	false	"Pagination cursor"
//	@Param			limit	query	int		false	"Number of results"	default(10)
//	@Param			from	query	string	false	"Start date (DD-MM-YYYY)"
//	@Param			to		query	string	false	"End date (DD-MM-YYYY)"
//	@Success		200		{object}	ReviewsListResponse
//	@Failure		401		{object}	ErrorResponse
//	@Failure		500		{object}	ErrorResponse
//	@Router			/v1/reviews [get]
func (h *Handler) GetUserReviews(c *gin.Context) {
	cursor := c.Query("cursor")
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))

	// Parse date filters (format: DD-MM-YYYY)
	var fromDate, toDate *time.Time
	if from := c.Query("from"); from != "" {
		if t, err := time.Parse("02-01-2006", from); err == nil {
			fromDate = &t
		}
	}
	if to := c.Query("to"); to != "" {
		if t, err := time.Parse("02-01-2006", to); err == nil {
			// Set to end of day
			endOfDay := t.Add(24*time.Hour - time.Second)
			toDate = &endOfDay
		}
	}

	userID := middleware.GetUserID(c)
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Error:   "unauthorized",
			Message: "User not authenticated",
		})
		return
	}

	result, err := h.service.GetUserReviews(c.Request.Context(), userID, cursor, limit, fromDate, toDate)
	if err != nil {
		fmt.Printf("[DEBUG GetUserReviews] Error: %v | Cursor: %s | Limit: %d\n", err, cursor, limit)
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "fetch_failed",
			Message: fmt.Sprintf("Failed to get reviews: %v", err),
		})
		return
	}

	c.JSON(http.StatusOK, result)
}

// GetMovieReviews handles GET /v1/movies/:tmdb_id/reviews
//
//	@Summary		Get movie reviews
//	@Description	Gets reviews for a movie with hybrid cursor pagination (DB + TMDB)
//	@Tags			reviews
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			tmdb_id	path		int		true	"TMDB Movie ID"
//	@Param			cursor	query	string	false	"Pagination cursor"
//	@Param			limit	query	int		false	"Number of results"	default(10)
//	@Success		200		{object}	ReviewsListResponse
//	@Failure		400		{object}	ErrorResponse
//	@Failure		401		{object}	ErrorResponse
//	@Failure		500		{object}	ErrorResponse
//	@Router			/v1/movies/{tmdb_id}/reviews [get]
func (h *Handler) GetMovieReviews(c *gin.Context) {
	tmdbIDStr := c.Param("tmdb_id")
	tmdbID, err := strconv.Atoi(tmdbIDStr)
	if err != nil || tmdbID <= 0 {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "invalid_tmdb_id",
			Message: "Valid TMDB ID is required",
		})
		return
	}

	cursor := c.Query("cursor")
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))

	result, err := h.service.GetMovieReviews(c.Request.Context(), tmdbID, cursor, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "fetch_failed",
			Message: "Failed to get reviews",
		})
		return
	}

	c.JSON(http.StatusOK, result)
}