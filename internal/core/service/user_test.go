package service

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"

	"seculoc-back/internal/adapter/storage/postgres"
)

func TestRegisterUser_Success(t *testing.T) {
	// 1. Setup
	core, _ := observer.New(zap.InfoLevel)
	testLogger := zap.New(core)
	mockQuerier := new(MockQuerier)
	svc := NewUserService(mockQuerier, testLogger)

	// 2. Mocks
	// GetUserByEmail should return NoRows (user does not exist)
	mockQuerier.On("GetUserByEmail", mock.Anything, "test@example.com").Return(postgres.User{}, pgx.ErrNoRows)

	// CreateUser should succeed
	expectedUser := postgres.User{
		ID:        1,
		Email:     "test@example.com",
		FirstName: pgtype.Text{String: "John", Valid: true},
		LastName:  pgtype.Text{String: "Doe", Valid: true},
	}
	mockQuerier.On("CreateUser", mock.Anything, mock.AnythingOfType("postgres.CreateUserParams")).Return(expectedUser, nil)

	// 3. Execution
	user, err := svc.Register(context.Background(), "test@example.com", "password123", "John", "Doe", "0611223344")

	// 4. Assertion
	assert.NoError(t, err)
	assert.Equal(t, int32(1), user.ID)
	mockQuerier.AssertExpectations(t)
}

func TestRegisterUser_AlreadyExists(t *testing.T) {
	// 1. Setup
	core, _ := observer.New(zap.InfoLevel)
	testLogger := zap.New(core)
	mockQuerier := new(MockQuerier)
	svc := NewUserService(mockQuerier, testLogger)

	// 2. Mocks
	// GetUserByEmail returns a user (conflict)
	existingUser := postgres.User{ID: 1, Email: "test@example.com"}
	mockQuerier.On("GetUserByEmail", mock.Anything, "test@example.com").Return(existingUser, nil)

	// 3. Execution
	_, err := svc.Register(context.Background(), "test@example.com", "password123", "John", "Doe", "0611223344")

	// 4. Assertion
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already exists")
	mockQuerier.AssertNotCalled(t, "CreateUser")
}

func TestLogin_Success(t *testing.T) {
	// Setup
	mockQuerier := new(MockQuerier)
	svc := NewUserService(mockQuerier, zap.NewNop())

	// Mock
	existingUser := postgres.User{ID: 1, Email: "test@example.com", PasswordHash: "hashed_password123"}
	mockQuerier.On("GetUserByEmail", mock.Anything, "test@example.com").Return(existingUser, nil)

	// Execute
	user, err := svc.Login(context.Background(), "test@example.com", "password123")

	// Assert
	assert.NoError(t, err)
	assert.Equal(t, int32(1), user.ID)
}

func TestLogin_InvalidCredentials(t *testing.T) {
	// Setup
	mockQuerier := new(MockQuerier)
	svc := NewUserService(mockQuerier, zap.NewNop())

	// Mock 1: User not found
	mockQuerier.On("GetUserByEmail", mock.Anything, "unknown@example.com").Return(postgres.User{}, pgx.ErrNoRows)

	_, err := svc.Login(context.Background(), "unknown@example.com", "password")
	assert.Error(t, err)
	assert.Equal(t, "invalid credentials", err.Error())

	// Mock 2: Wrong Password
	existingUser := postgres.User{ID: 1, Email: "test@example.com", PasswordHash: "hashed_password123"}
	mockQuerier.On("GetUserByEmail", mock.Anything, "test@example.com").Return(existingUser, nil)

	_, err = svc.Login(context.Background(), "test@example.com", "wrongpassword")
	assert.Error(t, err)
	assert.Equal(t, "invalid credentials", err.Error())
}
