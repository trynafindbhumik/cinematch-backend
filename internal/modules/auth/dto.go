package auth

// Request/Response DTOs (API contracts)

type SignupRequest struct {
	Name     string `json:"name" binding:"required"`
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=8"`
}

type SignupResponse struct {
	Message        string `json:"message"`
	VerificationID string `json:"verificationId,omitempty"`
}

// SessionParams holds device/session information for tracking
type SessionParams struct {
	DeviceName string
	UserAgent  string
	IPAddress  string
}

type VerifyRequest struct {
	VerificationID string `json:"verificationId" binding:"required"`
	Token          string `json:"token,omitempty"`
	OTP            string `json:"otp,omitempty"`
}

type VerifyResponse struct {
	AccessToken  string `json:"accessToken"`
	RefreshToken string `json:"-"` // Set as cookie by handler
}

type LoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

type LoginResponse struct {
	AccessToken  string `json:"accessToken"`
	RefreshToken string `json:"-"` // Set as cookie by handler
}

type LogoutRequest struct {
	RefreshToken string `json:"refreshToken,omitempty"`
}

type ResendVerificationRequest struct {
	Email string `json:"email" binding:"required,email"`
}

type ResendVerificationResponse struct {
	Message        string `json:"message"`
	VerificationID string `json:"verificationId,omitempty"`
}

type ForgotPasswordRequest struct {
	Email string `json:"email" binding:"required,email"`
}

type ForgotPasswordResponse struct {
	Message string `json:"message"`
}

type ResetPasswordRequest struct {
	VerificationID string `json:"verificationId"`
	Token          string `json:"token" binding:"required"`
	NewPassword    string `json:"newPassword" binding:"required,min=8"`
}

type ResetPasswordResponse struct {
	Message string `json:"message"`
}

type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message,omitempty"`
}