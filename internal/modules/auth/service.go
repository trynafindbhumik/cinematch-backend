package auth

// User authentication service handling signup, login, password reset, and email verification.

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/trynafindbhumik/cinematch-backend/internal/config"
	authmodels "github.com/trynafindbhumik/cinematch-backend/internal/models/auth"
	"github.com/trynafindbhumik/cinematch-backend/internal/shared/email"
	"github.com/trynafindbhumik/cinematch-backend/internal/shared/hash"
	"github.com/trynafindbhumik/cinematch-backend/internal/shared/jwt"
	"github.com/trynafindbhumik/cinematch-backend/internal/shared/logger"
	"github.com/trynafindbhumik/cinematch-backend/internal/shared/redis"
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

// Signup creates new user and sends verification email
func (s *Service) Signup(ctx context.Context, req *SignupRequest) (*SignupResponse, error) {
	// Check if user already exists
	existingUser, _ := s.repo.GetUserByEmail(ctx, req.Email)
	if existingUser != nil {
		return &SignupResponse{
			Message:        "Email already registered",
			VerificationID: "",
		}, nil
	}

	// Hash password with bcrypt
	passwordHash, err := hash.HashPassword(req.Password)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	// Create unverified user
	user, err := s.repo.CreateUser(ctx, req.Email, passwordHash, req.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	// Generate OTP and verification token
	otp, err := hash.GenerateOTP()
	if err != nil {
		return nil, fmt.Errorf("failed to generate OTP: %w", err)
	}

	rawToken, err := hash.GenerateToken()
	if err != nil {
		return nil, fmt.Errorf("failed to generate token: %w", err)
	}

	// Store verification data in Redis
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

	// Send verification email (async - don't block on email delivery)
	email.SendEmailAsync(email.EmailData{
		To:      req.Email,
		Subject: "Verify your CineMatch account",
		Body:    fmt.Sprintf("<h1>Welcome to CineMatch!</h1><p>Your verification code is: <strong>%s</strong></p><p>Or click here to verify: <a href=\"https://cinematchh.vercel.app/verify?token=%s\">Verify Email</a></p><p>This code expires in 15 minutes.</p>", otp, rawToken),
	})

	return &SignupResponse{
		Message:        "Verification required",
		VerificationID: verificationID,
	}, nil
}

// Verify confirms email with OTP or token
func (s *Service) Verify(ctx context.Context, req *VerifyRequest, sessionParams SessionParams) (*VerifyResponse, error) {
	var verificationID string
	var ev *authmodels.EmailVerification
	var err error

	// Try verificationID first if provided
	if req.VerificationID != "" {
		ev, err = s.repo.GetEmailVerification(ctx, req.VerificationID)
		if err == nil {
			verificationID = req.VerificationID
		}
	}

	// If no verificationID or not found, try token lookup
	if verificationID == "" && req.Token != "" {
		tokenHash := hash.HashSHA256(req.Token)
		ev, verificationID, err = s.repo.GetEmailVerificationByTokenWithID(ctx, tokenHash)
		if err != nil {
			return nil, ErrInvalidToken
		}
	}

	// Neither provided
	if verificationID == "" {
		return nil, ErrInvalidToken
	}

	// Check if verification has expired
	if time.Now().After(ev.ExpiresAt) {
		return nil, ErrTokenExpired
	}

	// Check if max attempts reached
	if ev.Attempts >= config.Auth.OTPMaxAttempts {
		return nil, ErrMaxAttemptsReached
	}

	// Verify using OTP or token
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
		// Increment failed attempts counter
		s.repo.UpdateEmailVerificationAttempts(ctx, verificationID, ev.Attempts+1)
		return nil, ErrInvalidToken
	}

	// Mark user as verified
	err = s.repo.SetUserVerified(ctx, ev.UserID)
	if err != nil {
		logger.Error("SetUserVerified failed", logger.Int64("user_id", ev.UserID), logger.Err(err))
		return nil, fmt.Errorf("failed to mark user verified: %w", err)
	}
	logger.Debug("SetUserVerified success", logger.Int64("user_id", ev.UserID))

	// Get user for token generation (use IsVerified=true since we just verified)
	user, err := s.repo.GetUserByID(ctx, ev.UserID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	// Generate tokens (user.IsVerified is still false in DB, pass true since we just verified)
	accessToken, jti, err := jwt.GenerateAccessToken(user.ID, user.Email, user.Role, true, user.IsFirstLogin)
	if err != nil {
		return nil, fmt.Errorf("failed to generate access token: %w", err)
	}

	refreshToken, err := jwt.GenerateRandomRefreshToken()
	if err != nil {
		return nil, fmt.Errorf("failed to generate refresh token: %w", err)
	}

	// Delete all existing sessions to invalidate them
	err = s.repo.DeleteAllUserSessions(ctx, ev.UserID)
	if err != nil {
		logger.Warn("DeleteAllUserSessions failed", logger.Int64("user_id", ev.UserID), logger.Err(err))
	}
	logger.Debug("Deleted all existing sessions", logger.Int64("user_id", ev.UserID))

	// Create new session with device info
	_, err = s.repo.CreateSession(ctx, user.ID, refreshToken, jti, sessionParams.DeviceName, sessionParams.UserAgent, sessionParams.IPAddress)
	if err != nil {
		logger.Error("CreateSession failed", logger.Int64("user_id", user.ID), logger.Err(err))
		return nil, fmt.Errorf("failed to create session: %w", err)
	}
	logger.Debug("Session created", logger.Int64("user_id", user.ID), logger.String("jti", jti))

	// Delete verification record
	s.repo.DeleteEmailVerification(ctx, verificationID)

	logger.Debug("Email verification completed",
		logger.Int64("user_id", ev.UserID),
	)

	return &VerifyResponse{
		AccessToken:     accessToken,
		RefreshToken:    refreshToken,
		IsVerified:      true,
		IsFirstLogin:    user.IsFirstLogin,
		NeedsOnboarding: user.IsFirstLogin,
		Message:         "Email verified successfully",
	}, nil
}

// Login authenticates user and creates session
func (s *Service) Login(ctx context.Context, req *LoginRequest, sessionParams SessionParams) (*LoginResponse, error) {
	// Find user by email
	user, err := s.repo.GetUserByEmail(ctx, req.Email)
	if err != nil {
		return nil, fmt.Errorf("invalid credentials")
	}

	// Check if account is temporarily locked
	if user.LockoutUntil != nil && time.Now().Before(*user.LockoutUntil) {
		return nil, fmt.Errorf("account is temporarily locked")
	}

	// Check if account is disabled and re-enable if so
	if user.IsDisabled {
		err = s.repo.EnableAccount(ctx, user.ID)
		if err != nil {
			logger.Warn("EnableAccount failed during login", logger.Int64("user_id", user.ID), logger.Err(err))
		}
		logger.Debug("Re-enabled disabled account during login", logger.Int64("user_id", user.ID))
	}

	// Verify password
	if !hash.CheckPassword(req.Password, user.PasswordHash) {
		// Increment failed attempts counter
		s.repo.IncrementFailedAttempts(ctx, user.ID)

		// Lock account after 5 failed attempts
		if user.FailedAttempts+1 >= 5 {
			lockoutUntil := time.Now().Add(time.Duration(config.Auth.LockoutDuration) * time.Minute)
			s.repo.LockoutUser(ctx, user.ID, lockoutUntil)
			return nil, fmt.Errorf("account is temporarily locked due to too many failed attempts")
		}
		return nil, fmt.Errorf("invalid credentials")
	}

	// Reset failed attempts on successful login
	s.repo.ResetFailedAttempts(ctx, user.ID)

	// Generate tokens
	accessToken, jti, err := jwt.GenerateAccessToken(user.ID, user.Email, user.Role, user.IsVerified, user.IsFirstLogin)
	if err != nil {
		return nil, fmt.Errorf("failed to generate access token: %w", err)
	}

	refreshToken, err := jwt.GenerateRandomRefreshToken()
	if err != nil {
		return nil, fmt.Errorf("failed to generate refresh token: %w", err)
	}

	// Create session with device info
	_, err = s.repo.CreateSession(ctx, user.ID, refreshToken, jti, sessionParams.DeviceName, sessionParams.UserAgent, sessionParams.IPAddress)
	if err != nil {
		if errors.Is(err, ErrMaxSessionsReached) {
			// Generate magic link for session management
			// Note: We don't create a session yet - user must manage sessions via magic link first
			magicLink, magicErr := s.CreateMagicLink(ctx, user.ID, user.Email)
			if magicErr != nil {
				return nil, fmt.Errorf("failed to generate magic link: %w", magicErr)
			}

			return &LoginResponse{
				AccessToken:        "", // No access token - user must use magic link to manage sessions
				RefreshToken:       "", // No refresh token either
				IsSessionExhausted: true,
				MagicLink:          magicLink,
			}, nil
		}
		return nil, fmt.Errorf("failed to create session: %w", err)
	}

	return &LoginResponse{
		AccessToken:        accessToken,
		RefreshToken:       refreshToken,
		IsSessionExhausted: false,
		MagicLink:          "",
	}, nil
}

// Logout invalidates session by removing refresh token from database
// Access token will expire naturally within its TTL (15 min)
func (s *Service) Logout(ctx context.Context, refreshToken string, accessToken string) error {
	if refreshToken != "" {
		if err := s.repo.DeleteSessionByRefreshToken(ctx, refreshToken); err != nil {
			logger.Error("Failed to delete session on logout",
				logger.Err(err),
			)
		}
	}

	return nil
}

// RefreshToken validates refresh token and issues new access token
// Implements token rotation for security - always rotates refresh token on each use
// Validates: session exists, user not deleted, user not disabled, user not locked
func (s *Service) RefreshToken(ctx context.Context, refreshToken string) (*RefreshTokenResponse, error) {
	// Find session by refresh token
	sess, err := s.repo.GetSessionByRefreshToken(ctx, refreshToken)
	if err != nil {
		return nil, fmt.Errorf("failed to get session: %w", err)
	}
	if sess == nil {
		return nil, ErrInvalidToken
	}

	logger.Debug("RefreshToken: GetSessionByRefreshToken",
		logger.String("token", refreshToken),
		logger.String("token_hash", hash.HashSHA256(refreshToken)),
		logger.Err(err),
		logger.String("session_id", sess.ID),
	)

	// Get user for validation
	user, err := s.repo.GetUserByID(ctx, sess.UserID)
	if err != nil {
		return nil, ErrInvalidToken
	}

	// Check if user is deleted
	if user.IsDeleted {
		return nil, ErrInvalidToken
	}

	// Check if user is disabled
	if user.DisabledUntil != nil && time.Now().Before(*user.DisabledUntil) {
		return nil, ErrInvalidToken
	}

	// Check if account is locked
	if user.LockoutUntil != nil && time.Now().Before(*user.LockoutUntil) {
		return nil, ErrInvalidToken
	}

	// Generate new access token
	accessToken, newJTI, err := jwt.GenerateAccessToken(user.ID, user.Email, user.Role, user.IsVerified, user.IsFirstLogin)
	if err != nil {
		return nil, fmt.Errorf("failed to generate access token: %w", err)
	}

	// Generate new refresh token (rotation for security)
	newRefreshToken, err := jwt.GenerateRandomRefreshToken()
	if err != nil {
		return nil, fmt.Errorf("failed to generate refresh token: %w", err)
	}

	// Update session with new refresh token and extend expiry
	err = s.repo.RotateRefreshToken(ctx, sess.ID, newRefreshToken)
	if err != nil {
		return nil, fmt.Errorf("failed to rotate refresh token: %w", err)
	}

	// Update JTI hash to match new access token
	if err := s.repo.UpdateSessionJTIHash(ctx, sess.ID, newJTI); err != nil {
		logger.Warn("Failed to update session JTI hash during refresh",
			logger.String("session_id", sess.ID),
			logger.Err(err),
		)
	}

	return &RefreshTokenResponse{
		AccessToken:  accessToken,
		RefreshToken: newRefreshToken,
	}, nil
}

// ResendVerification sends new OTP to unverified users
// Returns success even if email doesn't exist (prevents email enumeration)
func (s *Service) ResendVerification(ctx context.Context, req *ResendVerificationRequest) (*ResendVerificationResponse, error) {
	// Find user by email
	user, err := s.repo.GetUserByEmail(ctx, req.Email)
	if err != nil {
		// Return success to prevent email enumeration
		return &ResendVerificationResponse{
			Message:        "Verification code sent",
			VerificationID: "",
		}, nil
	}

	// Already verified
	if user.IsVerified {
		return &ResendVerificationResponse{
			Message:        "Verification code sent",
			VerificationID: "",
		}, nil
	}

	// Check resend cooldown
	existingEv, _, err := s.repo.GetLatestEmailVerificationByUserID(ctx, user.ID)
	if err == nil && existingEv != nil {
		cooldownEnd := existingEv.CreatedAt.Add(time.Duration(config.Auth.ResendCooldown) * time.Second)
		if time.Now().Before(cooldownEnd) {
			remaining := time.Until(cooldownEnd).Seconds()
			return nil, fmt.Errorf("%w: please wait %.0f seconds before requesting a new code", ErrCooldownNotPassed, remaining)
		}
	}

	// Delete existing verification records
	s.repo.DeleteEmailVerificationByUserID(ctx, user.ID)

	// Generate new OTP and token
	otp, err := hash.GenerateOTP()
	if err != nil {
		return nil, fmt.Errorf("failed to generate OTP: %w", err)
	}

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

	// Send verification email
	email.SendEmailAsync(email.EmailData{
		To:      req.Email,
		Subject: "Verify your CineMatch account",
		Body:    fmt.Sprintf("<h1>Welcome to CineMatch!</h1><p>Your verification code is: <strong>%s</strong></p><p>Or click here to verify: <a href=\"https://cinematchh.vercel.app/verify?token=%s\">Verify Email</a></p><p>This code expires in 15 minutes.</p>", otp, rawToken),
	})

	return &ResendVerificationResponse{
		Message:        "Verification code sent",
		VerificationID: verificationID,
	}, nil
}

// ForgotPassword initiates password reset flow
// Returns success even if email doesn't exist (prevents email enumeration)
func (s *Service) ForgotPassword(ctx context.Context, req *ForgotPasswordRequest) (*ForgotPasswordResponse, error) {
	// Find user by email
	user, err := s.repo.GetUserByEmail(ctx, req.Email)
	if err != nil {
		// Return success anyway to prevent email enumeration
		return &ForgotPasswordResponse{
			Message: "If an account exists with this email, a reset link has been sent",
		}, nil
	}

	// Only verified users can reset password
	if !user.IsVerified {
		return &ForgotPasswordResponse{
			Message: "If an account exists with this email, a reset link has been sent",
		}, nil
	}

	// Generate reset token
	rawToken, err := hash.GenerateToken()
	if err != nil {
		return nil, fmt.Errorf("failed to generate token: %w", err)
	}

	// Store verification record with password_reset type
	ev := &authmodels.EmailVerification{
		Email:     req.Email,
		UserID:    user.ID,
		Type:      authmodels.VerificationTypePasswordReset,
		TokenHash: hash.HashSHA256(rawToken),
		ExpiresAt: time.Now().Add(time.Duration(config.Auth.OTPExpiry) * time.Minute),
	}

	_, err = s.repo.CreateEmailVerification(ctx, ev)
	if err != nil {
		return nil, fmt.Errorf("failed to create verification: %w", err)
	}

	// Send password reset email
	email.SendEmailAsync(email.EmailData{
		To:      req.Email,
		Subject: "CineMatch Password Reset",
		Body:    fmt.Sprintf("<h1>Reset Your Password</h1><p>Click the link below to reset your password:</p><p><a href=\"https://cinematchh.vercel.app/reset-password?token=%s\">Reset Password</a></p><p>This link expires in 15 minutes.</p>", rawToken),
	})

	return &ForgotPasswordResponse{
		Message: "If an account exists with this email, a reset link has been sent",
	}, nil
}

// ResetPassword changes password using valid token
func (s *Service) ResetPassword(ctx context.Context, req *ResetPasswordRequest) (*ResetPasswordResponse, error) {
	// Lookup by token hash only (token is self-contained from email link)
	tokenHash := hash.HashSHA256(req.Token)
	ev, verificationID, err := s.repo.GetEmailVerificationByTokenWithID(ctx, tokenHash)
	if err != nil {
		return nil, ErrInvalidToken
	}
	if ev.Type != authmodels.VerificationTypePasswordReset {
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

	// Hash new password
	passwordHash, err := hash.HashPassword(req.NewPassword)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	// Update user password
	if err := s.repo.UpdateUserPassword(ctx, ev.UserID, passwordHash); err != nil {
		return nil, fmt.Errorf("failed to update password: %w", err)
	}

	// Delete verification record
	s.repo.DeleteEmailVerification(ctx, verificationID)

	// Invalidate all sessions for this user
	if err := s.repo.DeleteAllUserSessions(ctx, ev.UserID); err != nil {
		logger.Error("Failed to delete all user sessions on password reset",
			logger.Int64("user_id", ev.UserID),
			logger.Err(err),
		)
	}

	return &ResetPasswordResponse{
		Message: "Password has been reset successfully",
	}, nil
}

// ResendReset resends password reset email with cooldown protection
func (s *Service) ResendReset(ctx context.Context, req *ResendResetRequest) (*ResendResetResponse, error) {
	// Find user by email
	user, err := s.repo.GetUserByEmail(ctx, req.Email)
	if err != nil {
		// Return success anyway to prevent email enumeration
		return &ResendResetResponse{
			Message: "If an account exists with this email, a reset link has been sent",
		}, nil
	}

	// Only verified users can reset password
	if !user.IsVerified {
		return &ResendResetResponse{
			Message: "If an account exists with this email, a reset link has been sent",
		}, nil
	}

	// Check existing verification for cooldown via user_id lookup
	if existingEv, existingID, err := s.repo.GetLatestEmailVerificationByUserID(ctx, user.ID); err == nil {
		if existingEv != nil && existingEv.Type == authmodels.VerificationTypePasswordReset {
			cooldownEnd := existingEv.CreatedAt.Add(time.Duration(config.Auth.ResendCooldown) * time.Second)
			if time.Now().Before(cooldownEnd) {
				remaining := time.Until(cooldownEnd).Seconds()
				return nil, fmt.Errorf("%w: please wait %.0f seconds before requesting a new code", ErrCooldownNotPassed, remaining)
			}
			// Delete old verification
			s.repo.DeleteEmailVerification(ctx, existingID)
		}
	}

	// Generate new reset token
	rawToken, err := hash.GenerateToken()
	if err != nil {
		return nil, fmt.Errorf("failed to generate token: %w", err)
	}

	// Store verification record with password_reset type
	ev := &authmodels.EmailVerification{
		Email:     req.Email,
		UserID:    user.ID,
		Type:      authmodels.VerificationTypePasswordReset,
		TokenHash: hash.HashSHA256(rawToken),
		ExpiresAt: time.Now().Add(time.Duration(config.Auth.OTPExpiry) * time.Minute),
	}

	_, err = s.repo.CreateEmailVerification(ctx, ev)
	if err != nil {
		return nil, fmt.Errorf("failed to create verification: %w", err)
	}

	// Send password reset email
	email.SendEmailAsync(email.EmailData{
		To:      req.Email,
		Subject: "CineMatch Password Reset",
		Body:    fmt.Sprintf("<h1>Reset Your Password</h1><p>Click the link below to reset your password:</p><p><a href=\"https://cinematchh.vercel.app/reset-password?token=%s\">Reset Password</a></p><p>This link expires in 15 minutes.</p>", rawToken),
	})

	return &ResendResetResponse{
		Message: "If an account exists with this email, a reset link has been sent",
	}, nil
}

// InitVerify initiates email verification for logged-in unverified users
// Uses access token to identify user, then sends new OTP to their email
func (s *Service) InitVerify(ctx context.Context, userID int64) (*InitVerifyResponse, error) {
	// Get user by ID
	user, err := s.repo.GetUserByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	// Check if already verified
	if user.IsVerified {
		return &InitVerifyResponse{
			Message:    "Email already verified",
			Email:      user.Email,
			IsVerified: true,
		}, nil
	}

	// Delete any existing verification records for this user
	s.repo.DeleteEmailVerificationByUserID(ctx, userID)

	// Generate new OTP and token
	otp, err := hash.GenerateOTP()
	if err != nil {
		return nil, fmt.Errorf("failed to generate OTP: %w", err)
	}

	rawToken, err := hash.GenerateToken()
	if err != nil {
		return nil, fmt.Errorf("failed to generate token: %w", err)
	}

	// Create new verification record
	ev := &authmodels.EmailVerification{
		Email:     user.Email,
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

	// Send verification email
	email.SendEmailAsync(email.EmailData{
		To:      user.Email,
		Subject: "Verify your CineMatch account",
		Body:    fmt.Sprintf("<h1>Welcome to CineMatch!</h1><p>Your verification code is: <strong>%s</strong></p><p>Or click here to verify: <a href=\"https://cinematchh.vercel.app/verify?token=%s\">Verify Email</a></p><p>This code expires in 15 minutes.</p>", otp, rawToken),
	})

	return &InitVerifyResponse{
		Message:        "Verification code sent",
		Email:          user.Email,
		VerificationID: verificationID,
		IsVerified:     false,
	}, nil
}

// GetAllSessions returns all active sessions for a user
func (s *Service) GetAllSessions(ctx context.Context, userID int64, isMagicLink bool) (*GetSessionsResponse, error) {
	sessions, err := s.repo.GetAllUserSessions(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get sessions: %w", err)
	}

	var sessionResponses []SessionResponse
	for _, sess := range sessions {
		sessionResponses = append(sessionResponses, SessionResponse{
			ID:         sess.ID,
			DeviceName: sess.DeviceName,
			UserAgent:  sess.UserAgent,
			CreatedAt:  sess.CreatedAt.Format(time.RFC3339),
			LastLogin:  sess.LastUsedAt.Format(time.RFC3339),
			IsCurrent:  false, // Magic link can't identify current session
		})
	}

	// Ensure we return an empty array, not nil
	if sessionResponses == nil {
		sessionResponses = []SessionResponse{}
	}

	return &GetSessionsResponse{
		Sessions: sessionResponses,
		Total:    len(sessionResponses),
	}, nil
}

// GetAllSessionsWithCurrent returns all active sessions and marks the current one
func (s *Service) GetAllSessionsWithCurrent(ctx context.Context, userID int64, refreshTokenHash string) (*GetSessionsResponse, error) {
	sessions, err := s.repo.GetAllUserSessions(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get sessions: %w", err)
	}

	var sessionResponses []SessionResponse
	for _, sess := range sessions {
		sessionResponses = append(sessionResponses, SessionResponse{
			ID:         sess.ID,
			DeviceName: sess.DeviceName,
			UserAgent:  sess.UserAgent,
			CreatedAt:  sess.CreatedAt.Format(time.RFC3339),
			LastLogin:  sess.LastUsedAt.Format(time.RFC3339),
			IsCurrent:  sess.RefreshTokenHash == refreshTokenHash,
		})
	}

	// Ensure we return an empty array, not nil
	if sessionResponses == nil {
		sessionResponses = []SessionResponse{}
	}

	return &GetSessionsResponse{
		Sessions: sessionResponses,
		Total:    len(sessionResponses),
	}, nil
}

// DeleteSession deletes a session by ID
// If magicLink is provided and valid, it performs a magic link flow:
//   - Validates magic link
//   - Deletes the specified session
//   - Creates new session for the requesting device
//   - Deletes the magic link from Redis
//   - Returns new access token
func (s *Service) DeleteSession(ctx context.Context, userID int64, sessionID string, magicLink string, sessionParams SessionParams) (*DeleteSessionResponse, error) {
	// If magic link is provided, check if it's valid
	if magicLink != "" {
		magicLinkData, err := s.GetMagicLinkData(ctx, magicLink)
		if err != nil || magicLinkData == nil {
			return nil, ErrInvalidToken
		}

		// Verify magic link belongs to this user
		if magicLinkData.UserID != userID {
			return nil, ErrInvalidToken
		}

		// Get the session to delete to verify it belongs to this user
		sessionToDelete, err := s.repo.GetSessionByID(ctx, sessionID)
		if err != nil {
			return nil, fmt.Errorf("failed to get session: %w", err)
		}
		if sessionToDelete == nil {
			return nil, fmt.Errorf("session not found")
		}
		if sessionToDelete.UserID != userID {
			return nil, fmt.Errorf("session not found")
		}

		// Delete the specified session
		if err := s.repo.DeleteSessionByID(ctx, sessionID); err != nil {
			return nil, fmt.Errorf("failed to delete session: %w", err)
		}

		// Delete magic link from Redis
		if err := s.DeleteMagicLink(ctx, magicLink); err != nil {
			logger.Warn("Failed to delete magic link", logger.Err(err))
		}

		// Get user for token generation
		user, err := s.repo.GetUserByID(ctx, userID)
		if err != nil {
			return nil, fmt.Errorf("failed to get user: %w", err)
		}

		// Generate new tokens
		accessToken, jti, err := jwt.GenerateAccessToken(user.ID, user.Email, user.Role, user.IsVerified, user.IsFirstLogin)
		if err != nil {
			return nil, fmt.Errorf("failed to generate access token: %w", err)
		}

		refreshToken, err := jwt.GenerateRandomRefreshToken()
		if err != nil {
			return nil, fmt.Errorf("failed to generate refresh token: %w", err)
		}

		// Create new session for this device
		_, err = s.repo.CreateSession(ctx, user.ID, refreshToken, jti, sessionParams.DeviceName, sessionParams.UserAgent, sessionParams.IPAddress)
		if err != nil {
			return nil, fmt.Errorf("failed to create session: %w", err)
		}

		return &DeleteSessionResponse{
			Message:         "Session deleted and new session created",
			AccessToken:     accessToken,
			RefreshToken:    refreshToken,
			NeedsOnboarding: user.IsFirstLogin,
		}, nil
	}

	// Normal flow - delete session without creating new one
	// Get the session to verify it belongs to this user
	sessionToDelete, err := s.repo.GetSessionByID(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to get session: %w", err)
	}
	if sessionToDelete == nil {
		return nil, fmt.Errorf("session not found")
	}
	if sessionToDelete.UserID != userID {
		return nil, fmt.Errorf("session not found")
	}

	// Delete the session
	if err := s.repo.DeleteSessionByID(ctx, sessionID); err != nil {
		return nil, fmt.Errorf("failed to delete session: %w", err)
	}

	return &DeleteSessionResponse{
		Message: "Session deleted successfully",
	}, nil
}

// CreateMagicLink creates a magic link for session management
func (s *Service) CreateMagicLink(ctx context.Context, userID int64, email string) (string, error) {
	token, err := hash.GenerateToken()
	if err != nil {
		return "", fmt.Errorf("failed to generate token: %w", err)
	}

	magicLinkData := &redis.MagicLinkData{
		ID:        token,
		UserID:    userID,
		Email:     email,
		CreatedAt: time.Now().Unix(),
	}

	ttl := time.Duration(config.Auth.MagicLinkExpiry) * time.Minute
	if err := s.repo.CreateMagicLink(ctx, token, magicLinkData, ttl); err != nil {
		return "", fmt.Errorf("failed to create magic link: %w", err)
	}

	return token, nil
}

// GetMagicLinkData retrieves magic link data
func (s *Service) GetMagicLinkData(ctx context.Context, token string) (*redis.MagicLinkData, error) {
	return s.repo.GetMagicLink(ctx, token)
}

// DeleteMagicLink deletes a magic link
func (s *Service) DeleteMagicLink(ctx context.Context, token string) error {
	return s.repo.DeleteMagicLink(ctx, token)
}
