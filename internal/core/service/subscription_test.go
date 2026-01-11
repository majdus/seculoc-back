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

// MockQuerier is a mock of the sqlc Querier interface
type MockQuerier struct {
	mock.Mock
}

func (m *MockQuerier) CreateSubscription(ctx context.Context, arg postgres.CreateSubscriptionParams) (postgres.Subscription, error) {
	args := m.Called(ctx, arg)
	return args.Get(0).(postgres.Subscription), args.Error(1)
}

func (m *MockQuerier) CreateCreditTransaction(ctx context.Context, arg postgres.CreateCreditTransactionParams) (postgres.CreditTransaction, error) {
	args := m.Called(ctx, arg)
	return args.Get(0).(postgres.CreditTransaction), args.Error(1)
}

func (m *MockQuerier) GetUserCreditBalance(ctx context.Context, userID pgtype.Int4) (int32, error) {
	args := m.Called(ctx, userID)
	return args.Get(0).(int32), args.Error(1)
}

func (m *MockQuerier) GetUserSubscription(ctx context.Context, userID pgtype.Int4) (postgres.Subscription, error) {
	args := m.Called(ctx, userID)
	return args.Get(0).(postgres.Subscription), args.Error(1)
}

// MockTxManager handles transaction beginning
type MockTxManager struct {
	mock.Mock
}

func (m *MockTxManager) WithTx(ctx context.Context, fn func(postgres.Querier) error) error {
	// args contains the return values specified by .Return()
	args := m.Called(ctx, fn)
	return args.Error(0)
}

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
