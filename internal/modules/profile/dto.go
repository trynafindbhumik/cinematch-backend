package profile

// GetProfileResponse is returned when getting current user's profile
type GetProfileResponse struct {
	ID           int64   `json:"id"`
	PublicID     string  `json:"publicId"`
	Name         string  `json:"name"`
	Email        string  `json:"email"`
	ProfileURL   *string `json:"profileUrl"`
	IsVerified   bool    `json:"isVerified"`
	Tag          string  `json:"tag"` // screen_enthusiast, cinema_lover, cinephile, cinephile_pro, cinephile_elite
	SmartSuggest bool    `json:"smartSuggest"`
}

// UpdateProfileRequest is used to update name and/or profile picture
type UpdateProfileRequest struct {
	Name           string `json:"name,omitempty"`
	ProfilePicture []byte `json:"-"` // Binary data from FormData
	SmartSuggest   *bool  `json:"smartSuggest,omitempty"`
}

// UpdateProfileResponse is returned after successful profile update
type UpdateProfileResponse struct {
	Message      string  `json:"message"`
	ProfileURL   *string `json:"profileUrl,omitempty"`
	Name         string  `json:"name,omitempty"`
	SmartSuggest *bool   `json:"smartSuggest,omitempty"`
}

// Change Password
type ChangePasswordRequest struct {
	OldPassword         string `json:"oldPassword" binding:"required"`
	NewPassword         string `json:"newPassword" binding:"required,min=8"`
	LogoutFromAllDevices bool   `json:"logoutFromAllDevices"`
}

type ChangePasswordResponse struct {
	Message string `json:"message"`
}

// Email Change Flow - Step 1: Initiate (OTP sent to old email)
type InitiateEmailChangeRequest struct {
	NewEmail string `json:"newEmail" binding:"required,email"`
}

// ResendEmailChangeResponse returned after resending OTP
type ResendEmailChangeResponse struct {
	Message   string `json:"message"`
	Step      string `json:"step"` // "otp_sent_to_old" or "otp_sent_to_new"
	ExpiresAt int64  `json:"expiresAt"`
}

// Email change step response
type EmailChangeStepResponse struct {
	Message   string `json:"message"`
	Step      string `json:"step"` // "otp_sent_to_old", "otp_sent_to_new", "email_updated"
	NewEmail  string `json:"newEmail,omitempty"`
	ExpiresAt int64  `json:"expiresAt,omitempty"`
}

// Disable Account
type DisableAccountRequest struct {
	ConfirmText  string `json:"confirmText" binding:"required"`
	DurationDays int    `json:"durationDays" binding:"required,oneof=7 30 90"` // 7, 30, or 90 days
}

type DisableAccountResponse struct {
	Message       string `json:"message"`
	DisabledUntil int64  `json:"disabledUntil"` // Unix timestamp
}

// Delete Account
type DeleteAccountRequest struct {
	ConfirmText string `json:"confirmText" binding:"required"`
}

type DeleteAccountResponse struct {
	Message string `json:"message"`
}

// Error response
type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message,omitempty"`
}

// DeleteProfilePictureResponse is returned after deleting profile picture
type DeleteProfilePictureResponse struct {
	Message string `json:"message"`
}

// VerifyEmailRequest - JWT lookup, OTP required
type VerifyEmailRequest struct {
	OTP string `json:"otp" binding:"required"`
}

// VerifyEmailResponse is returned after successful email verification
type VerifyEmailResponse struct {
	Message    string `json:"message"`
	Step       string `json:"step,omitempty"` // "otp_sent_to_new", "email_updated"
	IsVerified bool   `json:"isVerified"`
	NewEmail   string `json:"newEmail,omitempty"` // For email_change_old flow
}
