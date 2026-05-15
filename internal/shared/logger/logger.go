package logger

// Structured logging package using Zap.
// Provides log levels, structured fields, and HTTP request logging.

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Log levels for environment configuration
// DEBUG = detailed info (development only)
// INFO = normal operations
// WARN = potential issues
// ERROR = failures that need attention
// PANIC = critical failures

const (
	LevelDebug = "debug"
	LevelInfo  = "info"
	LevelWarn  = "warn"
	LevelError = "error"
	LevelPanic = "panic"
)

type contextKey string

const (
	requestIDKey   contextKey = "requestID"
	userIDKey      contextKey = "userID"
	correlationKey contextKey = "correlationID"
)

var log *zap.Logger

func initLogger() {
	cfg := zap.Config{
		Level:            zap.NewAtomicLevel(),
		Development:      false,
		Encoding:         "json",
		EncoderConfig:    defaultEncoderConfig(),
		OutputPaths:      []string{"stdout"},
		ErrorOutputPaths: []string{"stderr"},
	}

	env := os.Getenv("ENVIRONMENT")
	logLevel := os.Getenv("LOG_LEVEL")

	// Development mode uses console output and defaults to debug
	if env == "development" {
		cfg.Development = true
		cfg.Encoding = "console"
		if logLevel == "" {
			logLevel = "debug"
		}
	} else {
		// Production mode uses JSON output and defaults to info
		cfg.Development = false
		cfg.Encoding = "json"
		if logLevel == "" {
			logLevel = "info"
		}
	}

	// Parse and set log level
	switch strings.ToLower(logLevel) {
	case LevelDebug:
		cfg.Level.SetLevel(zapcore.DebugLevel)
	case LevelInfo:
		cfg.Level.SetLevel(zapcore.InfoLevel)
	case LevelWarn:
		cfg.Level.SetLevel(zapcore.WarnLevel)
	case LevelError:
		cfg.Level.SetLevel(zapcore.ErrorLevel)
	case LevelPanic:
		cfg.Level.SetLevel(zapcore.PanicLevel)
	default:
		cfg.Level.SetLevel(zapcore.InfoLevel)
	}

	cfg.EncoderConfig = defaultEncoderConfig()

	var err error
	log, err = cfg.Build()
	if err != nil {
		log = zap.NewNop()
	}
}

func defaultEncoderConfig() zapcore.EncoderConfig {
	return zapcore.EncoderConfig{
		TimeKey:        "timestamp",
		LevelKey:       "level",
		NameKey:        "logger",
		CallerKey:      "caller",
		MessageKey:     "message",
		StacktraceKey:  "stacktrace",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    zapcore.CapitalLevelEncoder,
		EncodeTime:     zapcore.ISO8601TimeEncoder,
		EncodeDuration: zapcore.MillisDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
	}
}

// GetLogger returns the logger instance
func GetLogger() *zap.Logger {
	if log == nil {
		initLogger()
	}
	return log
}

// WithContext adds request-scoped fields to the logger
func WithContext(ctx context.Context) *zap.Logger {
	logger := GetLogger()

	if reqID, ok := ctx.Value(requestIDKey).(string); ok && reqID != "" {
		logger = logger.With(zap.String("request_id", reqID))
	}

	if userID, ok := ctx.Value(userIDKey).(int64); ok && userID != 0 {
		logger = logger.With(zap.Int64("user_id", userID))
	}

	if corrID, ok := ctx.Value(correlationKey).(string); ok && corrID != "" {
		logger = logger.With(zap.String("correlation_id", corrID))
	}

	return logger
}

// WithFields adds structured fields to the logger
func WithFields(fields ...zap.Field) *zap.Logger {
	return GetLogger().With(fields...)
}

// WithRequestID adds a request ID to the logger
func WithRequestID(requestID string) *zap.Logger {
	return GetLogger().With(zap.String("request_id", requestID))
}

// WithUserID adds a user ID to the logger
func WithUserID(userID int64) *zap.Logger {
	return GetLogger().With(zap.Int64("user_id", userID))
}

// WithOperation adds an operation name to the logger
func WithOperation(operation string) *zap.Logger {
	return GetLogger().With(zap.String("operation", operation))
}

// Debug logs debug messages (development only)
func Debug(msg string, fields ...zap.Field) {
	GetLogger().Debug(msg, fields...)
}

// Debugf logs debug messages with formatting
func Debugf(format string, args ...interface{}) {
	GetLogger().Debug(fmt.Sprintf(format, args...))
}

// Info logs info messages (both dev and prod)
func Info(msg string, fields ...zap.Field) {
	GetLogger().Info(msg, fields...)
}

// Infof logs info messages with formatting
func Infof(format string, args ...interface{}) {
	GetLogger().Info(fmt.Sprintf(format, args...))
}

// Warn logs warning messages
func Warn(msg string, fields ...zap.Field) {
	GetLogger().Warn(msg, fields...)
}

// Warnf logs warning messages with formatting
func Warnf(format string, args ...interface{}) {
	GetLogger().Warn(fmt.Sprintf(format, args...))
}

// Error logs error messages
func Error(msg string, fields ...zap.Field) {
	GetLogger().Error(msg, fields...)
}

// Errorf logs error messages with formatting
func Errorf(format string, args ...interface{}) {
	GetLogger().Error(fmt.Sprintf(format, args...))
}

// Panic logs panic messages and panics
func Panic(msg string, fields ...zap.Field) {
	GetLogger().Panic(msg, fields...)
}

// Fatal logs fatal messages and exits
func Fatal(msg string, fields ...zap.Field) {
	GetLogger().Fatal(msg, fields...)
}

// String creates a string field for logging
func String(key, val string) zap.Field {
	return zap.String(key, val)
}

// Int creates an int field for logging
func Int(key string, val int) zap.Field {
	return zap.Int(key, val)
}

// Int32 creates an int32 field for logging
func Int32(key string, val int32) zap.Field {
	return zap.Int32(key, val)
}

// Int64 creates an int64 field for logging
func Int64(key string, val int64) zap.Field {
	return zap.Int64(key, val)
}

// Bool creates a bool field for logging
func Bool(key string, val bool) zap.Field {
	return zap.Bool(key, val)
}

// Duration creates a duration field for logging
func Duration(key string, val time.Duration) zap.Field {
	return zap.Duration(key, val)
}

// Any creates a field for any value
func Any(key string, val interface{}) zap.Field {
	return zap.Any(key, val)
}

// Err creates an error field for logging
func Err(err error) zap.Field {
	return zap.Error(err)
}

// LogHTTPRequest logs HTTP request details
func LogHTTPRequest(method, path string, status int, duration time.Duration, userID int64) {
	GetLogger().Info("HTTP Request",
		zap.String("method", method),
		zap.String("path", path),
		zap.Int("status", status),
		zap.Duration("duration", duration),
		zap.Int64("user_id", userID),
	)
}

// LogHTTPRequestWithFields logs HTTP request with additional custom fields
func LogHTTPRequestWithFields(method, path string, status int, duration time.Duration, userID int64, fields []zap.Field) {
	logger := GetLogger().With(
		zap.String("method", method),
		zap.String("path", path),
		zap.Int("status", status),
		zap.Duration("duration", duration),
	)
	if userID > 0 {
		logger = logger.With(zap.Int64("user_id", userID))
	}
	for _, f := range fields {
		logger = logger.With(f)
	}
	logger.Info("HTTP Request")
}

// LogAuthLogin logs login attempt
func LogAuthLogin(success bool, email string, userID int64, err error) {
	if success {
		GetLogger().Info("Login successful",
			zap.String("email_prefix", maskEmail(email)),
			zap.Int64("user_id", userID),
		)
	} else {
		GetLogger().Warn("Login failed",
			zap.String("email_prefix", maskEmail(email)),
			zap.String("reason", "invalid_credentials"),
		)
	}
}

// LogAuthLogout logs logout event
func LogAuthLogout(userID int64) {
	GetLogger().Info("User logged out",
		zap.Int64("user_id", userID),
	)
}

// LogAuthSignup logs signup event
func LogAuthSignup(success bool, email string, userID int64) {
	GetLogger().Info("User signup",
		zap.String("email_prefix", maskEmail(email)),
		zap.Int64("user_id", userID),
		zap.Bool("success", success),
	)
}

// LogAuthTokenRefresh logs token refresh
func LogAuthTokenRefresh(success bool, userID int64) {
	GetLogger().Info("Token refreshed",
		zap.Int64("user_id", userID),
		zap.Bool("success", success),
	)
}

// LogAuthPasswordReset logs password reset request
func LogAuthPasswordReset(success bool, email string) {
	GetLogger().Info("Password reset requested",
		zap.String("email_prefix", maskEmail(email)),
		zap.Bool("success", success),
	)
}

// LogProfileUpdate logs profile updates
func LogProfileUpdate(userID int64, fields []string) {
	GetLogger().Info("Profile updated",
		zap.Int64("user_id", userID),
		zap.Strings("updated_fields", fields),
	)
}

// LogMovieOperation logs movie-related operations
func LogMovieOperation(operation string, userID int64, tmdbID int) {
	GetLogger().Debug(fmt.Sprintf("Movie %s", operation),
		zap.String("operation", operation),
		zap.Int64("user_id", userID),
		zap.Int("tmdb_id", tmdbID),
	)
}

// LogDBError logs database errors
func LogDBError(operation string, err error) {
	GetLogger().Error("Database error",
		zap.String("operation", operation),
		zap.Error(err),
	)
}

// LogExternalAPIError logs external API errors (TMDB, Redis, etc.)
func LogExternalAPIError(apiName, operation string, err error) {
	GetLogger().Error("External API error",
		zap.String("api", apiName),
		zap.String("operation", operation),
		zap.Error(err),
	)
}

// LogSecurityEvent logs security-related events
func LogSecurityEvent(event string, details map[string]interface{}) {
	fields := []zap.Field{zap.String("event", event)}
	for k, v := range details {
		fields = append(fields, zap.Any(k, v))
	}
	GetLogger().Warn("Security event", fields...)
}

// Sync flushes any buffered log entries
func Sync() {
	if log != nil {
		log.Sync()
	}
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

// StdLogger returns an io.Writer for third-party libraries that need standard logging
func StdLogger() io.Writer {
	return &zapWriter{}
}

type zapWriter struct{}

func (z *zapWriter) Write(p []byte) (n int, err error) {
	GetLogger().Info(strings.TrimSpace(string(p)))
	return len(p), nil
}
