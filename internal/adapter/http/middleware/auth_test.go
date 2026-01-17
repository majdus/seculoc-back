package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"seculoc-back/internal/platform/auth"

	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
)

func TestValidateToken_Success(t *testing.T) {
	// Setup Gin
	gin.SetMode(gin.TestMode)
	r := gin.New()

	// Setup Viper
	viper.Set("JWT_SECRET", "testsecret")
	viper.Set("JWT_EXPIRATION_HOURS", 24)

	// Generate Valid Token
	token, _ := auth.GenerateToken(1, "test@example.com", "owner")

	// Apply Middleware
	r.Use(AuthMiddleware())
	r.GET("/protected", func(c *gin.Context) {
		userID, _ := c.Get("userID")
		email, _ := c.Get("email")
		c.JSON(http.StatusOK, gin.H{
			"userID": userID,
			"email":  email,
		})
	})

	// Request
	req, _ := http.NewRequest("GET", "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// Assert
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestValidateToken_MissingHeader(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(AuthMiddleware())
	r.GET("/protected", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req, _ := http.NewRequest("GET", "/protected", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestValidateToken_InvalidFormat(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(AuthMiddleware())
	r.GET("/protected", func(c *gin.Context) { c.Status(http.StatusOK) })

	req, _ := http.NewRequest("GET", "/protected", nil)
	req.Header.Set("Authorization", "Basic user:pass") // Wrong Scheme
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestValidateToken_InvalidToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	viper.Set("JWT_SECRET", "testsecret")
	r := gin.New()
	r.Use(AuthMiddleware())
	r.GET("/protected", func(c *gin.Context) { c.Status(http.StatusOK) })

	req, _ := http.NewRequest("GET", "/protected", nil)
	req.Header.Set("Authorization", "Bearer invalid.token.string")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}
