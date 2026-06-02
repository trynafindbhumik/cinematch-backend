package email

import (
	"fmt"
	"os"
	"strconv"
	"sync"

	"github.com/resend/resend-go/v3"
	"github.com/trynafindbhumik/cinematch-backend/internal/shared/logger"
)

type EmailConfig struct {
	APIKey      string
	FromAddress string
	FromName    string
	Enabled     bool
}

var (
	emailCfg     EmailConfig
	emailCfgOnce sync.Once
)

func LoadEmailConfig() {
	emailCfgOnce.Do(func() {
		emailCfg.APIKey = getEnv("RESEND_API_KEY", "")
		emailCfg.FromAddress = getEnv("EMAIL_FROM_ADDRESS", "noreply@cinematch.com")
		emailCfg.FromName = getEnv("EMAIL_FROM_NAME", "CineMatch")
		emailCfg.Enabled = getEnvBool("EMAIL_ENABLED", true)
	})
}

// SendVerificationEmail sends a verification email with OTP and link
func SendVerificationEmail(to, otp, verificationID, token string) error {
	link := fmt.Sprintf("https://cinematchh.vercel.app/verify?id=%s&token=%s", verificationID, token)
	body := fmt.Sprintf(`
		<h1>Welcome to CineMatch!</h1>
		<p>Your verification code is: <strong>%s</strong></p>
		<p>Or click here to verify: <a href="%s">Verify Email</a></p>
		<p>This code expires in 15 minutes.</p>
	`, otp, link)
	return SendEmail(EmailData{To: to, Subject: "Verify your CineMatch account", Body: body})
}

// SendPasswordResetEmail sends a password reset email
func SendPasswordResetEmail(to, token, verificationID string) error {
	link := fmt.Sprintf("https://cinematchh.vercel.app/reset-password?token=%s&id=%s", token, verificationID)
	body := fmt.Sprintf(`
		<h1>Reset Your Password</h1>
		<p>Click the link below to reset your password:</p>
		<p><a href="%s">Reset Password</a></p>
		<p>This link expires in 15 minutes.</p>
	`, link)
	return SendEmail(EmailData{To: to, Subject: "CineMatch Password Reset", Body: body})
}

func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

func getEnvBool(key string, defaultVal bool) bool {
	if val := os.Getenv(key); val != "" {
		if boolVal, err := strconv.ParseBool(val); err == nil {
			return boolVal
		}
	}
	return defaultVal
}

type EmailData struct {
	To         string
	Subject    string
	Body       string
	Attachment *Attachment // Optional attachment
}

type Attachment struct {
	Filename string
	MimeType string
	Data     []byte
}

var (
	resendClient     *resend.Client
	resendClientOnce sync.Once
)

func getClient() *resend.Client {
	resendClientOnce.Do(func() {
		resendClient = resend.NewClient(emailCfg.APIKey)
	})
	return resendClient
}

func SendEmail(email EmailData) error {
	// Check if email is enabled
	if !emailCfg.Enabled {
		logger.Debug("Email disabled, skipping send",
			logger.String("to", email.To),
		)
		return nil
	}

	if emailCfg.APIKey == "" {
		logger.Warn("Resend not configured, skipping email send")
		return nil
	}

	params := &resend.SendEmailRequest{
		From:    fmt.Sprintf("%s <%s>", emailCfg.FromName, emailCfg.FromAddress),
		To:      []string{email.To},
		Subject: email.Subject,
		Html:    email.Body,
	}

	// Add attachment if provided
	if email.Attachment != nil {
		params.Attachments = []*resend.Attachment{
			{
				Filename:    email.Attachment.Filename,
				Content:     email.Attachment.Data,
				ContentType: email.Attachment.MimeType,
			},
		}
	}

	if _, err := getClient().Emails.Send(params); err != nil {
		return fmt.Errorf("failed to send email: %w", err)
	}

	return nil
}

// SendEmailAsync sends email asynchronously (fire and forget)
func SendEmailAsync(email EmailData) {
	go func() {
		if err := SendEmail(email); err != nil {
			logger.Error("Async email send failed",
				logger.String("to", email.To),
				logger.Err(err),
			)
		}
	}()
}
