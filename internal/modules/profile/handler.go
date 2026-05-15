package profile

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/trynafindbhumik/cinematch-backend/internal/shared/middleware"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// GetProfile returns current user's profile
//
//	@Summary		Get current user profile
//	@Description	Returns the profile of the authenticated user
//	@Tags			profile
//	@Produce		json
//	@Security		BearerAuth
//	@Success		200	{object}	GetProfileResponse
//	@Failure		401	{object}	ErrorResponse	"Unauthorized"
//	@Failure		404	{object}	ErrorResponse	"User not found"
//	@Failure		500	{object}	ErrorResponse	"Server error"
//	@Router			/v1/profile/me [get]
func (h *Handler) GetProfile(c *gin.Context) {
	userID := middleware.GetUserID(c)
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, ErrorResponse{Error: "unauthorized"})
		return
	}

	profile, err := h.service.GetProfile(c.Request.Context(), userID)
	if err != nil {
		if errors.Is(err, ErrUserNotFound) {
			c.JSON(http.StatusNotFound, ErrorResponse{Error: "user not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "failed to get profile"})
		return
	}

	c.JSON(http.StatusOK, profile)
}

// UpdateProfile handles profile picture and name updates
//
//	@Summary		Update user profile
//	@Description	Updates name and/or profile picture for the authenticated user
//	@Tags			profile
//	@Accept			multipart/form-data
//	@Produce		json
//	@Security		BearerAuth
//	@Param			name			formData	string	false	"Display name"
//	@Param			profile_picture	formData	file	false	"Profile picture (max 5MB)"
//	@Param			smartSuggest		formData	bool	false	"Enable smart suggestions"
//	@Success		200	{object}	UpdateProfileResponse
//	@Failure		400	{object}	ErrorResponse	"Invalid request"
//	@Failure		401	{object}	ErrorResponse	"Unauthorized"
//	@Failure		500	{object}	ErrorResponse	"Server error"
//	@Router			/v1/profile/me [put]
func (h *Handler) UpdateProfile(c *gin.Context) {
	userID := middleware.GetUserID(c)
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, ErrorResponse{Error: "unauthorized"})
		return
	}

	userPublicID := c.GetString("userPublicID")

	if err := c.Request.ParseMultipartForm(10 << 20); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "failed to parse form data"})
		return
	}

	name := c.PostForm("name")

	smartSuggest := c.PostForm("smartSuggest")
	var smartSuggestBool *bool
	if smartSuggest != "" {
		val := smartSuggest == "true" || smartSuggest == "1"
		smartSuggestBool = &val
	}

	var profilePicture []byte
	file, err := c.FormFile("profile_picture")
	if err == nil {
		if file.Size > 5*1024*1024 {
			c.JSON(http.StatusBadRequest, ErrorResponse{Error: "file size exceeds 5MB limit"})
			return
		}

		src, err := file.Open()
		if err != nil {
			c.JSON(http.StatusBadRequest, ErrorResponse{Error: "failed to read file"})
			return
		}
		defer src.Close()

		buf := make([]byte, file.Size)
		if _, err := src.Read(buf); err != nil {
			c.JSON(http.StatusBadRequest, ErrorResponse{Error: "failed to read file content"})
			return
		}
		profilePicture = buf
	}

	req := &UpdateProfileRequest{
		Name:           name,
		ProfilePicture: profilePicture,
		SmartSuggest:   smartSuggestBool,
	}

	result, err := h.service.UpdateProfile(c.Request.Context(), userID, userPublicID, req)
	if err != nil {
		if errors.Is(err, ErrNoChangesDetected) {
			c.JSON(http.StatusBadRequest, ErrorResponse{Error: "no changes provided"})
			return
		}
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "failed to update profile"})
		return
	}

	c.JSON(http.StatusOK, result)
}

// ChangePassword verifies old password and updates to new password
//
//	@Summary		Change password
//	@Description	Verifies old password and updates to new password
//	@Tags			profile
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			request	body		ChangePasswordRequest	true	"Password change request"
//	@Success		200		{object}	ChangePasswordResponse
//	@Failure		400		{object}	ErrorResponse			"Invalid request"
//	@Failure		401		{object}	ErrorResponse			"Invalid old password"
//	@Failure		500		{object}	ErrorResponse			"Server error"
//	@Router			/v1/profile/password [put]
func (h *Handler) ChangePassword(c *gin.Context) {
	userID := middleware.GetUserID(c)
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, ErrorResponse{Error: "unauthorized"})
		return
	}

	var req ChangePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid request"})
		return
	}

	result, err := h.service.ChangePassword(c.Request.Context(), userID, &req)
	if err != nil {
		if errors.Is(err, ErrInvalidPassword) {
			c.JSON(http.StatusUnauthorized, ErrorResponse{Error: "invalid password"})
			return
		}
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "failed to change password"})
		return
	}

	c.JSON(http.StatusOK, result)
}

// InitiateEmailChange starts email change flow (sends OTP to old email)
//
//	@Summary		Initiate email change
//	@Description	Starts the email change flow by sending OTP to old email
//	@Tags			profile
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			request	body		InitiateEmailChangeRequest	true	"New email address"
//	@Success		200		{object}	EmailChangeStepResponse
//	@Failure		400		{object}	ErrorResponse				"Invalid request"
//	@Failure		409		{object}	ErrorResponse				"Email already in use"
//	@Failure		500		{object}	ErrorResponse				"Server error"
//	@Router			/v1/profile/email/change [post]
func (h *Handler) InitiateEmailChange(c *gin.Context) {
	userID := middleware.GetUserID(c)
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, ErrorResponse{Error: "unauthorized"})
		return
	}

	var req InitiateEmailChangeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid request"})
		return
	}

	result, err := h.service.InitiateEmailChange(c.Request.Context(), userID, req.NewEmail)
	if err != nil {
		if errors.Is(err, ErrEmailAlreadyExists) {
			c.JSON(http.StatusConflict, ErrorResponse{Error: "email already in use"})
			return
		}
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "failed to initiate email change"})
		return
	}

	c.JSON(http.StatusOK, result)
}

// VerifyEmail verifies email with OTP (used for email change flow)
//
//	@Summary		Verify email change
//	@Description	Verifies email change with OTP
//	@Tags			profile
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			request	body		VerifyEmailRequest	true	"OTP verification request"
//	@Success		200		{object}	VerifyEmailResponse
//	@Failure		400		{object}	ErrorResponse	"Invalid request"
//	@Failure		401		{object}	ErrorResponse	"Invalid OTP or expired"
//	@Failure		429		{object}	ErrorResponse	"Max attempts reached"
//	@Failure		500		{object}	ErrorResponse	"Server error"
//	@Router			/v1/profile/verify [post]
func (h *Handler) VerifyEmail(c *gin.Context) {
	userID := middleware.GetUserID(c)
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, ErrorResponse{Error: "unauthorized"})
		return
	}

	var req VerifyEmailRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid_request"})
		return
	}

	resp, err := h.service.VerifyEmail(c.Request.Context(), userID, &req)
	if err != nil {
		switch {
		case errors.Is(err, ErrInvalidOTP):
			c.JSON(http.StatusUnauthorized, ErrorResponse{Error: "invalid_otp"})
		case errors.Is(err, ErrEmailChangeExpired):
			c.JSON(http.StatusUnauthorized, ErrorResponse{Error: "token_expired"})
		case errors.Is(err, ErrMaxAttemptsReached):
			c.JSON(http.StatusTooManyRequests, ErrorResponse{Error: "max_attempts"})
		default:
			c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "server_error"})
		}
		return
	}

	c.JSON(http.StatusOK, resp)
}

// ResendEmailChange resends OTP for email change flow
//
//	@Summary		Resend email change OTP
//	@Description	Resends OTP for email change flow
//	@Tags			profile
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Success		200	{object}	ResendEmailChangeResponse
//	@Failure		401	{object}	ErrorResponse	"Unauthorized"
//	@Failure		429	{object}	ErrorResponse	"Cooldown active"
//	@Failure		500	{object}	ErrorResponse	"Server error"
//	@Router			/v1/profile/email/resend [post]
func (h *Handler) ResendEmailChange(c *gin.Context) {
	userID := middleware.GetUserID(c)
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, ErrorResponse{Error: "unauthorized"})
		return
	}

	result, err := h.service.ResendEmailChange(c.Request.Context(), userID)
	if err != nil {
		if errors.Is(err, ErrCooldownNotPassed) {
			c.JSON(http.StatusTooManyRequests, ErrorResponse{Error: "cooldown_active", Message: err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "failed to resend OTP"})
		return
	}

	c.JSON(http.StatusOK, result)
}

// DisableAccount temporarily disables user account
//
//	@Summary		Disable account
//	@Description	Temporarily disables user account for specified duration
//	@Tags			profile
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			request	body		DisableAccountRequest	true	"Disable account request"
//	@Success		200		{object}	DisableAccountResponse
//	@Failure		400		{object}	ErrorResponse			"Invalid request"
//	@Failure		401		{object}	ErrorResponse			"Unauthorized"
//	@Failure		500		{object}	ErrorResponse			"Server error"
//	@Router			/v1/profile/disable [put]
func (h *Handler) DisableAccount(c *gin.Context) {
	userID := middleware.GetUserID(c)
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, ErrorResponse{Error: "unauthorized"})
		return
	}

	var req DisableAccountRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid request"})
		return
	}

	result, err := h.service.DisableAccount(c.Request.Context(), userID, &req)
	if err != nil {
		if errors.Is(err, ErrInvalidConfirmText) {
			c.JSON(http.StatusBadRequest, ErrorResponse{Error: "confirmation text must be 'DISABLE'"})
			return
		}
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "failed to disable account"})
		return
	}

	c.JSON(http.StatusOK, result)
}

// DeleteAccount soft-deletes user account
//
//	@Summary		Delete account
//	@Description	Soft-deletes the user account permanently
//	@Tags			profile
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			request	body		DeleteAccountRequest	true	"Delete account request"
//	@Success		200		{object}	DeleteAccountResponse
//	@Failure		400		{object}	ErrorResponse	"Invalid request"
//	@Failure		401		{object}	ErrorResponse	"Unauthorized"
//	@Failure		500		{object}	ErrorResponse	"Server error"
//	@Router			/v1/profile/me [delete]
func (h *Handler) DeleteAccount(c *gin.Context) {
	userID := middleware.GetUserID(c)
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, ErrorResponse{Error: "unauthorized"})
		return
	}

	var req DeleteAccountRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid request"})
		return
	}

	result, err := h.service.DeleteAccount(c.Request.Context(), userID, &req)
	if err != nil {
		if errors.Is(err, ErrInvalidConfirmText) {
			c.JSON(http.StatusBadRequest, ErrorResponse{Error: "confirmation text must be 'DELETE'"})
			return
		}
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "failed to delete account"})
		return
	}

	c.JSON(http.StatusOK, result)
}

// DeleteProfilePicture removes the user's profile picture
//
//	@Summary		Delete profile picture
//	@Description	Removes the authenticated user's profile picture
//	@Tags			profile
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Success		200	{object}	DeleteProfilePictureResponse
//	@Failure		401	{object}	ErrorResponse	"Unauthorized"
//	@Failure		404	{object}	ErrorResponse	"No profile picture to delete"
//	@Failure		500	{object}	ErrorResponse	"Server error"
//	@Router			/v1/profile/me/picture [delete]
func (h *Handler) DeleteProfilePicture(c *gin.Context) {
	userID := middleware.GetUserID(c)
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, ErrorResponse{Error: "unauthorized"})
		return
	}

	result, err := h.service.DeleteProfilePicture(c.Request.Context(), userID)
	if err != nil {
		if err.Error() == "no profile picture to delete" {
			c.JSON(http.StatusNotFound, ErrorResponse{Error: "no profile picture to delete"})
			return
		}
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "failed to delete profile picture"})
		return
	}

	c.JSON(http.StatusOK, result)
}

// GetUserIDFromContext extracts user ID from context using middleware helpers
func GetUserIDFromContext(c *gin.Context) (int64, string) {
	return middleware.GetUserID(c), middleware.GetUserPublicID(c)
}
