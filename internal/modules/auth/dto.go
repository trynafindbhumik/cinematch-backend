package auth

// Request and response DTOs for auth endpoints.

type SignupRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=8"`
	Name     string `json:"name" binding:"required,min=2"`
}

type SignupResponse struct {
	Message        string `json:"message"`
	VerificationID string `json:"verification_id"`
}

type VerifyRequest struct {
	VerificationID string `json:"verification_id,omitempty"`
	OTP            string `json:"otp,omitempty"`
	Token          string `json:"token,omitempty"`
}

type VerifyResponse struct {
	AccessToken     string `json:"access_token"`
	RefreshToken    string `json:"-"`
	IsVerified      bool   `json:"is_verified"`
	IsFirstLogin     bool   `json:"is_first_login"`
	NeedsOnboarding bool   `json:"needs_onboarding"`
	Message         string `json:"message"`
}

type LoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

type LoginResponse struct {
	AccessToken        string `json:"access_token"`
	RefreshToken       string `json:"-"` // Set as cookie by handler
	IsVerified         bool   `json:"is_verified"`
	IsFirstLogin       bool   `json:"is_first_login"`
	NeedsOnboarding    bool   `json:"needs_onboarding"`
	IsSessionExhausted bool   `json:"is_session_exhausted"`
	MagicLink          string `json:"magic_link"`
}

type LogoutRequest struct {
}

type RefreshTokenRequest struct {
	RefreshToken string `json:"refreshToken,omitempty"`
}

type RefreshTokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"-"` // Set as cookie by handler
}

type ResendVerificationRequest struct {
	Email string `json:"email" binding:"required,email"`
}

type ResendVerificationResponse struct {
	Message        string `json:"message"`
	VerificationID string `json:"verification_id,omitempty"`
	ExpiresAt      int64  `json:"expires_at,omitempty"`
}

type ResendResetRequest struct {
	Email string `json:"email" binding:"required,email"`
}

type ResendResetResponse struct {
	Message string `json:"message"`
}

type ForgotPasswordRequest struct {
	Email string `json:"email" binding:"required,email"`
}

type ForgotPasswordResponse struct {
	Message string `json:"message"`
}

type ResetPasswordRequest struct {
	Token       string `json:"token" binding:"required"`
	NewPassword string `json:"new_password" binding:"required,min=8"`
}

type ResetPasswordResponse struct {
	Message string `json:"message"`
}

// SessionParams holds device/session information for tracking
type SessionParams struct {
	DeviceName string
	UserAgent  string
	IPAddress  string
}

type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message,omitempty"`
}

type InitVerifyRequest struct {
	Token string `json:"token" binding:"required"`
}

type InitVerifyResponse struct {
	Message        string `json:"message"`
	Email          string `json:"email"`
	VerificationID string `json:"verification_id,omitempty"`
	IsVerified      bool   `json:"is_verified"`
}

// Session DTOs

type SessionResponse struct {
	ID         string `json:"id"`
	DeviceName string `json:"device_name"`
	UserAgent  string `json:"user_agent,omitempty"`
	CreatedAt  string `json:"created_at"`
	LastLogin  string `json:"last_login"`
	IsCurrent  bool   `json:"is_current"`
}

type GetSessionsResponse struct {
	Sessions []SessionResponse `json:"sessions"`
	Total    int               `json:"total"`
}

type DeleteSessionRequest struct {
	SessionID string `json:"session_id" binding:"required"`
	MagicLink string `json:"magic_link,omitempty"`
}

type DeleteSessionResponse struct {
	Message         string `json:"message"`
	AccessToken    string `json:"access_token,omitempty"`
	RefreshToken   string `json:"-"` // Set as cookie by handler
	NeedsOnboarding bool   `json:"needs_onboarding,omitempty"`
}
