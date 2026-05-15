package profile

// Profile service handling user profile operations, email changes, and account management.

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/trynafindbhumik/cinematch-backend/internal/config"
	authmodels "github.com/trynafindbhumik/cinematch-backend/internal/models/auth"
	"github.com/trynafindbhumik/cinematch-backend/internal/shared/cloudinary"
	"github.com/trynafindbhumik/cinematch-backend/internal/shared/email"
	"github.com/trynafindbhumik/cinematch-backend/internal/shared/hash"
)

var (
	ErrInvalidConfirmText = errors.New("invalid confirmation text")
	ErrAccountDisabled    = errors.New("account is currently disabled")
	ErrEmailChangeExpired = errors.New("email change request has expired")
	ErrInvalidOTP         = errors.New("invalid or expired OTP")
	ErrMaxAttemptsReached = errors.New("maximum verification attempts reached")
	ErrCooldownNotPassed  = errors.New("cooldown not passed")
)

// AuthRepository interface for email verification operations
type AuthRepository interface {
	CreateEmailVerification(ctx context.Context, ev *authmodels.EmailVerification) (string, error)
	GetEmailVerification(ctx context.Context, id string) (*authmodels.EmailVerification, error)
	GetEmailVerificationByTokenWithID(ctx context.Context, tokenHash string) (*authmodels.EmailVerification, string, error)
	GetLatestEmailVerificationByUserID(ctx context.Context, userID int64) (*authmodels.EmailVerification, string, error)
	UpdateEmailVerificationAttempts(ctx context.Context, id string, attempts int) error
	DeleteEmailVerification(ctx context.Context, id string) error
	GetUserByID(ctx context.Context, userID int64) (*authmodels.User, error)
	SetUserUnverified(ctx context.Context, userID int64) error
	SetUserVerified(ctx context.Context, userID int64) error
	CreateSession(ctx context.Context, userID int64, refreshToken, jti, deviceName, userAgent, ipAddress string) (string, error)
	DeleteAllUserSessions(ctx context.Context, userID int64) error
}

type Service struct {
	repo     *Repository
	authRepo AuthRepository
}

func NewService(repo *Repository, authRepo AuthRepository) *Service {
	return &Service{
		repo:     repo,
		authRepo: authRepo,
	}
}

// Minimum time between disabling an account (1 week)
const disableCooldownDuration = 7 * 24 * time.Hour

// DisableAccount disables the user's account for a specified duration
func (s *Service) DisableAccount(ctx context.Context, userID int64, req *DisableAccountRequest) (*DisableAccountResponse, error) {
	if req.ConfirmText != "DISABLE" {
		return nil, ErrInvalidConfirmText
	}

	if req.DurationDays != 7 && req.DurationDays != 30 && req.DurationDays != 90 {
		return nil, errors.New("duration must be 7, 30, or 90 days")
	}

	user, err := s.repo.GetUserByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	if user.PreviouslyDisabledAt != nil {
		cooldownEnd := user.PreviouslyDisabledAt.Add(disableCooldownDuration)
		if time.Now().Before(cooldownEnd) {
			remaining := time.Until(cooldownEnd).Hours() / 24
			return nil, fmt.Errorf("cannot disable account again for %.0f days. Please wait %d days", remaining, int(remaining)+1)
		}
	}

	disabledUntil := time.Now().AddDate(0, 0, req.DurationDays)

	if err := s.repo.DisableAccount(ctx, userID, disabledUntil); err != nil {
		return nil, fmt.Errorf("failed to disable account: %w", err)
	}

	s.repo.DeleteAllUserSessions(ctx, userID)

	return &DisableAccountResponse{
		Message:       fmt.Sprintf("Account disabled until %s", disabledUntil.Format("2006-01-02")),
		DisabledUntil: disabledUntil.Unix(),
	}, nil
}

// GetProfile returns the current user's profile
func (s *Service) GetProfile(ctx context.Context, userID int64) (*GetProfileResponse, error) {
	user, err := s.repo.GetUserByID(ctx, userID)
	if err != nil {
		return nil, err
	}

	return &GetProfileResponse{
		ID:           user.ID,
		PublicID:     user.PublicID,
		Name:         user.Name,
		Email:        user.Email,
		ProfileURL:   user.ProfileURL,
		IsVerified:   user.IsVerified,
		Tag:          user.Tag,
		SmartSuggest: user.SmartSuggest,
	}, nil
}

// UpdateProfile updates the user's name and/or profile picture
func (s *Service) UpdateProfile(ctx context.Context, userID int64, userPublicID string, req *UpdateProfileRequest) (*UpdateProfileResponse, error) {
	var profileURL *string
	if len(req.ProfilePicture) > 0 {
		url, err := cloudinary.UploadProfilePicture(ctx, req.ProfilePicture, userPublicID)
		if err != nil {
			return nil, fmt.Errorf("failed to upload profile picture: %w", err)
		}
		profileURL = &url
	}

	if req.Name == "" && profileURL == nil && req.SmartSuggest == nil {
		return nil, ErrNoChangesDetected
	}

	if err := s.repo.UpdateProfile(ctx, userID, req.Name, profileURL, req.SmartSuggest); err != nil {
		return nil, fmt.Errorf("failed to update profile: %w", err)
	}

	user, _ := s.repo.GetUserByID(ctx, userID)

	return &UpdateProfileResponse{
		Message:      "Profile updated successfully",
		ProfileURL:   user.ProfileURL,
		Name:         user.Name,
		SmartSuggest: &user.SmartSuggest,
	}, nil
}

// DeleteProfilePicture removes the user's profile picture
func (s *Service) DeleteProfilePicture(ctx context.Context, userID int64) (*DeleteProfilePictureResponse, error) {
	user, err := s.repo.GetUserByID(ctx, userID)
	if err != nil {
		return nil, err
	}

	if user.ProfileURL == nil || *user.ProfileURL == "" {
		return nil, fmt.Errorf("no profile picture to delete")
	}

	if err := s.repo.ClearProfilePicture(ctx, userID); err != nil {
		return nil, fmt.Errorf("failed to delete profile picture: %w", err)
	}

	return &DeleteProfilePictureResponse{
		Message: "Profile picture deleted successfully",
	}, nil
}

// ChangePassword verifies old password and updates to new password
func (s *Service) ChangePassword(ctx context.Context, userID int64, req *ChangePasswordRequest) (*ChangePasswordResponse, error) {
	if !hash.CheckPassword(req.OldPassword, s.getPasswordHash(ctx, userID)) {
		return nil, ErrInvalidPassword
	}

	newPasswordHash, err := hash.HashPassword(req.NewPassword)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	if err := s.repo.UpdatePassword(ctx, userID, newPasswordHash); err != nil {
		return nil, fmt.Errorf("failed to update password: %w", err)
	}

	if req.LogoutFromAllDevices {
		s.repo.DeleteAllUserSessions(ctx, userID)
	}

	return &ChangePasswordResponse{
		Message: "Password changed successfully",
	}, nil
}

func (s *Service) getPasswordHash(ctx context.Context, userID int64) string {
	hash, _ := s.repo.GetUserPasswordHash(ctx, userID)
	return hash
}

// InitiateEmailChange starts the email change flow by sending OTP to old email
func (s *Service) InitiateEmailChange(ctx context.Context, userID int64, newEmail string) (*EmailChangeStepResponse, error) {
	exists, err := s.repo.CheckEmailExists(ctx, newEmail, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to check email: %w", err)
	}
	if exists {
		return nil, ErrEmailAlreadyExists
	}

	user, err := s.repo.GetUserByID(ctx, userID)
	if err != nil {
		return nil, err
	}

	// Delete any existing email change verifications
	if existingEv, existingID, err := s.authRepo.GetLatestEmailVerificationByUserID(ctx, userID); err == nil {
		if existingEv.Type == authmodels.VerificationTypeEmailChangeOld || existingEv.Type == authmodels.VerificationTypeEmailChangeNew {
			s.authRepo.DeleteEmailVerification(ctx, existingID)
		}
	}

	otp, err := hash.GenerateOTP()
	if err != nil {
		return nil, fmt.Errorf("failed to generate OTP: %w", err)
	}

	ev := &authmodels.EmailVerification{
		Email:     newEmail,
		UserID:    userID,
		Type:      authmodels.VerificationTypeEmailChangeOld,
		OTPHash:   hash.HashSHA256(otp),
		ExpiresAt: time.Now().Add(time.Duration(config.Auth.OTPExpiry) * time.Minute),
	}

	_, err = s.authRepo.CreateEmailVerification(ctx, ev)
	if err != nil {
		return nil, fmt.Errorf("failed to create email verification: %w", err)
	}

	email.SendEmailAsync(email.EmailData{
		To:      user.Email,
		Subject: "Verify your old email for email change",
		Body:    fmt.Sprintf("<h1>Email Change Request</h1><p>Your OTP to verify your old email is: <strong>%s</strong></p><p>This code expires in 15 minutes.</p>", otp),
	})

	return &EmailChangeStepResponse{
		Message:   "OTP sent to your current email",
		Step:      "otp_sent_to_old",
		ExpiresAt: ev.ExpiresAt.Unix(),
	}, nil
}

// DeleteAccount soft deletes the user's account
func (s *Service) DeleteAccount(ctx context.Context, userID int64, req *DeleteAccountRequest) (*DeleteAccountResponse, error) {
	if req.ConfirmText != "DELETE" {
		return nil, ErrInvalidConfirmText
	}

	if err := s.repo.SoftDeleteAccount(ctx, userID); err != nil {
		return nil, fmt.Errorf("failed to delete account: %w", err)
	}

	s.repo.DeleteAllUserSessions(ctx, userID)

	return &DeleteAccountResponse{
		Message: "Account scheduled for deletion. Your data will be retained for 30 days before permanent deletion.",
	}, nil
}

// CleanupDeletedAccounts permanently deletes accounts that have been soft-deleted beyond retention period
func (s *Service) CleanupDeletedAccounts(ctx context.Context, retentionDays int) (int64, error) {
	return s.repo.CleanupDeletedAccounts(ctx, retentionDays)
}

// VerifyEmail verifies email using JWT lookup + OTP
func (s *Service) VerifyEmail(ctx context.Context, userID int64, req *VerifyEmailRequest) (*VerifyEmailResponse, error) {
	if req.OTP == "" {
		return nil, ErrInvalidOTP
	}

	// Get latest verification for user (JWT lookup)
	ev, verificationID, err := s.authRepo.GetLatestEmailVerificationByUserID(ctx, userID)
	if err != nil || verificationID == "" {
		return nil, ErrInvalidOTP
	}

	// Check if it's a valid email change type
	if ev.Type != authmodels.VerificationTypeEmailChangeOld && ev.Type != authmodels.VerificationTypeEmailChangeNew {
		return nil, ErrInvalidOTP
	}

	// Check expiration
	if time.Now().After(ev.ExpiresAt) {
		return nil, ErrEmailChangeExpired
	}

	// Check max attempts
	if ev.Attempts >= config.Auth.OTPMaxAttempts {
		return nil, ErrMaxAttemptsReached
	}

	// Verify OTP
	otpHash := hash.HashSHA256(req.OTP)
	if ev.OTPHash != otpHash {
		s.authRepo.UpdateEmailVerificationAttempts(ctx, verificationID, ev.Attempts+1)
		return nil, ErrInvalidOTP
	}

	// Delete the verification
	s.authRepo.DeleteEmailVerification(ctx, verificationID)

	// Handle based on verification type
	if ev.Type == authmodels.VerificationTypeEmailChangeOld {
		return s.handleEmailChangeOldVerification(ctx, userID, ev)
	}

	// email_change_new
	return s.handleEmailChangeNewVerification(ctx, userID)
}

// handleEmailChangeOldVerification handles verification of old email in email change flow
func (s *Service) handleEmailChangeOldVerification(ctx context.Context, userID int64, ev *authmodels.EmailVerification) (*VerifyEmailResponse, error) {
	newEmail := ev.Email

	// Update the user's email to the new email
	if err := s.repo.UpdateUserEmail(ctx, userID, newEmail); err != nil {
		return nil, fmt.Errorf("failed to update email: %w", err)
	}

	// Set user as unverified so they need to verify the new email
	s.authRepo.SetUserUnverified(ctx, userID)

	// Invalidate all sessions
	s.repo.DeleteAllUserSessions(ctx, userID)

	// Send OTP to the NEW email for verification (Step 2)
	newOtp, err := hash.GenerateOTP()
	if err != nil {
		return nil, fmt.Errorf("failed to generate OTP for new email: %w", err)
	}

	newEv := &authmodels.EmailVerification{
		Email:     newEmail,
		UserID:    userID,
		Type:      authmodels.VerificationTypeEmailChangeNew,
		OTPHash:   hash.HashSHA256(newOtp),
		ExpiresAt: time.Now().Add(time.Duration(config.Auth.OTPExpiry) * time.Minute),
	}

	_, err = s.authRepo.CreateEmailVerification(ctx, newEv)
	if err != nil {
		return nil, fmt.Errorf("failed to create email verification for new email: %w", err)
	}

	email.SendEmailAsync(email.EmailData{
		To:      newEmail,
		Subject: "Verify your new email for email change",
		Body:    fmt.Sprintf("<h1>Email Change Request</h1><p>Your OTP to verify your new email is: <strong>%s</strong></p><p>This code expires in 15 minutes.</p>", newOtp),
	})

	return &VerifyEmailResponse{
		Message:    "Email updated. Please verify your new email.",
		Step:       "otp_sent_to_new",
		IsVerified: false,
		NewEmail:   newEmail,
	}, nil
}

// handleEmailChangeNewVerification handles verification of new email (final step)
func (s *Service) handleEmailChangeNewVerification(ctx context.Context, userID int64) (*VerifyEmailResponse, error) {
	s.authRepo.SetUserVerified(ctx, userID)

	return &VerifyEmailResponse{
		Message:    "Email verified successfully",
		Step:       "email_updated",
		IsVerified: true,
	}, nil
}

// ResendEmailChange resends OTP for email change flow with cooldown
func (s *Service) ResendEmailChange(ctx context.Context, userID int64) (*ResendEmailChangeResponse, error) {
	user, err := s.repo.GetUserByID(ctx, userID)
	if err != nil {
		return nil, err
	}

	// Get latest email change verification if any
	ev, verificationID, err := s.authRepo.GetLatestEmailVerificationByUserID(ctx, userID)
	if err != nil {
		return s.initiateEmailChangeResend(ctx, userID, user.Email)
	}

	// Check if it's a valid email change type
	if ev.Type != authmodels.VerificationTypeEmailChangeOld && ev.Type != authmodels.VerificationTypeEmailChangeNew {
		return s.initiateEmailChangeResend(ctx, userID, user.Email)
	}

	// Check cooldown
	cooldownEnd := ev.CreatedAt.Add(time.Duration(config.Auth.ResendCooldown) * time.Second)
	if time.Now().Before(cooldownEnd) {
		remaining := time.Until(cooldownEnd).Seconds()
		return nil, fmt.Errorf("%w: please wait %.0f seconds before requesting a new code", ErrCooldownNotPassed, remaining)
	}

	// Check if expired (allow resend within 5 minute grace period)
	if time.Now().After(ev.ExpiresAt) && time.Since(ev.ExpiresAt) > 5*time.Minute {
		newEmail := ev.Email
		s.authRepo.DeleteEmailVerification(ctx, verificationID)
		return s.initiateEmailChangeResend(ctx, userID, newEmail)
	}

	// If verification is still valid, resend OTP based on step
	if ev.Type == authmodels.VerificationTypeEmailChangeOld {
		return s.resendOTPForStep(ctx, userID, user.Email, "otp_sent_to_old")
	}

	return s.resendOTPForStep(ctx, userID, ev.Email, "otp_sent_to_new")
}

// initiateEmailChangeResend starts a new email change flow when no valid verification exists
func (s *Service) initiateEmailChangeResend(ctx context.Context, userID int64, currentEmail string) (*ResendEmailChangeResponse, error) {
	// Delete any existing email change verifications
	if existingEv, existingID, err := s.authRepo.GetLatestEmailVerificationByUserID(ctx, userID); err == nil {
		if existingEv.Type == authmodels.VerificationTypeEmailChangeOld || existingEv.Type == authmodels.VerificationTypeEmailChangeNew {
			s.authRepo.DeleteEmailVerification(ctx, existingID)
		}
	}

	otp, err := hash.GenerateOTP()
	if err != nil {
		return nil, fmt.Errorf("failed to generate OTP: %w", err)
	}

	ev := &authmodels.EmailVerification{
		Email:     currentEmail, // Will be replaced with new email after step 1
		UserID:    userID,
		Type:      authmodels.VerificationTypeEmailChangeOld,
		OTPHash:   hash.HashSHA256(otp),
		ExpiresAt: time.Now().Add(time.Duration(config.Auth.OTPExpiry) * time.Minute),
	}

	_, err = s.authRepo.CreateEmailVerification(ctx, ev)
	if err != nil {
		return nil, fmt.Errorf("failed to create email verification: %w", err)
	}

	email.SendEmailAsync(email.EmailData{
		To:      currentEmail,
		Subject: "Resend: Verify your email for email change",
		Body:    fmt.Sprintf("<h1>Email Change Request</h1><p>Your OTP to verify your email is: <strong>%s</strong></p><p>This code expires in 15 minutes.</p>", otp),
	})

	return &ResendEmailChangeResponse{
		Message:   fmt.Sprintf("OTP resent to your current email"),
		Step:      "otp_sent_to_old",
		ExpiresAt: ev.ExpiresAt.Unix(),
	}, nil
}

// resendOTPForStep resends OTP for an existing verification
func (s *Service) resendOTPForStep(ctx context.Context, userID int64, emailToSend string, step string) (*ResendEmailChangeResponse, error) {
	// Get current verification to delete it
	existingEv, existingID, err := s.authRepo.GetLatestEmailVerificationByUserID(ctx, userID)
	if err == nil && existingID != "" {
		s.authRepo.DeleteEmailVerification(ctx, existingID)
	}
	_ = existingEv // suppress unused

	otp, err := hash.GenerateOTP()
	if err != nil {
		return nil, fmt.Errorf("failed to generate OTP: %w", err)
	}

	var evType string
	if step == "otp_sent_to_old" {
		evType = authmodels.VerificationTypeEmailChangeOld
	} else {
		evType = authmodels.VerificationTypeEmailChangeNew
	}

	ev := &authmodels.EmailVerification{
		Email:     emailToSend,
		UserID:    userID,
		Type:      evType,
		OTPHash:   hash.HashSHA256(otp),
		ExpiresAt: time.Now().Add(time.Duration(config.Auth.OTPExpiry) * time.Minute),
	}

	_, err = s.authRepo.CreateEmailVerification(ctx, ev)
	if err != nil {
		return nil, fmt.Errorf("failed to create email verification: %w", err)
	}

	email.SendEmailAsync(email.EmailData{
		To:      emailToSend,
		Subject: "Resend: Verify your email",
		Body:    fmt.Sprintf("<h1>Email Verification</h1><p>Your OTP is: <strong>%s</strong></p><p>This code expires in 15 minutes.</p>", otp),
	})

	return &ResendEmailChangeResponse{
		Message:   fmt.Sprintf("OTP resent to %s", emailToSend),
		Step:      step,
		ExpiresAt: ev.ExpiresAt.Unix(),
	}, nil
}
