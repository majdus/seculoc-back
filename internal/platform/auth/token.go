package auth

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/spf13/viper"
)

type Claims struct {
	UserID int32  `json:"user_id"`
	Email  string `json:"email"`
	jwt.RegisteredClaims
}

var ErrInvalidToken = errors.New("invalid token")

// GenerateToken generates a JWT token for the user.
func GenerateToken(userID int32, email string) (string, error) {
	secret := viper.GetString("JWT_SECRET")
	if secret == "" {
		return "", errors.New("JWT_SECRET not setup")
	}

	expirationTime := time.Now().Add(24 * time.Hour)
	claims := &Claims{
		UserID: userID,
		Email:  email,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expirationTime),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    "seculoc",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}

// ValidateToken parses and validates the token.
func ValidateToken(tokenString string) (*Claims, error) {
	secret := viper.GetString("JWT_SECRET")
	if secret == "" {
		return nil, errors.New("JWT_SECRET not setup")
	}

	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, ErrInvalidToken
		}
		return []byte(secret), nil
	})

	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(*Claims); ok && token.Valid {
		return claims, nil
	}

	return nil, ErrInvalidToken
}
