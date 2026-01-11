package service

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"

	"seculoc-back/internal/adapter/storage/postgres"
)

func TestSubscribeUser_TransactionFailure_LogsError(t *testing.T) {
	// 1. Setup Logger Observer
	core, observedLogs := observer.New(zap.InfoLevel)
	testLogger := zap.New(core)

	// 2. Setup Mocks
	mockQuerier := new(MockQuerier)
	expectedErr := errors.New("db connection error")

	// We expect CreateSubscription to be called and fail
	mockQuerier.On("CreateSubscription", mock.Anything, mock.MatchedBy(func(arg postgres.CreateSubscriptionParams) bool {
		return arg.UserID.Int32 == 123 && arg.PlanType == postgres.SubPlanPremium
	})).Return(postgres.Subscription{}, expectedErr)

	// WithTx mock simply runs the function with our mockQuerier
	mockTx := new(MockTxManager)
	mockTx.On("WithTx", mock.Anything, mock.Anything).Return(expectedErr).Run(func(args mock.Arguments) {
		fn := args.Get(1).(func(postgres.Querier) error)
		_ = fn(mockQuerier)
	})

	// 3. Initialize Service
	svc := NewSubscriptionService(mockTx, testLogger)

	// 4. Execute
	ctx := context.Background()
	// Inject logger into context for the service to retrieve
	// We assume the service uses logger.FromContext which looks for "logger" key
	ctx = context.WithValue(ctx, "logger", testLogger)

	err := svc.SubscribeUser(ctx, 123, "premium", "monthly")

	// 5. Assertions
	assert.ErrorIs(t, err, expectedErr)

	// Verify Log
	logs := observedLogs.FilterMessage("subscription transaction failed")
	assert.Equal(t, 1, logs.Len(), "Expected 1 error log message")

	fieldMap := logs.All()[0].ContextMap()
	assert.Equal(t, int64(123), fieldMap["user_id"])
	assert.Equal(t, "premium", fieldMap["plan"])
	// Error field might be cleaner to check with assert.Contains if it's a string, or just presence
	assert.NotNil(t, fieldMap["error"])
}

func TestSubscribeUser_Success_LogsInfo(t *testing.T) {
	// 1. Setup Logger Observer
	core, observedLogs := observer.New(zap.InfoLevel)
	testLogger := zap.New(core)

	// 2. Setup Mocks
	mockQuerier := new(MockQuerier)

	// Expect CreateSubscription
	mockQuerier.On("CreateSubscription", mock.Anything, mock.MatchedBy(func(arg postgres.CreateSubscriptionParams) bool {
		return arg.UserID.Int32 == 123 && arg.PlanType == postgres.SubPlanPremium
	})).Return(postgres.Subscription{ID: 1}, nil)

	// Expect CreateCreditTransaction (since Premium plan has amount > 0)
	mockQuerier.On("CreateCreditTransaction", mock.Anything, mock.MatchedBy(func(arg postgres.CreateCreditTransactionParams) bool {
		return arg.UserID.Int32 == 123 && arg.Amount == 2990 && arg.TransactionType == "plan_purchase"
	})).Return(postgres.CreditTransaction{ID: 1}, nil)

	// WithTx mock
	mockTx := new(MockTxManager)
	mockTx.On("WithTx", mock.Anything, mock.Anything).Return(nil).Run(func(args mock.Arguments) {
		fn := args.Get(1).(func(postgres.Querier) error)
		_ = fn(mockQuerier)
	})

	// 3. Initialize Service
	svc := NewSubscriptionService(mockTx, testLogger)

	// 4. Execute
	ctx := context.Background()
	ctx = context.WithValue(ctx, "logger", testLogger)

	err := svc.SubscribeUser(ctx, 123, "premium", "monthly")

	// 5. Assertions
	assert.NoError(t, err)

	// Verify Log
	logs := observedLogs.FilterMessage("subscription created")
	assert.Equal(t, 1, logs.Len(), "Expected 1 info log message")

	fieldMap := logs.All()[0].ContextMap()
	assert.Equal(t, int64(123), fieldMap["user_id"])
	assert.Equal(t, "premium", fieldMap["plan"])
}

func TestIncreaseLimit_Success(t *testing.T) {
	// Setup
	mockTx := new(MockTxManager)
	mockQuerier := new(MockQuerier)
	svc := NewSubscriptionService(mockTx, zap.NewNop())
	ctx := context.Background()
	userID := int32(1)

	// Mock 1: Get Subscription (Must be Serenity or Premium)
	activeSub := postgres.Subscription{
		ID:       1,
		UserID:   pgtype.Int4{Int32: userID, Valid: true},
		PlanType: postgres.SubPlanSerenity,
		Status:   pgtype.Text{String: "active", Valid: true},
	}
	mockQuerier.On("GetUserSubscription", mock.Anything, pgtype.Int4{Int32: userID, Valid: true}).Return(activeSub, nil)

	// Mock 2: Update Limit
	mockQuerier.On("UpdateSubscriptionLimit", mock.Anything, mock.MatchedBy(func(arg postgres.UpdateSubscriptionLimitParams) bool {
		return arg.UserID.Int32 == userID && arg.MaxPropertiesLimit.Int32 == 1
	})).Return(nil)

	// WithTx
	mockTx.On("WithTx", mock.Anything, mock.Anything).Return(nil).Run(func(args mock.Arguments) {
		fn := args.Get(1).(func(postgres.Querier) error)
		_ = fn(mockQuerier)
	})

	// Execute
	err := svc.IncreaseLimit(ctx, userID, 1) // Add 1 slot

	// Assert
	assert.NoError(t, err)
}

func TestIncreaseLimit_NotEligible(t *testing.T) {
	// Setup
	mockTx := new(MockTxManager)
	mockQuerier := new(MockQuerier)
	svc := NewSubscriptionService(mockTx, zap.NewNop())
	ctx := context.Background()
	userID := int32(1)

	// Mock 1: Get Subscription (Discovery - Not Eligible)
	activeSub := postgres.Subscription{
		ID:       1,
		UserID:   pgtype.Int4{Int32: userID, Valid: true},
		PlanType: postgres.SubPlanDiscovery,
		Status:   pgtype.Text{String: "active", Valid: true},
	}
	mockQuerier.On("GetUserSubscription", mock.Anything, pgtype.Int4{Int32: userID, Valid: true}).Return(activeSub, nil)

	// WithTx
	mockTx.On("WithTx", mock.Anything, mock.Anything).Return(errors.New("plan not eligible")).Run(func(args mock.Arguments) {
		fn := args.Get(1).(func(postgres.Querier) error)
		// Simulating failure inside Tx logic
		_ = fn(mockQuerier)
	})

	err := svc.IncreaseLimit(ctx, userID, 1)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "plan not eligible")
}
