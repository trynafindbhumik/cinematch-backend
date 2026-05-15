package middleware

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// TimeoutConfig holds configuration for the timeout middleware
type TimeoutConfig struct {
	Timeout time.Duration
}

// Timeout returns a middleware that adds a custom timeout for specific paths
func Timeout(config TimeoutConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Wrap the response writer with a timeout-aware writer
		done := make(chan struct{})

		go func() {
			c.Next()
			close(done)
		}()

		select {
		case <-done:
			// Request completed normally
		case <-time.After(config.Timeout):
			c.AbortWithStatusJSON(http.StatusGatewayTimeout, gin.H{
				"error": "request timeout",
			})
		}
	}
}