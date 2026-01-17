package middleware

import (
	"github.com/gin-gonic/gin"
)

// SecurityHeadersMiddleware adds recommended security headers to every response.
func SecurityHeadersMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Prevent clickjacking
		c.Header("X-Frame-Options", "DENY")

		// Prevent MIME sniffing
		c.Header("X-Content-Type-Options", "nosniff")

		// Enable XSS protection in older browsers
		c.Header("X-XSS-Protection", "1; mode=block")

		// Content Security Policy (Basic restrictive policy)
		// We allow 'self' and maybe data: for images, but for an API 'default-src none' might be too strict if it serves Swagger UI.
		// Since we serve Swagger UI, we need to allow 'self' and inline scripts/styles used by Swagger.
		// For an API serving mostly JSON, 'default-src 'none'; frame-ancestors 'none';' is good practice but might break Swagger.
		// Let's stick to essential headers for now.
		// c.Header("Content-Security-Policy", "default-src 'self'")

		c.Next()
	}
}
