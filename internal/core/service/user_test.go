package service

import (
	"context"
	"errors"
	"fmt"
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
	mockTx := new(MockTxManager)

	// Wire Tx
	mockTx.On("WithTx", mock.Anything, mock.Anything).Return(nil).Run(func(args mock.Arguments) {
		fn := args.Get(1).(func(postgres.Querier) error)
		_ = fn(mockQuerier)
	})

	svc := NewUserService(mockTx, testLogger)

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
	user, err := svc.Register(context.Background(), "test@example.com", "password123", "John", "Doe", "0611223344", "")

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
	mockTx := new(MockTxManager)

	// Wire Tx - It will fail inside, so WithTx returns error?
	// Actually the service logic returns error, so WithTx bubbles it up.
	mockTx.On("WithTx", mock.Anything, mock.Anything).Return(fmt.Errorf("user with email test@example.com already exists")).Run(func(args mock.Arguments) {
		fn := args.Get(1).(func(postgres.Querier) error)
		_ = fn(mockQuerier)
	})

	svc := NewUserService(mockTx, testLogger)

	// 2. Mocks
	// GetUserByEmail returns a user (conflict)
	existingUser := postgres.User{ID: 1, Email: "test@example.com"}
	mockQuerier.On("GetUserByEmail", mock.Anything, "test@example.com").Return(existingUser, nil)

	// 3. Execution
	_, err := svc.Register(context.Background(), "test@example.com", "password123", "John", "Doe", "0611223344", "")

	// 4. Assertion
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already exists")
	mockQuerier.AssertNotCalled(t, "CreateUser")
}

func TestLogin_Success(t *testing.T) {
	// Setup
	mockQuerier := new(MockQuerier)
	mockTx := new(MockTxManager)

	mockTx.On("WithTx", mock.Anything, mock.Anything).Return(nil).Run(func(args mock.Arguments) {
		fn := args.Get(1).(func(postgres.Querier) error)
		_ = fn(mockQuerier)
	})

	svc := NewUserService(mockTx, zap.NewNop())

	// Mock
	existingUser := postgres.User{ID: 1, Email: "test@example.com", PasswordHash: "hashed_password123"}
	// Mocks for Login
	mockQuerier.On("GetUserByEmail", mock.Anything, "test@example.com").Return(existingUser, nil)
	mockQuerier.On("CountPropertiesByOwner", mock.Anything, mock.AnythingOfType("pgtype.Int4")).Return(int64(1), nil)
	mockQuerier.On("CountLeasesByTenant", mock.Anything, mock.AnythingOfType("pgtype.Int4")).Return(int64(0), nil)
	mockQuerier.On("CountBookingsByTenant", mock.Anything, mock.AnythingOfType("pgtype.Int4")).Return(int64(0), nil)
	mockQuerier.On("GetUserSubscription", mock.Anything, mock.AnythingOfType("pgtype.Int4")).Return(postgres.Subscription{}, pgx.ErrNoRows) // No subscription
	mockQuerier.On("GetUserCreditBalance", mock.Anything, mock.AnythingOfType("pgtype.Int4")).Return(int32(10), nil)                        // 10 credits

	// Execute
	resp, err := svc.Login(context.Background(), "test@example.com", "password123")

	// Assert
	assert.NoError(t, err)
	assert.Equal(t, int32(1), resp.User.ID)
	assert.True(t, resp.Capabilities.CanActAsOwner)
	assert.False(t, resp.Capabilities.CanActAsTenant)
	assert.Equal(t, ContextOwner, resp.CurrentContext)
}

func TestLogin_InvalidCredentials(t *testing.T) {

	// Case 1: User Not Found
	// Mock Tx for Case 1
	mockTx2 := new(MockTxManager)
	mockQuerier2 := new(MockQuerier)
	svc2 := NewUserService(mockTx2, zap.NewNop())

	mockTx2.On("WithTx", mock.Anything, mock.Anything).Return(errors.New("invalid credentials")).Run(func(args mock.Arguments) {
		fn := args.Get(1).(func(postgres.Querier) error)
		_ = fn(mockQuerier2)
	})

	mockQuerier2.On("GetUserByEmail", mock.Anything, "unknown@example.com").Return(postgres.User{}, pgx.ErrNoRows)

	_, err := svc2.Login(context.Background(), "unknown@example.com", "password")
	assert.Error(t, err)
	assert.Equal(t, "invalid credentials", err.Error())

	// Case 2: Wrong Password
	mockTx3 := new(MockTxManager)
	mockQuerier3 := new(MockQuerier)
	svc3 := NewUserService(mockTx3, zap.NewNop())

	mockTx3.On("WithTx", mock.Anything, mock.Anything).Return(nil).Run(func(args mock.Arguments) {
		fn := args.Get(1).(func(postgres.Querier) error)
		_ = fn(mockQuerier3)
	})

	existingUser := postgres.User{ID: 1, Email: "test@example.com", PasswordHash: "hashed_password123"}
	mockQuerier3.On("GetUserByEmail", mock.Anything, "test@example.com").Return(existingUser, nil)

	// Mock capability checks (Login proceeds to check these even if password will fail later, as they are in the same Tx block)
	mockQuerier3.On("CountPropertiesByOwner", mock.Anything, pgtype.Int4{Int32: 1, Valid: true}).Return(int64(0), nil)
	mockQuerier3.On("CountLeasesByTenant", mock.Anything, pgtype.Int4{Int32: 1, Valid: true}).Return(int64(0), nil)
	mockQuerier3.On("CountBookingsByTenant", mock.Anything, pgtype.Int4{Int32: 1, Valid: true}).Return(int64(0), nil)
	mockQuerier3.On("GetUserSubscription", mock.Anything, mock.AnythingOfType("pgtype.Int4")).Return(postgres.Subscription{}, pgx.ErrNoRows)
	mockQuerier3.On("GetUserCreditBalance", mock.Anything, mock.AnythingOfType("pgtype.Int4")).Return(int32(0), nil)

	_, err = svc3.Login(context.Background(), "test@example.com", "wrongpassword")
	assert.Error(t, err)
	assert.Equal(t, "invalid credentials", err.Error())
}
