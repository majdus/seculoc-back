package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func TestRequestLogger(t *testing.T) {
	// 1. Setup Logger Observer
	// We need to override the singleton logger because middleware uses logger.Get() fallback
	// or relies on logger.Get() to create the initial context logger.
	// For this test, let's just inspect what the middleware puts in the context and what it logs.

	// Note: We can't easily swap the global logger in `logger` package without a SetLogger method.
	// Let's assume for now we verify the Context injection primarily.
	// The key `LoggerKey` is public. We can check if logger is in context.

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(RequestLogger())

	r.GET("/test", func(c *gin.Context) {
		// Verify Request ID
		rid := c.GetString("request_id")
		assert.NotEmpty(t, rid, "Request ID should be in context")

		// Verify logger in Gin context
		l, exists := c.Get(LoggerKey)
		assert.True(t, exists, "Logger should be in Gin context")
		assert.IsType(t, &zap.Logger{}, l)

		c.Status(http.StatusOK)
	})

	// Setup request
	req, _ := http.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	// Execute
	r.ServeHTTP(w, req)

	// Verify Response
	assert.Equal(t, http.StatusOK, w.Code)
	assert.NotEmpty(t, w.Header().Get("X-Request-ID"), "X-Request-ID header should be present in response")
}

func TestGetLogger(t *testing.T) {
	// Test extraction
	l := zap.NewNop()
	ctx := context.WithValue(context.Background(), LoggerKey, l)

	got := GetLogger(ctx)
	assert.Equal(t, l, got)

	// Test fallback
	gotFallback := GetLogger(context.Background())
	assert.NotNil(t, gotFallback)
}
