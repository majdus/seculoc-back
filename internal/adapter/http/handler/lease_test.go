package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestListLeases_Unauthorized(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewLeaseHandler(nil)
	r := gin.New()
	r.GET("/leases", h.List)

	req, _ := http.NewRequest("GET", "/leases", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestDownloadLease_InvalidID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewLeaseHandler(nil)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("userID", int32(123))
		c.Next()
	})
	r.GET("/leases/:id/download", h.Download)

	req, _ := http.NewRequest("GET", "/leases/abc/download", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}
