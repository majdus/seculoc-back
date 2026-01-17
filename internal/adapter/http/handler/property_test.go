package handler

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestCreateProperty_Validation(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name       string
		payload    string
		expectCode int
	}{
		{
			name:       "Missing Address",
			payload:    `{"rental_type": "long_term", "details": {"size": 50}}`,
			expectCode: http.StatusBadRequest,
		},
		{
			name:       "Missing Rental Type",
			payload:    `{"address": "123 Rue de la Paix", "details": {"size": 50}}`,
			expectCode: http.StatusBadRequest,
		},
		{
			name:       "Invalid Rental Type",
			payload:    `{"address": "123 Rue de la Paix", "rental_type": "invalid", "details": {"size": 50}}`,
			expectCode: http.StatusBadRequest,
		},
		{
			name:       "Missing Details",
			payload:    `{"address": "123 Rue de la Paix", "rental_type": "long_term"}`,
			expectCode: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := NewPropertyHandler(nil) // Service not needed for binding failure
			r := gin.New()
			// Mock Middleware Set User
			r.Use(func(c *gin.Context) {
				c.Set("userID", int32(1))
				c.Next()
			})
			r.POST("/properties", h.Create)

			req, _ := http.NewRequest("POST", "/properties", bytes.NewBufferString(tt.payload))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			assert.Equal(t, tt.expectCode, w.Code)
		})
	}
}

func TestUpdateProperty_Validation(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name       string
		payload    string
		expectCode int
	}{
		{
			name:       "Invalid Rental Type",
			payload:    `{"rental_type": "invalid"}`,
			expectCode: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := NewPropertyHandler(nil)
			r := gin.New()
			r.Use(func(c *gin.Context) {
				c.Set("userID", int32(1))
				c.Next()
			})
			r.PUT("/properties/:id", h.Update)

			req, _ := http.NewRequest("PUT", "/properties/1", bytes.NewBufferString(tt.payload))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			assert.Equal(t, tt.expectCode, w.Code)
		})
	}
}
