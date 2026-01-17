package auth

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHashPassword(t *testing.T) {
	password := "securepassword"
	hash, err := HashPassword(password)
	assert.NoError(t, err)
	assert.NotEmpty(t, hash)
	assert.Contains(t, hash, "$argon2id$")
}

func TestCheckPasswordHash(t *testing.T) {
	password := "securepassword"
	hash, _ := HashPassword(password)

	match, err := CheckPasswordHash(password, hash)
	assert.NoError(t, err)
	assert.True(t, match)

	match, err = CheckPasswordHash("wrongpassword", hash)
	assert.NoError(t, err)
	assert.False(t, match)
}

func TestCheckPasswordHash_InvalidFormat(t *testing.T) {
	match, err := CheckPasswordHash("password", "invalidhash")
	assert.Error(t, err)
	assert.False(t, match)
}
