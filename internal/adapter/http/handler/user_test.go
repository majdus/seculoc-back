package handler

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestRegister_Validation(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name       string
		payload    string
		expectCode int
	}{
		{
			name:       "Missing Fields",
			payload:    `{"email": "test@example.com"}`,
			expectCode: http.StatusBadRequest,
		},
		{
			name:       "Invalid Email",
			payload:    `{"email": "invalid-email", "password": "password123", "first_name": "John", "last_name": "Doe", "phone": "123"}`,
			expectCode: http.StatusBadRequest,
		},
		{
			name:       "Short Password",
			payload:    `{"email": "test@example.com", "password": "short", "first_name": "John", "last_name": "Doe", "phone": "123"}`,
			expectCode: http.StatusBadRequest,
		},
		// Nominal case cannot be tested without mocking Service,
		// but we are targeting "Validation Logic" as per audit.
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup Handler with nil service (should validation fail before using it?)
			// Yes, ShouldBindJSON happens first.
			h := NewUserHandler(nil)
			r := gin.New()
			r.POST("/register", h.Register)

			req, _ := http.NewRequest("POST", "/register", bytes.NewBufferString(tt.payload))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			assert.Equal(t, tt.expectCode, w.Code)
		})
	}
}

func TestLogin_Validation(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name       string
		payload    string
		expectCode int
	}{
		{
			name:       "Missing Password",
			payload:    `{"email": "test@example.com"}`,
			expectCode: http.StatusBadRequest,
		},
		{
			name:       "Invalid Email Format",
			payload:    `{"email": "not-an-email", "password": "password"}`,
			expectCode: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := NewUserHandler(nil)
			r := gin.New()
			r.POST("/login", h.Login)

			req, _ := http.NewRequest("POST", "/login", bytes.NewBufferString(tt.payload))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			assert.Equal(t, tt.expectCode, w.Code)
		})
	}
}

func TestSwitchContext_Validation(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name       string
		payload    string
		expectCode int
	}{
		{
			name:       "Invalid Context",
			payload:    `{"target_context": "admin"}`, // Allowed are owner/tenant
			expectCode: http.StatusBadRequest,
		},
		{
			name:       "Missing Target",
			payload:    `{}`,
			expectCode: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := NewUserHandler(nil)
			r := gin.New()
			// Mock Auth Middleware behavior by setting context
			r.Use(func(c *gin.Context) {
				c.Set("userID", int32(1))
				c.Next()
			})
			r.POST("/switch", h.SwitchContext)

			req, _ := http.NewRequest("POST", "/switch", bytes.NewBufferString(tt.payload))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			assert.Equal(t, tt.expectCode, w.Code)
		})
	}
}
