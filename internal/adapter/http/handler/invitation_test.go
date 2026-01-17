package handler

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestInviteTenant_Validation(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name       string
		payload    string
		expectCode int
	}{
		{
			name:       "Missing Email",
			payload:    `{"property_id": 1}`,
			expectCode: http.StatusBadRequest,
		},
		{
			name:       "Missing PropertyID",
			payload:    `{"email": "tenant@example.com"}`,
			expectCode: http.StatusBadRequest,
		},
		{
			name:       "Invalid Email",
			payload:    `{"email": "not-email", "property_id": 1}`,
			expectCode: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := NewInvitationHandler(nil)
			r := gin.New()
			r.Use(func(c *gin.Context) {
				c.Set("userID", int32(1))
				c.Next()
			})
			r.POST("/invitations", h.InviteTenant)

			req, _ := http.NewRequest("POST", "/invitations", bytes.NewBufferString(tt.payload))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			assert.Equal(t, tt.expectCode, w.Code)
		})
	}
}

func TestAcceptInvitation_Validation(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewInvitationHandler(nil)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("userID", int32(1))
		c.Next()
	})
	r.POST("/invitations/accept", h.AcceptInvitation)

	// Missing Token
	req, _ := http.NewRequest("POST", "/invitations/accept", bytes.NewBufferString(`{}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}
