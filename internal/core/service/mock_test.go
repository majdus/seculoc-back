package service

import (
	"context"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/mock"

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

func (m *MockQuerier) CreateUser(ctx context.Context, arg postgres.CreateUserParams) (postgres.User, error) {
	args := m.Called(ctx, arg)
	return args.Get(0).(postgres.User), args.Error(1)
}

func (m *MockQuerier) GetUserByEmail(ctx context.Context, email string) (postgres.User, error) {
	args := m.Called(ctx, email)
	return args.Get(0).(postgres.User), args.Error(1)
}

func (m *MockQuerier) GetUserById(ctx context.Context, id int32) (postgres.User, error) {
	args := m.Called(ctx, id)
	return args.Get(0).(postgres.User), args.Error(1)
}

// Property Domain Methods
func (m *MockQuerier) CreateProperty(ctx context.Context, arg postgres.CreatePropertyParams) (postgres.Property, error) {
	args := m.Called(ctx, arg)
	return args.Get(0).(postgres.Property), args.Error(1)
}

func (m *MockQuerier) ListPropertiesByOwner(ctx context.Context, ownerID pgtype.Int4) ([]postgres.Property, error) {
	args := m.Called(ctx, ownerID)
	return args.Get(0).([]postgres.Property), args.Error(1)
}

func (m *MockQuerier) GetProperty(ctx context.Context, id int32) (postgres.Property, error) {
	args := m.Called(ctx, id)
	return args.Get(0).(postgres.Property), args.Error(1)
}

func (m *MockQuerier) HasReceivedInitialBonus(ctx context.Context, userID pgtype.Int4) (bool, error) {
	args := m.Called(ctx, userID)
	return args.Get(0).(bool), args.Error(1)
}

func (m *MockQuerier) CountPropertiesByOwner(ctx context.Context, ownerID pgtype.Int4) (int64, error) {
	args := m.Called(ctx, ownerID)
	return args.Get(0).(int64), args.Error(1)
}

func (m *MockQuerier) CountPropertiesByOwnerAndType(ctx context.Context, arg postgres.CountPropertiesByOwnerAndTypeParams) (int64, error) {
	args := m.Called(ctx, arg)
	return args.Get(0).(int64), args.Error(1)
}

func (m *MockQuerier) UpdateSubscriptionLimit(ctx context.Context, arg postgres.UpdateSubscriptionLimitParams) error {
	args := m.Called(ctx, arg)
	return args.Error(0)
}

func (m *MockQuerier) CreateSolvencyCheck(ctx context.Context, arg postgres.CreateSolvencyCheckParams) (postgres.SolvencyCheck, error) {
	args := m.Called(ctx, arg)
	return args.Get(0).(postgres.SolvencyCheck), args.Error(1)
}

func (m *MockQuerier) SoftDeleteProperty(ctx context.Context, arg postgres.SoftDeletePropertyParams) (int32, error) {
	args := m.Called(ctx, arg)
	return args.Get(0).(int32), args.Error(1)
}

// MockTxManager handles transaction beginning
type MockTxManager struct {
	mock.Mock
}

func (m *MockTxManager) WithTx(ctx context.Context, fn func(postgres.Querier) error) error {
	args := m.Called(ctx, fn)
	return args.Error(0)
}
