package auth

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/trynafindbhumik/cinematch-backend/internal/config"
	authmodels "github.com/trynafindbhumik/cinematch-backend/internal/models/auth"
	"github.com/trynafindbhumik/cinematch-backend/internal/shared/email"
	"github.com/trynafindbhumik/cinematch-backend/internal/shared/hash"
	"github.com/trynafindbhumik/cinematch-backend/internal/shared/jwt"
)

var (
	ErrInvalidToken       = errors.New("invalid token")
	ErrTokenExpired       = errors.New("token expired")
	ErrMaxAttemptsReached = errors.New("max attempts reached")
	ErrCooldownNotPassed  = errors.New("cooldown not passed")
)

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

// Signup creates a new user and sends verification OTP
func (s *Service) Signup(ctx context.Context, req *SignupRequest) (*SignupResponse, error) {
	// Check if user already exists
	existingUser, _ := s.repo.GetUserByEmail(ctx, req.Email)
	if existingUser != nil {
		return &SignupResponse{
			Message:        "Email already registered",
			VerificationID: "",
		}, nil
	}

	// Hash password
	passwordHash, err := hash.HashPassword(req.Password)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	// Create user (unverified)
	user, err := s.repo.CreateUser(ctx, req.Email, passwordHash, req.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	// Generate OTP
	otp, err := hash.GenerateOTP()
	if err != nil {
		return nil, fmt.Errorf("failed to generate OTP: %w", err)
	}

	// Generate verification token (for link)
	rawToken, err := hash.GenerateToken()
	if err != nil {
		return nil, fmt.Errorf("failed to generate token: %w", err)
	}

	// Create verification record with both OTP and token hashes
	ev := &authmodels.EmailVerification{
		Email:     req.Email,
		UserID:    user.ID,
		Type:      authmodels.VerificationTypeSignup,
		OTPHash:   hash.HashSHA256(otp),
		TokenHash: hash.HashSHA256(rawToken),
		ExpiresAt: time.Now().Add(time.Duration(config.Auth.OTPExpiry) * time.Minute),
	}

	verificationID, err := s.repo.CreateEmailVerification(ctx, ev)
	if err != nil {
		return nil, fmt.Errorf("failed to create verification: %w", err)
	}

	// Convert ID to string for verification ID
	vid := fmt.Sprintf("%d", verificationID)

	// Send verification email with OTP and link (async - don't block on email)
	email.SendEmailAsync(email.EmailData{
		To:      req.Email,
		Subject: "Verify your CineMatch account",
		Body:    fmt.Sprintf("<h1>Welcome to CineMatch!</h1><p>Your verification code is: <strong>%s</strong></p><p>Or click here to verify: <a href=\"https://cinematch.com/verify?id=%s&token=%s\">Verify Email</a></p><p>This code expires in 15 minutes.</p>", otp, vid, rawToken),
	})

	return &SignupResponse{
		Message:        "Verification required",
		VerificationID: vid,
	}, nil
}

// Verify verifies email with OTP or token and creates session
func (s *Service) Verify(ctx context.Context, req *VerifyRequest, sessionParams SessionParams) (*VerifyResponse, error) {
	// Parse verification ID
	verificationID, err := strconv.ParseInt(req.VerificationID, 10, 64)
	if err != nil {
		return nil, ErrInvalidToken
	}

	// Get verification record
	ev, err := s.repo.GetEmailVerification(ctx, verificationID)
	if err != nil {
		return nil, ErrInvalidToken
	}

	// Check if already used
	if ev.UsedAt != nil {
		return nil, ErrInvalidToken
	}

	// Check expiration
	if time.Now().After(ev.ExpiresAt) {
		return nil, ErrTokenExpired
	}

	// Check max attempts
	if ev.Attempts >= config.Auth.OTPMaxAttempts {
		return nil, ErrMaxAttemptsReached
	}

	// Verify using OTP or Token
	verified := false
	if req.OTP != "" {
		otpHash := hash.HashSHA256(req.OTP)
		if ev.OTPHash == otpHash {
			verified = true
		}
	} else if req.Token != "" {
		tokenHash := hash.HashSHA256(req.Token)
		if ev.TokenHash == tokenHash {
			verified = true
		}
	}

	if !verified {
		// Increment attempts
		s.repo.UpdateEmailVerificationAttempts(ctx, verificationID, ev.Attempts+1)
		return nil, ErrInvalidToken
	}

	// Mark verification as used
	s.repo.MarkEmailVerificationUsed(ctx, verificationID)

	// Set user as verified
	s.repo.SetUserVerified(ctx, ev.UserID)

	// Get user
	user, err := s.repo.GetUserByID(ctx, ev.UserID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	// Generate access token with jti (include is_verified and is_first_login)
	accessToken, _, err := jwt.GenerateAccessToken(user.ID, user.Email, user.IsVerified, user.IsFirstLogin)
	if err != nil {
		return nil, fmt.Errorf("failed to generate access token: %w", err)
	}

	// Generate random refresh token (opaque, not JWT)
	refreshToken, err := jwt.GenerateRandomRefreshToken()
	if err != nil {
		return nil, fmt.Errorf("failed to generate refresh token: %w", err)
	}

	// Create session with device info
	sessionID, err := s.repo.CreateSession(ctx, user.ID, refreshToken, sessionParams.DeviceName, sessionParams.UserAgent, sessionParams.IPAddress)
	if err != nil {
		return nil, fmt.Errorf("failed to create session: %w", err)
	}
	fmt.Printf("Session created: %s for user %d\n", sessionID, user.ID)

	// Delete verification record
	s.repo.DeleteEmailVerification(ctx, verificationID)

	return &VerifyResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
	}, nil
}

// Login authenticates user and creates session
func (s *Service) Login(ctx context.Context, req *LoginRequest, sessionParams SessionParams) (*LoginResponse, error) {
	// Get user by email
	user, err := s.repo.GetUserByEmail(ctx, req.Email)
	if err != nil {
		return nil, fmt.Errorf("invalid credentials")
	}

	// Check if account is locked
	if user.LockoutUntil != nil && time.Now().Before(*user.LockoutUntil) {
		return nil, fmt.Errorf("account is temporarily locked")
	}

	// Check password
	if !hash.CheckPassword(req.Password, user.PasswordHash) {
		// Increment failed attempts
		s.repo.IncrementFailedAttempts(ctx, user.ID)

		// Check if should lockout
		if user.FailedAttempts+1 >= 5 {
			lockoutUntil := time.Now().Add(time.Duration(config.Auth.LockoutDuration) * time.Minute)
			s.repo.LockoutUser(ctx, user.ID, lockoutUntil)
			return nil, fmt.Errorf("account is temporarily locked due to too many failed attempts")
		}
		return nil, fmt.Errorf("invalid credentials")
	}

	// Reset failed attempts on successful login
	s.repo.ResetFailedAttempts(ctx, user.ID)

	// Generate access token with jti (include is_verified and is_first_login)
	accessToken, _, err := jwt.GenerateAccessToken(user.ID, user.Email, user.IsVerified, user.IsFirstLogin)
	if err != nil {
		return nil, fmt.Errorf("failed to generate access token: %w", err)
	}

	// Generate random refresh token
	refreshToken, err := jwt.GenerateRandomRefreshToken()
	if err != nil {
		return nil, fmt.Errorf("failed to generate refresh token: %w", err)
	}

	// Create session with device info
	_, err = s.repo.CreateSession(ctx, user.ID, refreshToken, sessionParams.DeviceName, sessionParams.UserAgent, sessionParams.IPAddress)
	if err != nil {
		return nil, fmt.Errorf("failed to create session: %w", err)
	}

	return &LoginResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
	}, nil
}

// Logout invalidates the session
func (s *Service) Logout(ctx context.Context, refreshToken string) error {
	if refreshToken == "" {
		return nil // Allow logout without token (just clear cookie)
	}
	return s.repo.DeleteSessionByRefreshToken(ctx, refreshToken)
}

// ResendVerification resends verification OTP for unverified users
func (s *Service) ResendVerification(ctx context.Context, req *ResendVerificationRequest) (*ResendVerificationResponse, error) {
	// Get user by email
	user, err := s.repo.GetUserByEmail(ctx, req.Email)
	if err != nil {
		// Return success to prevent email enumeration
		return &ResendVerificationResponse{
			Message:        "Verification code sent",
			VerificationID: "",
		}, nil
	}

	// Check if already verified
	if user.IsVerified {
		return &ResendVerificationResponse{
			Message:        "Verification code sent",
			VerificationID: "",
		}, nil
	}

	// Check cooldown - get most recent verification record
	existingEv, err := s.repo.GetLatestEmailVerificationByUserID(ctx, user.ID)
	if err == nil && existingEv != nil {
		cooldownEnd := existingEv.CreatedAt.Add(time.Duration(config.Auth.ResendCooldown) * time.Second)
		if time.Now().Before(cooldownEnd) {
			remaining := time.Until(cooldownEnd).Seconds()
			return nil, fmt.Errorf("%w: please wait %.0f seconds before requesting a new code", ErrCooldownNotPassed, remaining)
		}
	}

	// Delete any existing verification records for this user/email
	s.repo.DeleteEmailVerificationByUserID(ctx, user.ID)

	// Generate new OTP
	otp, err := hash.GenerateOTP()
	if err != nil {
		return nil, fmt.Errorf("failed to generate OTP: %w", err)
	}

	// Generate new verification token
	rawToken, err := hash.GenerateToken()
	if err != nil {
		return nil, fmt.Errorf("failed to generate token: %w", err)
	}

	// Create new verification record
	ev := &authmodels.EmailVerification{
		Email:     req.Email,
		UserID:    user.ID,
		Type:      authmodels.VerificationTypeSignup,
		OTPHash:   hash.HashSHA256(otp),
		TokenHash: hash.HashSHA256(rawToken),
		ExpiresAt: time.Now().Add(time.Duration(config.Auth.OTPExpiry) * time.Minute),
	}

	verificationID, err := s.repo.CreateEmailVerification(ctx, ev)
 	if err != nil {
		return nil, fmt.Errorf("failed to create verification: %w", err)
	}

	vid := fmt.Sprintf("%d", verificationID)

	// Send verification email (async)
	email.SendEmailAsync(email.EmailData{
		To:      req.Email,
		Subject: "Verify your CineMatch account",
		Body:    fmt.Sprintf("<h1>Welcome to CineMatch!</h1><p>Your verification code is: <strong>%s</strong></p><p>Or click here to verify: <a href=\"https://cinematch.com/verify?id=%s&token=%s\">Verify Email</a></p><p>This code expires in 15 minutes.</p>", otp, vid, rawToken),
	})

	return &ResendVerificationResponse{
		Message:        "Verification code sent",
		VerificationID: vid,
	}, nil
}

// ForgotPassword sends password reset OTP/token to user email
func (s *Service) ForgotPassword(ctx context.Context, req *ForgotPasswordRequest) (*ForgotPasswordResponse, error) {
	// Get user by email
	user, err := s.repo.GetUserByEmail(ctx, req.Email)
	if err != nil {
		// Return success anyway to prevent email enumeration
		return &ForgotPasswordResponse{
			Message: "If an account exists with this email, a reset link has been sent",
		}, nil
	}

	// Check if user is verified
	if !user.IsVerified {
		return &ForgotPasswordResponse{
			Message: "If an account exists with this email, a reset link has been sent",
		}, nil
	}

	// Generate reset token (for link only, no OTP)
	rawToken, err := hash.GenerateToken()
	if err != nil {
		return nil, fmt.Errorf("failed to generate token: %w", err)
	}

	// Create verification record with type "password_reset"
	ev := &authmodels.EmailVerification{
		Email:     req.Email,
		UserID:    user.ID,
		Type:      authmodels.VerificationTypePasswordReset,
		TokenHash: hash.HashSHA256(rawToken),
		ExpiresAt: time.Now().Add(time.Duration(config.Auth.OTPExpiry) * time.Minute),
	}

	verificationID, err := s.repo.CreateEmailVerification(ctx, ev)
	if err != nil {
		return nil, fmt.Errorf("failed to create verification: %w", err)
	}

	// Send password reset email (async)
	email.SendEmailAsync(email.EmailData{
		To:      req.Email,
		Subject: "CineMatch Password Reset",
		Body:    fmt.Sprintf("<h1>Reset Your Password</h1><p>Click the link below to reset your password:</p><p><a href=\"https://cinematch.com/reset-password?token=%s&id=%d\">Reset Password</a></p><p>This link expires in 15 minutes.</p>", rawToken, verificationID),
	})

	return &ForgotPasswordResponse{
		Message: "If an account exists with this email, a reset link has been sent",
	}, nil
}

// ResetPassword resets password using OTP or token
func (s *Service) ResetPassword(ctx context.Context, req *ResetPasswordRequest) (*ResetPasswordResponse, error) {
	var ev *authmodels.EmailVerification
	var err error

	// Try verificationID first if provided
	if req.VerificationID != "" {
		id, parseErr := strconv.ParseInt(req.VerificationID, 10, 64)
		if parseErr == nil {
			ev, err = s.repo.GetEmailVerification(ctx, id)
			if err == nil && ev.Type == "password_reset" && ev.TokenHash == hash.HashSHA256(req.Token) {
				// Valid verification ID and token match
			} else if err != nil {
				ev = nil // Fall through to token lookup
			}
		}
	}

	// If no verificationID or it failed, try token lookup directly
	if ev == nil {
		tokenHash := hash.HashSHA256(req.Token)
		ev, err = s.repo.GetEmailVerificationByToken(ctx, tokenHash)
		if err != nil {
			return nil, ErrInvalidToken
		}
		// Verify it's a password_reset type
		if ev.Type != "password_reset" {
			return nil, ErrInvalidToken
		}
	}

	// Check if already used
	if ev.UsedAt != nil {
		return nil, ErrInvalidToken
	}

	// Check expiration
	if time.Now().After(ev.ExpiresAt) {
		return nil, ErrTokenExpired
	}

	// Check max attempts
	if ev.Attempts >= config.Auth.OTPMaxAttempts {
		return nil, ErrMaxAttemptsReached
	}

	// Verify token
	tokenHash := hash.HashSHA256(req.Token)
	if ev.TokenHash != tokenHash {
		// Increment attempts
		s.repo.UpdateEmailVerificationAttempts(ctx, ev.ID, ev.Attempts+1)
		return nil, ErrInvalidToken
	}

	// Hash new password
	passwordHash, err := hash.HashPassword(req.NewPassword)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	// Update user password
	if err := s.repo.UpdateUserPassword(ctx, ev.UserID, passwordHash); err != nil {
		return nil, fmt.Errorf("failed to update password: %w", err)
	}

	// Mark verification as used
	s.repo.MarkEmailVerificationUsed(ctx, ev.ID)

	// Delete verification record
	s.repo.DeleteEmailVerification(ctx, ev.ID)

	// Invalidate all sessions for this user (optional security measure)
	s.repo.DeleteAllUserSessions(ctx, ev.UserID)

	return &ResetPasswordResponse{
		Message: "Password has been reset successfully",
	}, nil
}