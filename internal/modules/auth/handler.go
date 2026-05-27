package auth

// Authentication module handling user signup, login, logout, and token management.

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/trynafindbhumik/cinematch-backend/internal/config"
	"github.com/trynafindbhumik/cinematch-backend/internal/shared/hash"
	"github.com/trynafindbhumik/cinematch-backend/internal/shared/logger"
	"github.com/trynafindbhumik/cinematch-backend/internal/shared/jwt"
	"github.com/trynafindbhumik/cinematch-backend/internal/shared/middleware"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// setRefreshCookie sets HTTP-only cookie for refresh token storage.
// Cookie is secure (HTTPS-only) in production but lax in development.
func (h *Handler) setRefreshCookie(c *gin.Context, token string, maxAge int) {
	secure := config.App.Environment == "production"

	if secure {
		c.SetSameSite(http.SameSiteNoneMode)
	} else {
		c.SetSameSite(http.SameSiteLaxMode)
	}
		c.SetCookie(
		"refreshToken",
		token,
		maxAge,
		"/",
		"",
		secure,
		true,
	)
}

// Signup handles user registration
//
//	@Summary		Register a new user
//	@Description	Creates a new user account and sends verification OTP via email
//	@Tags			auth
//	@Accept			json
//	@Produce		json
//	@Param			request	body		SignupRequest	true	"Signup request"
//	@Success		200		{object}	SignupResponse	"User created, verification required"
//	@Success		409		{object}	ErrorResponse	"Email already exists"
//	@Failure		400		{object}	ErrorResponse	"Invalid request"
//	@Failure		500		{object}	ErrorResponse	"Server error"
//	@Router			/v1/auth/signup [post]
func (h *Handler) Signup(c *gin.Context) {
	var req SignupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "invalid_request",
			Message: err.Error(),
		})
		return
	}

	resp, err := h.service.Signup(c.Request.Context(), &req)
	if err != nil {
		logger.Error("Signup failed",
			logger.String("email_prefix", maskEmail(req.Email)),
			logger.Err(err),
		)
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "server_error",
			Message: "Failed to process signup",
		})
		return
	}

	// Email already exists returns empty verificationId
	if resp.VerificationID == "" {
		c.JSON(http.StatusConflict, ErrorResponse{
			Error:   "email_exists",
			Message: "Email already exists",
		})
		return
	}

	c.JSON(http.StatusOK, resp)
}

// Login handles user authentication
//
//	@Summary		Login user
//	@Description	Authenticates user with email and password, creates session
//	@Tags			auth
//	@Accept			json
//	@Produce		json
//	@Param			request	body		LoginRequest	true	"Login request"
//	@Success		200		{object}	LoginResponse	"Login successful"
//	@Failure		400		{object}	ErrorResponse	"Invalid request"
//	@Failure		401		{object}	ErrorResponse	"Invalid credentials"
//	@Failure		429		{object}	ErrorResponse	"Account locked"
//	@Failure		500		{object}	ErrorResponse	"Server error"
//	@Router			/v1/auth/login [post]
func (h *Handler) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "invalid_request",
			Message: err.Error(),
		})
		return
	}

	resp, err := h.service.Login(c.Request.Context(), &req, SessionParams{
		DeviceName: c.GetHeader("X-Device-Name"),
		UserAgent:  c.GetHeader("User-Agent"),
		IPAddress:  c.ClientIP(),
	})
	if err != nil {
		// Return 429 if account is temporarily locked due to failed attempts
		if err.Error() == "account is temporarily locked" || err.Error() == "account is temporarily locked due to too many failed attempts" {
			c.JSON(http.StatusTooManyRequests, ErrorResponse{
				Error:   "account_locked",
				Message: "Too many failed attempts. Please try again later.",
			})
			return
		}
		// Don't treat ErrMaxSessionsReached as error here - it's handled in response
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Error:   "invalid_credentials",
			Message: "Invalid email or password",
		})
		return
	}

	// If sessions exhausted, still set cookie but return with session exhausted flag
	h.setRefreshCookie(c, resp.RefreshToken, 30*24*60*60)
	resp.RefreshToken = "" // Clear from JSON since it's set as cookie

	// If magic link was generated (session exhausted), also clear from cookie response
	// but keep it in the JSON response for frontend to use
	if resp.MagicLink != "" {
		// Magic link stays in JSON response
	}

	c.JSON(http.StatusOK, resp)
}

// Logout handles user logout
//
//	@Summary		Logout user
//	@Description	Invalidates user session and clears refresh token cookie
//	@Tags			auth
//	@Accept			json
//	@Produce		json
//	@Param			request	body	LogoutRequest	false	"Logout request"
//	@Success		204		"No content"
//	@Router			/v1/auth/logout [post]
func (h *Handler) Logout(c *gin.Context) {
	refreshToken, _ := c.Cookie("refreshToken")

	// Extract access token to revoke it
	accessToken := ""
	if authHeader := c.GetHeader("Authorization"); authHeader != "" {
		accessToken = strings.TrimPrefix(authHeader, "Bearer ")
	}

	if refreshToken != "" || accessToken != "" {
		h.service.Logout(c.Request.Context(), refreshToken, accessToken)
	}

	// Clear the cookie
	h.setRefreshCookie(c, "", -1)

	c.Status(http.StatusNoContent)
}

// RefreshToken handles token refresh
//
//	@Summary		Refresh access token
//	@Description	Validates refresh token from HTTP-only cookie and issues new access token
//	@Tags			auth
//	@Produce		json
//	@Security		BearerAuth
//	@Success		200	{object}	RefreshTokenResponse	"New access token"
//	@Failure		401	{object}	ErrorResponse			"Invalid or expired refresh token"
//	@Router			/v1/auth/refresh [post]
func (h *Handler) RefreshToken(c *gin.Context) {
	// Get refresh token from HTTP-only cookie (not accessible to frontend JS)
	refreshToken, err := c.Cookie("refreshToken")
	logger.Debug("RefreshToken handler",
		logger.String("cookie_value", refreshToken),
		logger.Bool("cookie_err_nil", err == nil),
		logger.String("err_type", fmt.Sprintf("%T", err)),
	)
	if err != nil || refreshToken == "" {
		logger.Debug("RefreshToken: missing token from cookie")
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Error:   "missing_token",
			Message: "Refresh token is required",
		})
		return
	}

	resp, err := h.service.RefreshToken(c.Request.Context(), refreshToken)
	logger.Debug("RefreshToken service call",
		logger.String("refresh_token", refreshToken),
		logger.Err(err),
	)
	if err != nil {
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Error:   "invalid_token",
			Message: "Invalid or expired refresh token",
		})
		return
	}

	// Set new cookie and clear from JSON response
	h.setRefreshCookie(c, resp.RefreshToken, 30*24*60*60)
	resp.RefreshToken = ""

	c.JSON(http.StatusOK, resp)
}

// Verify handles email verification (signup only)
//
//	@Summary		Verify email
//	@Description	Verifies user email with OTP or token from email link
//	@Tags			auth
//	@Accept			json
//	@Produce		json
//	@Param			request	body		VerifyRequest	true	"Verification request"
//	@Success		200		{object}	VerifyResponse	"Email verified"
//	@Failure		400		{object}	ErrorResponse	"Invalid request"
//	@Failure		401		{object}	ErrorResponse	"Invalid or expired token"
//	@Failure		429		{object}	ErrorResponse	"Max attempts reached"
//	@Failure		500		{object}	ErrorResponse	"Server error"
//	@Router			/v1/auth/verify [post]
func (h *Handler) Verify(c *gin.Context) {
	var req VerifyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "invalid_request",
			Message: err.Error(),
		})
		return
	}

	sessionParams := SessionParams{
		DeviceName: c.GetHeader("X-Device-Name"),
		UserAgent:  c.GetHeader("User-Agent"),
		IPAddress: c.ClientIP(),
	}

	resp, err := h.service.Verify(c.Request.Context(), &req, sessionParams)
	if err != nil {
		logger.Error("Verify failed", logger.Err(err), logger.Any("verification_id", req.VerificationID))
		switch {
		case errors.Is(err, ErrInvalidToken):
			c.JSON(http.StatusUnauthorized, ErrorResponse{
				Error:   "invalid_token",
				Message: "Invalid or expired verification code",
			})
		case errors.Is(err, ErrTokenExpired):
			c.JSON(http.StatusUnauthorized, ErrorResponse{
				Error:   "token_expired",
				Message: "Verification code has expired",
			})
		case errors.Is(err, ErrMaxAttemptsReached):
			c.JSON(http.StatusTooManyRequests, ErrorResponse{
				Error:   "max_attempts",
				Message: "Maximum attempts reached. Please request a new code.",
			})
		default:
			logger.Error("Verify failed",
				logger.Err(err),
			)
			c.JSON(http.StatusInternalServerError, ErrorResponse{
				Error:   "server_error",
				Message: "Failed to verify email",
			})
		}
		return
	}

	// Set refresh token cookie (same as login flow)
	h.setRefreshCookie(c, resp.RefreshToken, 30*24*60*60)

	c.JSON(http.StatusOK, resp)
}

// ResendVerification handles resending verification OTP for signup
//
//	@Summary		Resend signup verification code
//	@Description	Resends verification OTP for unverified users (signup flow)
//	@Tags			auth
//	@Accept			json
//	@Produce		json
//	@Param			request	body		ResendVerificationRequest	true	"Resend verification request"
//	@Success		200		{object}	ResendVerificationResponse	"Verification code sent"
//	@Failure		400		{object}	ErrorResponse				"Invalid request"
//	@Failure		429		{object}	ErrorResponse				"Cooldown active"
//	@Failure		500		{object}	ErrorResponse				"Server error"
//	@Router			/v1/auth/resend [post]
func (h *Handler) ResendVerification(c *gin.Context) {
	var req ResendVerificationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "invalid_request",
			Message: err.Error(),
		})
		return
	}

	resp, err := h.service.ResendVerification(c.Request.Context(), &req)
	if err != nil {
		if errors.Is(err, ErrCooldownNotPassed) {
			c.JSON(http.StatusTooManyRequests, ErrorResponse{
				Error:   "cooldown_active",
				Message: err.Error(),
			})
			return
		}
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "server_error",
			Message: "Failed to process request",
		})
		return
	}

	c.JSON(http.StatusOK, resp)
}

// ResendReset handles resending password reset email
//
//	@Summary		Resend password reset email
//	@Description	Resends password reset link for users who didn't receive it
//	@Tags			auth
//	@Accept			json
//	@Produce		json
//	@Param			request	body		ResendResetRequest	true	"Resend reset request"
//	@Success		200		{object}	ResendResetResponse	"Reset email sent"
//	@Failure		400		{object}	ErrorResponse		"Invalid request"
//	@Failure		429		{object}	ErrorResponse		"Cooldown active"
//	@Failure		500		{object}	ErrorResponse		"Server error"
//	@Router			/v1/auth/resend-reset [post]
func (h *Handler) ResendReset(c *gin.Context) {
	var req ResendResetRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "invalid_request",
			Message: err.Error(),
		})
		return
	}

	resp, err := h.service.ResendReset(c.Request.Context(), &req)
	if err != nil {
		if errors.Is(err, ErrCooldownNotPassed) {
			c.JSON(http.StatusTooManyRequests, ErrorResponse{
				Error:   "cooldown_active",
				Message: err.Error(),
			})
			return
		}
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "server_error",
			Message: "Failed to process request",
		})
		return
	}

	c.JSON(http.StatusOK, resp)
}

// ForgotPassword handles password reset request
//
//	@Summary		Request password reset
//	@Description	Sends password reset email with token link
//	@Tags			auth
//	@Accept			json
//	@Produce		json
//	@Param			request	body		ForgotPasswordRequest	true	"Forgot password request"
//	@Success		200		{object}	ForgotPasswordResponse	"Reset email sent"
//	@Failure		400		{object}	ErrorResponse			"Invalid request"
//	@Failure		500		{object}	ErrorResponse			"Server error"
//	@Router			/v1/auth/forgot-password [post]
func (h *Handler) ForgotPassword(c *gin.Context) {
	var req ForgotPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "invalid_request",
			Message: err.Error(),
		})
		return
	}

	resp, err := h.service.ForgotPassword(c.Request.Context(), &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "server_error",
			Message: "Failed to process request",
		})
		return
	}

	c.JSON(http.StatusOK, resp)
}

// ResetPassword handles password reset
//
//	@Summary		Reset password
//	@Description	Resets user password using token from email link
//	@Tags			auth
//	@Accept			json
//	@Produce		json
//	@Param			request	body		ResetPasswordRequest	true	"Reset password request"
//	@Success		200		{object}	ResetPasswordResponse	"Password reset successful"
//	@Failure		400		{object}	ErrorResponse			"Invalid request"
//	@Failure		401		{object}	ErrorResponse			"Invalid or expired token"
//	@Failure		429		{object}	ErrorResponse			"Max attempts reached"
//	@Failure		500		{object}	ErrorResponse			"Server error"
//	@Router			/v1/auth/reset-password [post]
func (h *Handler) ResetPassword(c *gin.Context) {
	var req ResetPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "invalid_request",
			Message: err.Error(),
		})
		return
	}

	resp, err := h.service.ResetPassword(c.Request.Context(), &req)
	if err != nil {
		switch {
		case errors.Is(err, ErrInvalidToken):
			c.JSON(http.StatusUnauthorized, ErrorResponse{
				Error:   "invalid_token",
				Message: "Invalid or expired reset code",
			})
		case errors.Is(err, ErrTokenExpired):
			c.JSON(http.StatusUnauthorized, ErrorResponse{
				Error:   "token_expired",
				Message: "Reset code has expired",
			})
		case errors.Is(err, ErrMaxAttemptsReached):
			c.JSON(http.StatusTooManyRequests, ErrorResponse{
				Error:   "max_attempts",
				Message: "Maximum attempts reached",
			})
		default:
			c.JSON(http.StatusInternalServerError, ErrorResponse{
				Error:   "server_error",
				Message: "Failed to reset password",
			})
		}
		return
	}

	c.JSON(http.StatusOK, resp)
}

// maskEmail hides most of email for safe logging
func maskEmail(email string) string {
	if len(email) < 3 {
		return "***"
	}
	parts := strings.Split(email, "@")
	if len(parts) != 2 {
		return "***"
	}
	local := parts[0]
	domain := parts[1]

	if len(local) > 2 {
		local = local[:2] + "***"
	} else {
		local = "**"
	}

	return local + "@" + domain
}

// InitVerify handles sending verification OTP to logged-in unverified users
//
//	@Summary		Initiate email verification
//	@Description	Sends new verification OTP to logged-in user who hasn't verified their email
//	@Tags			auth
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Success		200	{object}	InitVerifyResponse
//	@Failure		401	{object}	ErrorResponse	"Unauthorized"
//	@Failure		500	{object}	ErrorResponse	"Server error"
//	@Router			/v1/auth/init-verify [post]
func (h *Handler) InitVerify(c *gin.Context) {
	userID := middleware.GetUserID(c)
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Error:   "unauthorized",
			Message: "Authentication required",
		})
		return
	}

	resp, err := h.service.InitVerify(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "server_error",
			Message: "Failed to initiate verification",
		})
		return
	}

	c.JSON(http.StatusOK, resp)
}

// GetAllSessions returns all active sessions for the authenticated user
//
//	@Summary		Get all sessions
//	@Description	Returns all active sessions for the authenticated user. Accepts either Bearer token or magic link.
//	@Tags			auth
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			magic_link	query	string	false	"Magic link for session management (alternative to Bearer token)"
//	@Success		200		{object}	GetSessionsResponse
//	@Failure		401		{object}	ErrorResponse	"Unauthorized"
//	@Failure		500		{object}	ErrorResponse	"Server error"
//	@Router			/v1/auth/sessions [get]
func (h *Handler) GetAllSessions(c *gin.Context) {
	var userID int64

	// Check for magic link first
	magicLink := c.Query("magic_link")
	if magicLink != "" {
		magicLinkData, err := h.service.GetMagicLinkData(c.Request.Context(), magicLink)
		if err != nil || magicLinkData == nil {
			c.JSON(http.StatusUnauthorized, ErrorResponse{
				Error:   "invalid_magic_link",
				Message: "Invalid or expired magic link",
			})
			return
		}
		userID = magicLinkData.UserID
	} else {
		// Fall back to Bearer token
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
			c.JSON(http.StatusUnauthorized, ErrorResponse{
				Error:   "unauthorized",
				Message: "Authentication required",
			})
			return
		}

		tokenString := strings.TrimPrefix(authHeader, "Bearer ")
		claims, err := jwt.ValidateAccessToken(tokenString)
		if err != nil {
			c.JSON(http.StatusUnauthorized, ErrorResponse{
				Error:   "invalid_token",
				Message: "Invalid or expired token",
			})
			return
		}
		userID = claims.UserID
	}

	var resp *GetSessionsResponse
	var err error

	if magicLink != "" {
		// Magic link - all sessions will have IsCurrent = false
		resp, err = h.service.GetAllSessions(c.Request.Context(), userID, true)
	} else {
		// Bearer token - get refresh token from cookie to identify current session
		refreshToken, _ := c.Cookie("refreshToken")
		refreshTokenHash := hash.HashSHA256(refreshToken)
		resp, err = h.service.GetAllSessionsWithCurrent(c.Request.Context(), userID, refreshTokenHash)
	}

	if err != nil {
		logger.Error("GetAllSessions failed", logger.Err(err), logger.Int64("user_id", userID), logger.String("error_type", fmt.Sprintf("%T", err)))
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "server_error",
			Message: "Failed to get sessions",
		})
		return
	}

	c.JSON(http.StatusOK, resp)
}

// DeleteSession deletes a session by ID
//
//	@Summary		Delete session
//	@Description	Deletes a session by ID. If magic_link is provided, performs magic link flow.
//	@Tags			auth
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			request	body		DeleteSessionRequest	true	"Delete session request"
//	@Param			magic_link	query		string	false	"Magic link for session management (alternative to Bearer token)"
//	@Success		200		{object}	DeleteSessionResponse
//	@Failure		401		{object}	ErrorResponse	"Unauthorized"
//	@Failure		404		{object}	ErrorResponse	"Session not found"
//	@Failure		500		{object}	ErrorResponse	"Server error"
//	@Router			/v1/auth/sessions [delete]
func (h *Handler) DeleteSession(c *gin.Context) {
	var userID int64

	// Check for magic link first
	magicLink := c.Query("magic_link")
	if magicLink != "" {
		magicLinkData, err := h.service.GetMagicLinkData(c.Request.Context(), magicLink)
		if err != nil || magicLinkData == nil {
			c.JSON(http.StatusUnauthorized, ErrorResponse{
				Error:   "invalid_magic_link",
				Message: "Invalid or expired magic link",
			})
			return
		}
		userID = magicLinkData.UserID
	} else {
		// Fall back to Bearer token
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
			c.JSON(http.StatusUnauthorized, ErrorResponse{
				Error:   "unauthorized",
				Message: "Authentication required",
			})
			return
		}

		tokenString := strings.TrimPrefix(authHeader, "Bearer ")
		claims, err := jwt.ValidateAccessToken(tokenString)
		if err != nil {
			c.JSON(http.StatusUnauthorized, ErrorResponse{
				Error:   "invalid_token",
				Message: "Invalid or expired token",
			})
			return
		}
		userID = claims.UserID
	}

	var req DeleteSessionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "invalid_request",
			Message: err.Error(),
		})
		return
	}

	resp, err := h.service.DeleteSession(c.Request.Context(), userID, req.SessionID, magicLink, SessionParams{
		DeviceName: c.GetHeader("X-Device-Name"),
		UserAgent:  c.GetHeader("User-Agent"),
		IPAddress:  c.ClientIP(),
	})
	if err != nil {
		logger.Error("DeleteSession failed", logger.Err(err), logger.Int64("user_id", userID), logger.String("session_id", req.SessionID))
		if err == ErrInvalidToken {
			c.JSON(http.StatusUnauthorized, ErrorResponse{
				Error:   "invalid_token",
				Message: "Invalid or expired magic link",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "server_error",
			Message: "Failed to delete session",
		})
		return
	}

	// If new access token is returned (magic link flow), set new refresh cookie
	if resp.AccessToken != "" {
		h.setRefreshCookie(c, resp.RefreshToken, 30*24*60*60)
		resp.RefreshToken = ""
	}

	c.JSON(http.StatusOK, resp)
}
