package handler

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestSubscribe_Validation(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name       string
		payload    string
		expectCode int
	}{
		{
			name:       "Missing Fields",
			payload:    `{}`,
			expectCode: http.StatusBadRequest,
		},
		{
			name:       "Invalid Plan",
			payload:    `{"plan": "ultra-pro", "frequency": "monthly"}`,
			expectCode: http.StatusBadRequest,
		},
		{
			name:       "Invalid Frequency",
			payload:    `{"plan": "premium", "frequency": "hourly"}`,
			expectCode: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := NewSubscriptionHandler(nil, nil)
			r := gin.New()
			r.Use(func(c *gin.Context) {
				c.Set("userID", int32(1))
				c.Next()
			})
			r.POST("/subscriptions", h.Subscribe)

			req, _ := http.NewRequest("POST", "/subscriptions", bytes.NewBufferString(tt.payload))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			assert.Equal(t, tt.expectCode, w.Code)
		})
	}
}

func TestIncreaseLimit_Validation(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewSubscriptionHandler(nil, nil)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("userID", int32(1))
		c.Next()
	})
	r.POST("/subscriptions/upgrade", h.IncreaseLimit)

	// Invalid Slots (0)
	req, _ := http.NewRequest("POST", "/subscriptions/upgrade", bytes.NewBufferString(`{"additional_slots": 0}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}
