package auth

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
)

func TestToken_Success(t *testing.T) {
	viper.Set("JWT_SECRET", "supersecret")
	viper.Set("JWT_EXPIRATION_HOURS", 1)

	// 1. Generate
	token, err := GenerateToken(123, "test@example.com", "owner")
	assert.NoError(t, err)
	assert.NotEmpty(t, token)

	// 2. Validate
	claims, err := ValidateToken(token)
	assert.NoError(t, err)
	assert.NotNil(t, claims)
	assert.Equal(t, int32(123), claims.UserID)
	assert.Equal(t, "test@example.com", claims.Email)
	assert.Equal(t, "owner", claims.CurrentContext)
}

func TestToken_MissingSecret(t *testing.T) {
	viper.Set("JWT_SECRET", "")

	_, err := GenerateToken(1, "mail", "ctx")
	assert.Error(t, err)

	_, err = ValidateToken("some.token")
	assert.Error(t, err)
}

func TestToken_Expired(t *testing.T) {
	viper.Set("JWT_SECRET", "secret")

	// Create expired token manually
	expirationTime := time.Now().Add(-1 * time.Hour)
	claims := &Claims{
		UserID: 1,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expirationTime),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, _ := token.SignedString([]byte("secret"))

	_, err := ValidateToken(tokenString)
	assert.Error(t, err) // Should fail due to expiration
}

func TestToken_InvalidSignature(t *testing.T) {
	viper.Set("JWT_SECRET", "secret")
	token, _ := GenerateToken(1, "mail", "ctx")

	viper.Set("JWT_SECRET", "wrongcheck")
	_, err := ValidateToken(token)
	assert.Error(t, err)
}
