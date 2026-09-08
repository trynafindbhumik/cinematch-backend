package profile

// Authentication middleware validating JWT tokens.

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	jwtutil "github.com/trynafindbhumik/cinematch-backend/internal/shared/jwt"
	"github.com/trynafindbhumik/cinematch-backend/internal/shared/middleware"
)

// AuthMiddleware validates JWT token and sets user context.
func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "missing authorization header"})
			c.Abort()
			return
		}

		tokenString := strings.TrimPrefix(authHeader, "Bearer ")

		// Fast local JWT validation
		claims, err := jwtutil.ValidateAccessToken(tokenString)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
			c.Abort()
			return
		}

		// Token is valid - set user context for downstream handlers
		c.Set("userID", claims.UserID)
		c.Set("userEmail", claims.Email)
		c.Set("userRole", claims.Role)
		c.Set("isVerified", claims.IsVerified)
		c.Set("isFirstLogin", claims.IsFirstLogin)
		c.Set("userPublicID", fmt.Sprintf("usr_%d", claims.UserID))

		c.Next()
	}
}

// AuthMiddlewareOptional validates JWT token but doesn't require it.
// Used for endpoints that behave differently for authenticated vs anonymous users.
func AuthMiddlewareOptional() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
			c.Next()
			return
		}

		tokenString := strings.TrimPrefix(authHeader, "Bearer ")

		// Fast local JWT validation
		claims, err := jwtutil.ValidateAccessToken(tokenString)
		if err != nil {
			c.Next()
			return
		}

		// Set user context for authenticated requests
		c.Set("userID", claims.UserID)
		c.Set("userEmail", claims.Email)
		c.Set("userRole", claims.Role)
		c.Set("isVerified", claims.IsVerified)
		c.Set("isFirstLogin", claims.IsFirstLogin)
		c.Set("userPublicID", fmt.Sprintf("usr_%d", claims.UserID))

		c.Next()
	}
}

// GetUserID delegates to shared middleware package.
func GetUserID(c *gin.Context) int64 {
	return middleware.GetUserID(c)
}

// GetUserEmail delegates to shared middleware package.
func GetUserEmail(c *gin.Context) string {
	return middleware.GetUserEmail(c)
}

// GetUserRole delegates to shared middleware package.
func GetUserRole(c *gin.Context) string {
	return middleware.GetUserRole(c)
}

// GetUserPublicID delegates to shared middleware package.
func GetUserPublicID(c *gin.Context) string {
	return middleware.GetUserPublicID(c)
}
