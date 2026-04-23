package email

import (
	"crypto/tls"
	"fmt"
	"os"
	"strconv"
	"sync"

	"gopkg.in/mail.v2"
)

type EmailConfig struct {
	SMTPHost     string
	SMTPPort     string
	SMTPUsername string
	SMTPPassword string
	FromAddress  string
	FromName     string
	Enabled      bool
}

var (
	emailCfg     EmailConfig
	emailCfgOnce sync.Once
)

func LoadEmailConfig() {
	emailCfgOnce.Do(func() {
		emailCfg.SMTPHost = getEnv("SMTP_HOST", "smtp.gmail.com")
		emailCfg.SMTPPort = getEnv("SMTP_PORT", "587")
		emailCfg.SMTPUsername = getEnv("SMTP_USERNAME", "")
		emailCfg.SMTPPassword = getEnv("SMTP_PASSWORD", "")
		emailCfg.FromAddress = getEnv("EMAIL_FROM_ADDRESS", "noreply@cinematch.com")
		emailCfg.FromName = getEnv("EMAIL_FROM_NAME", "CineMatch")
		emailCfg.Enabled = getEnvBool("EMAIL_ENABLED", true)
	})
}

// SendVerificationEmail sends a verification email with OTP and link
func SendVerificationEmail(to, otp, verificationID, token string) error {
	link := fmt.Sprintf("https://cinematch.com/verify?id=%s&token=%s", verificationID, token)
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
	link := fmt.Sprintf("https://cinematch.com/reset-password?token=%s&id=%s", token, verificationID)
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
	To      string
	Subject string
	Body    string
}

var dialer *mail.Dialer
var dialerOnce sync.Once

func getDialer() *mail.Dialer {
	dialerOnce.Do(func() {
		port, _ := strconv.Atoi(emailCfg.SMTPPort)
		dialer = mail.NewDialer(
			emailCfg.SMTPHost,
			port,
			emailCfg.SMTPUsername,
			emailCfg.SMTPPassword,
		)
		dialer.StartTLSPolicy = mail.OpportunisticStartTLS
		dialer.TLSConfig = &tls.Config{
			ServerName: emailCfg.SMTPHost,
		}
	})
	return dialer
}

func SendEmail(email EmailData) error {
	// Check if email is enabled
	if !emailCfg.Enabled {
		fmt.Printf("Email: Disabled, skipping send to %s\n", email.To)
		return nil
	}

	if emailCfg.SMTPUsername == "" || emailCfg.SMTPPassword == "" {
		fmt.Println("Email: SMTP not configured, skipping email send")
		return nil
	}

	m := mail.NewMessage()
	m.SetHeader("From", fmt.Sprintf("%s <%s>", emailCfg.FromName, emailCfg.FromAddress))
	m.SetHeader("To", email.To)
	m.SetHeader("Subject", email.Subject)
	m.SetBody("text/html", email.Body)

	d := getDialer()
	if err := d.DialAndSend(m); err != nil {
		return fmt.Errorf("failed to send email: %w", err)
	}

	return nil
}

// SendEmailAsync sends email asynchronously (fire and forget)
func SendEmailAsync(email EmailData) {
	go func() {
		if err := SendEmail(email); err != nil {
			fmt.Printf("Async email send failed to %s: %v\n", email.To, err)
		}
	}()
}