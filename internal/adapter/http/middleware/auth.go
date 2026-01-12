package middleware

import (
	"net/http"
	"strings"

	"seculoc-back/internal/platform/auth"
	"seculoc-back/internal/platform/logger"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// AuthMiddleware ensures that the request has a valid JWT token.
func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "authorization header required"})
			return
		}

		// Bearer <token>
		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || parts[0] != "Bearer" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid authorization header format"})
			return
		}

		tokenString := parts[1]
		claims, err := auth.ValidateToken(tokenString)
		if err != nil {
			log := logger.FromContext(c.Request.Context())
			log.Warn("invalid token", zap.Error(err))
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
			return
		}

		// Store user ID in context
		c.Set("userID", claims.UserID)
		c.Set("email", claims.Email)

		// Also update the request context logger to include UserID for subsequent logs
		// This is tricky because we replaced the request context logger in RequestLogger middleware
		// But Gin context and Request context are linked.
		// Let's rely on c.Get("userID") in Handlers.

		c.Next()
	}
}

// GetUserID retrieves the user ID from the Gin context.
func GetUserID(c *gin.Context) (int32, bool) {
	val, exists := c.Get("userID")
	if !exists {
		return 0, false
	}
	id, ok := val.(int32)
	return id, ok
}
