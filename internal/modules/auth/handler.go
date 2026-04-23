package auth

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/trynafindbhumik/cinematch-backend/internal/config"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// setRefreshCookie sets the refresh token cookie with appropriate security settings
func (h *Handler) setRefreshCookie(c *gin.Context, token string, maxAge int) {
	secure := config.App.Environment == "production"
	sameSite := "Lax"
	if secure {
		sameSite = "Strict"
	}

	c.SetCookie(
		"refreshToken",
		token,
		maxAge,
		"/",
		"",
		secure,
		true, // httpOnly
	)
	// Set SameSite via header
	c.Header("Set-Cookie", fmt.Sprintf("refreshToken=%s; Path=/; Max-Age=%d; HttpOnly; SameSite=%s; Secure=%t",
		token, maxAge, sameSite, secure))
}

// Signup handles user registration
// @Summary Register a new user
// @Description Creates a new user account and sends verification OTP via email
// @Tags auth
// @Accept json
// @Produce json
// @Param request body SignupRequest true "Signup request"
// @Success 201 {object} SignupResponse "User created, verification required"
// @Success 409 {object} ErrorResponse "Email already exists"
// @Failure 400 {object} ErrorResponse "Invalid request"
// @Failure 500 {object} ErrorResponse "Server error"
// @Router /v1/auth/signup [post]
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
		// Log the actual error for debugging
		fmt.Printf("Signup error: %v\n", err)
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "server_error",
			Message: "Failed to process signup",
		})
		return
	}

	// Check if email already exists (indicated by empty verificationId)
	if resp.VerificationID == "" {
		c.JSON(http.StatusConflict, ErrorResponse{
			Error:   "email_exists",
			Message: "Email already exists",
		})
		return
	}

	c.JSON(http.StatusOK, resp)
}

// Verify handles email verification
// @Summary Verify email address
// @Description Verifies user email with OTP or token and creates a session
// @Tags auth
// @Accept json
// @Produce json
// @Param request body VerifyRequest true "Verification request"
// @Success 201 {object} VerifyResponse "Email verified, session created"
// @Failure 400 {object} ErrorResponse "Invalid request"
// @Failure 401 {object} ErrorResponse "Invalid OTP or token"
// @Failure 410 {object} ErrorResponse "OTP expired"
// @Failure 429 {object} ErrorResponse "Max attempts reached"
// @Failure 500 {object} ErrorResponse "Server error"
// @Router /v1/auth/verify [post]
func (h *Handler) Verify(c *gin.Context) {
	var req VerifyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "invalid_request",
			Message: err.Error(),
		})
		return
	}

	resp, err := h.service.Verify(c.Request.Context(), &req, SessionParams{
		DeviceName: c.GetHeader("X-Device-Name"),
		UserAgent:  c.GetHeader("User-Agent"),
		IPAddress:  c.ClientIP(),
	})
	if err != nil {
		switch {
		case errors.Is(err, ErrInvalidToken):
			if req.OTP != "" {
				c.JSON(http.StatusUnauthorized, ErrorResponse{
					Error:   "invalid_otp",
					Message: "Invalid OTP",
				})
			} else {
				c.JSON(http.StatusUnauthorized, ErrorResponse{
					Error:   "invalid_token",
					Message: "Invalid token",
				})
			}
		case errors.Is(err, ErrTokenExpired):
			c.JSON(http.StatusUnauthorized, ErrorResponse{
				Error:   "token_expired",
				Message: "OTP has expired",
			})
		case errors.Is(err, ErrMaxAttemptsReached):
			c.JSON(http.StatusTooManyRequests, ErrorResponse{
				Error:   "max_attempts",
				Message: "Maximum verification attempts reached",
			})
		default:
			c.JSON(http.StatusInternalServerError, ErrorResponse{
				Error:   "server_error",
				Message: "Failed to verify",
			})
		}
		return
	}

	h.setRefreshCookie(c, resp.RefreshToken, 7*24*60*60)
	// Don't expose refresh token in JSON
	resp.RefreshToken = ""

	c.JSON(http.StatusCreated, resp)
}

// Login handles user authentication
// @Summary Login user
// @Description Authenticates user with email and password, creates session
// @Tags auth
// @Accept json
// @Produce json
// @Param request body LoginRequest true "Login request"
// @Success 200 {object} LoginResponse "Login successful"
// @Failure 400 {object} ErrorResponse "Invalid request"
// @Failure 401 {object} ErrorResponse "Invalid credentials"
// @Failure 429 {object} ErrorResponse "Account locked"
// @Failure 500 {object} ErrorResponse "Server error"
// @Router /v1/auth/login [post]
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
		// Check if it's a lockout error
		if err.Error() == "account is temporarily locked" || err.Error() == "account is temporarily locked due to too many failed attempts" {
			c.JSON(http.StatusTooManyRequests, ErrorResponse{
				Error:   "account_locked",
				Message: "Too many failed attempts. Please try again later.",
			})
			return
		}
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Error:   "invalid_credentials",
			Message: "Invalid email or password",
		})
		return
	}

	h.setRefreshCookie(c, resp.RefreshToken, 7*24*60*60)

	c.JSON(http.StatusOK, resp)
}

// Logout handles user logout
// @Summary Logout user
// @Description Invalidates user session and clears refresh token cookie
// @Tags auth
// @Accept json
// @Produce json
// @Param request body LogoutRequest false "Logout request"
// @Success 204 "No content"
// @Router /v1/auth/logout [post]
func (h *Handler) Logout(c *gin.Context) {
	refreshToken, _ := c.Cookie("refreshToken")

	if refreshToken != "" {
		h.service.Logout(c.Request.Context(), refreshToken)
	}

	// Clear the cookie
	h.setRefreshCookie(c, "", -1)

	c.Status(http.StatusNoContent)
}

// ResendVerification handles resending verification OTP
// @Summary Resend verification code
// @Description Resends verification OTP for unverified users
// @Tags auth
// @Accept json
// @Produce json
// @Param request body ResendVerificationRequest true "Resend verification request"
// @Success 200 {object} ResendVerificationResponse "Verification code sent"
// @Failure 400 {object} ErrorResponse "Invalid request"
// @Failure 500 {object} ErrorResponse "Server error"
// @Router /v1/auth/resend [post]
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

// ForgotPassword handles password reset request
// @Summary Request password reset
// @Description Sends password reset email with token link
// @Tags auth
// @Accept json
// @Produce json
// @Param request body ForgotPasswordRequest true "Forgot password request"
// @Success 200 {object} ForgotPasswordResponse "Reset email sent"
// @Failure 400 {object} ErrorResponse "Invalid request"
// @Failure 500 {object} ErrorResponse "Server error"
// @Router /v1/auth/forgot-password [post]
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
// @Summary Reset password
// @Description Resets user password using token from email link
// @Tags auth
// @Accept json
// @Produce json
// @Param request body ResetPasswordRequest true "Reset password request"
// @Success 200 {object} ResetPasswordResponse "Password reset successful"
// @Failure 400 {object} ErrorResponse "Invalid request"
// @Failure 401 {object} ErrorResponse "Invalid or expired token"
// @Failure 429 {object} ErrorResponse "Max attempts reached"
// @Failure 500 {object} ErrorResponse "Server error"
// @Router /v1/auth/reset-password [post]
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