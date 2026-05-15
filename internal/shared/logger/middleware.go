package logger

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

/*
Request Logging Middleware

This middleware logs all HTTP requests with:
- Request ID (UUID)
- Method and path
- Status code
- Duration
- Client IP
- User ID (if authenticated)
- User Agent
- Correlation ID (for distributed tracing)

Log Format (JSON for production, console for development):
{
  "level": "info",
  "timestamp": "2024-01-15T10:30:00.000Z",
  "message": "HTTP Request",
  "request_id": "550e8400-e29b-41d4-a716-446655440000",
  "method": "POST",
  "path": "/v1/auth/login",
  "status": 200,
  "duration_ms": 45,
  "client_ip": "192.168.1.1",
  "user_id": 123,
  "user_agent": "Mozilla/5.0..."
}
*/

const (
	// RequestIDHeader is the header name for request ID
	RequestIDHeader = "X-Request-ID"
	// CorrelationIDHeader is the header name for correlation ID
	CorrelationIDHeader = "X-Correlation-ID"
	// RequestIDKey is the context key for request ID
	RequestIDKey = "requestID"
	// StartTimeKey is the context key for request start time
	StartTimeKey = "startTime"
)

// RequestLogger returns a gin middleware for request logging
func RequestLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Generate or extract request ID
		requestID := c.GetHeader(RequestIDHeader)
		if requestID == "" {
			requestID = uuid.New().String()
		}

		// Extract correlation ID
		correlationID := c.GetHeader(CorrelationIDHeader)

		// Set start time
		startTime := time.Now()

		// Store in context and headers
		c.Set(RequestIDKey, requestID)
		c.Header(RequestIDHeader, requestID)
		if correlationID != "" {
			c.Header(CorrelationIDHeader, correlationID)
		}

		// Store start time for duration calculation
		c.Set(StartTimeKey, startTime)

		// Process request
		c.Next()

		// Calculate duration
		duration := time.Since(startTime)

		// Get user ID if authenticated
		userID, _ := c.Get("userID")
		var userIDInt int64
		if uid, ok := userID.(int64); ok {
			userIDInt = uid
		}

		// Get status
		status := c.Writer.Status()

		// Log the request
		fields := []zap.Field{
			zap.String("request_id", requestID),
			zap.String("method", c.Request.Method),
			zap.String("path", c.Request.URL.Path),
			zap.Int("status", status),
			zap.Duration("duration", duration),
			zap.String("client_ip", c.ClientIP()),
			zap.String("user_agent", c.Request.UserAgent()),
			zap.Int("body_size", c.Writer.Size()),
		}

		if correlationID != "" {
			fields = append(fields, zap.String("correlation_id", correlationID))
		}

		if userIDInt > 0 {
			fields = append(fields, zap.Int64("user_id", userIDInt))
		}

		// Add error if present
		if len(c.Errors) > 0 {
			for _, e := range c.Errors {
				fields = append(fields, zap.String("gin_error", e.Error()))
			}
		}

		// Log based on status code
		msg := "HTTP Request"
		switch {
		case status >= 500:
			GetLogger().Error(msg, fields...)
		case status >= 400:
			GetLogger().Warn(msg, fields...)
		default:
			GetLogger().Info(msg, fields...)
		}
	}
}

// GetRequestID extracts request ID from gin context
func GetRequestID(c *gin.Context) string {
	if reqID, exists := c.Get(RequestIDKey); exists {
		return reqID.(string)
	}
	return ""
}

// GetStartTime extracts start time from gin context
func GetStartTime(c *gin.Context) time.Time {
	if startTime, exists := c.Get(StartTimeKey); exists {
		return startTime.(time.Time)
	}
	return time.Now()
}
