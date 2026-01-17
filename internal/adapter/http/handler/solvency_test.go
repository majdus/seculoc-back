package handler

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestInitiateCheck_Validation(t *testing.T) {
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
			payload:    `{"candidate_email": "test@example.com"}`,
			expectCode: http.StatusBadRequest,
		},
		{
			name:       "Invalid Email",
			payload:    `{"candidate_email": "not-email", "property_id": 1}`,
			expectCode: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := NewSolvencyHandler(nil)
			r := gin.New()
			r.Use(func(c *gin.Context) {
				c.Set("userID", int32(1))
				c.Next()
			})
			r.POST("/solvency/check", h.CreateCheck)

			req, _ := http.NewRequest("POST", "/solvency/check", bytes.NewBufferString(tt.payload))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			assert.Equal(t, tt.expectCode, w.Code)
		})
	}
}

func TestPurchaseCredits_Validation(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewSolvencyHandler(nil)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("userID", int32(1))
		c.Next()
	})
	r.POST("/solvency/credits", h.BuyCredits)

	// Invalid Pack
	req, _ := http.NewRequest("POST", "/solvency/credits", bytes.NewBufferString(`{"pack_type": "invalid"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)

	// Missing field handled by oneof validation or required
}
