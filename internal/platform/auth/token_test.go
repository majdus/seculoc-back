package auth

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
)

func TestGenerateToken_RespectsExpirationConfig(t *testing.T) {
	// Setup
	viper.Set("JWT_SECRET", "test_secret")
	viper.Set("JWT_EXPIRATION_HOURS", 2)
	defer viper.Reset()

	// Execute
	tokenString, err := GenerateToken(1, "test@example.com", "owner")
	assert.NoError(t, err)

	// Verify
	token, _ := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		return []byte("test_secret"), nil
	})

	claims, ok := token.Claims.(*Claims)
	assert.True(t, ok)

	// Check expiration is roughly 2 hours from now
	expectedExp := time.Now().Add(2 * time.Hour)
	assert.WithinDuration(t, expectedExp, claims.ExpiresAt.Time, 5*time.Second)
}

func TestGenerateToken_DefaultExpiration(t *testing.T) {
	// Setup
	viper.Reset()
	viper.Set("JWT_SECRET", "test_secret")
	// No expiration set

	// Execute
	tokenString, err := GenerateToken(1, "test@example.com", "owner")
	assert.NoError(t, err)

	// Verify
	token, _ := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		return []byte("test_secret"), nil
	})

	claims, ok := token.Claims.(*Claims)
	assert.True(t, ok)

	// Check default is 24 hours
	expectedExp := time.Now().Add(24 * time.Hour)
	assert.WithinDuration(t, expectedExp, claims.ExpiresAt.Time, 5*time.Second)
}
