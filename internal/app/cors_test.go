package app

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
)

func TestCORS_Integration(t *testing.T) {
	// Setup Viper
	frontendURL := "http://allowed-origin.com"
	viper.Set("FRONTEND_URL", frontendURL)
	viper.Set("GIN_MODE", "test")

	// Create Router and Configure CORS using the helper from app.go
	r := gin.New()
	configureCORS(r)

	// Add a dummy endpoint to test simple requests
	r.GET("/ping", func(c *gin.Context) {
		c.String(200, "pong")
	})

	t.Run("Allowed Origin - Preflight (OPTIONS)", func(t *testing.T) {
		req := httptest.NewRequest("OPTIONS", "/ping", nil)
		req.Header.Set("Origin", frontendURL)
		req.Header.Set("Access-Control-Request-Method", "GET")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNoContent, w.Code)
		assert.Equal(t, frontendURL, w.Header().Get("Access-Control-Allow-Origin"))
		assert.Contains(t, w.Header().Get("Access-Control-Allow-Methods"), "GET")
		assert.Equal(t, "true", w.Header().Get("Access-Control-Allow-Credentials"))
	})

	t.Run("Allowed Origin - Simple Request (GET)", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/ping", nil)
		req.Header.Set("Origin", frontendURL)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, frontendURL, w.Header().Get("Access-Control-Allow-Origin"))
		assert.Equal(t, "true", w.Header().Get("Access-Control-Allow-Credentials"))
	})

	t.Run("Disallowed Origin", func(t *testing.T) {
		req := httptest.NewRequest("OPTIONS", "/ping", nil)
		req.Header.Set("Origin", "http://evil.com")
		req.Header.Set("Access-Control-Request-Method", "GET")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		// With gin-contrib/cors default config + restrict origin:
		// If origin is not allowed, it effectively ignores the CORS headers or returns 403 depending on configuration.
		// By default it acts as if CORS is not enabled for that origin (no Allow-Origin header).
		// Wait, DefaultConfig() + AllowOrigins restricts it.
		// Let's verify what happens. Usually it just doesn't send the headers back.
		// If it's a preflight and fails, it might return 403.

		assert.Equal(t, http.StatusForbidden, w.Code)
	})
}
