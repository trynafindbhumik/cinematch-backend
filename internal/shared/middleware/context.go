package middleware

// User context helpers for extracting authenticated user data from gin context.

import "github.com/gin-gonic/gin"

// GetUserID extracts user ID from gin context.
// Returns 0 if user is not authenticated.
func GetUserID(c *gin.Context) int64 {
	if userID, exists := c.Get("userID"); exists {
		return userID.(int64)
	}
	return 0
}

// GetUserEmail extracts user email from gin context.
func GetUserEmail(c *gin.Context) string {
	if email, exists := c.Get("userEmail"); exists {
		return email.(string)
	}
	return ""
}

// GetUserRole extracts user role from gin context.
func GetUserRole(c *gin.Context) string {
	if role, exists := c.Get("userRole"); exists {
		return role.(string)
	}
	return ""
}

// GetUserPublicID extracts user public ID from gin context.
func GetUserPublicID(c *gin.Context) string {
	if pid, exists := c.Get("userPublicID"); exists {
		return pid.(string)
	}
	return ""
}

// IsAuthenticated checks if the request has a valid authenticated user.
func IsAuthenticated(c *gin.Context) bool {
	return GetUserID(c) != 0
}
