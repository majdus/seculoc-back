package service

import (
	"context"
	"errors"
	"math/big"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.uber.org/zap"

	"seculoc-back/internal/adapter/storage/postgres"
)

func TestListLeases_Success(t *testing.T) {
	// Setup
	mockQuerier := new(MockQuerier)
	mockTx := new(MockTxManager)

	mockTx.On("WithTx", mock.Anything, mock.Anything).Return(nil).Run(func(args mock.Arguments) {
		fn := args.Get(1).(func(postgres.Querier) error)
		_ = fn(mockQuerier)
	})

	svc := NewLeaseService(mockTx, zap.NewNop())

	tenantID := int32(5)

	// Mock Data
	now := time.Now()
	leasesDB := []postgres.ListLeasesByTenantRow{
		{
			ID:              1,
			PropertyID:      pgtype.Int4{Int32: 10, Valid: true},
			TenantID:        pgtype.Int4{Int32: tenantID, Valid: true},
			StartDate:       pgtype.Date{Time: now, Valid: true},
			RentAmount:      pgtype.Numeric{Int: FactoryBigInt(500), Exp: 0, Valid: true}, // Simplification for mock, assumes helper or direct setup
			DepositAmount:   pgtype.Numeric{Int: FactoryBigInt(1000), Exp: 0, Valid: true},
			LeaseStatus:     pgtype.Text{String: "active", Valid: true},
			ContractUrl:     pgtype.Text{String: "http://doc.pdf", Valid: true},
			PropertyAddress: "123 Main St",
			RentalType:      "long_term",
		},
	}

	mockQuerier.On("ListLeasesByTenant", mock.Anything, pgtype.Int4{Int32: tenantID, Valid: true}).Return(leasesDB, nil)

	// Execute
	leases, err := svc.ListLeases(context.Background(), tenantID)

	// Assert
	assert.NoError(t, err)
	assert.Len(t, leases, 1)
	assert.Equal(t, int32(1), leases[0].ID)
	assert.Equal(t, "123 Main St", leases[0].PropertyAddress)
	assert.Equal(t, "active", leases[0].Status)
}

func TestListLeases_Empty(t *testing.T) {
	mockQuerier := new(MockQuerier)
	mockTx := new(MockTxManager)

	mockTx.On("WithTx", mock.Anything, mock.Anything).Return(nil).Run(func(args mock.Arguments) {
		fn := args.Get(1).(func(postgres.Querier) error)
		_ = fn(mockQuerier)
	})

	svc := NewLeaseService(mockTx, zap.NewNop())
	tenantID := int32(5)

	mockQuerier.On("ListLeasesByTenant", mock.Anything, pgtype.Int4{Int32: tenantID, Valid: true}).Return([]postgres.ListLeasesByTenantRow{}, nil)

	leases, err := svc.ListLeases(context.Background(), tenantID)

	assert.NoError(t, err)
	assert.Len(t, leases, 0)
	assert.NotNil(t, leases) // Should be empty slice, not nil
}

func TestListLeases_DBError(t *testing.T) {
	mockQuerier := new(MockQuerier)
	mockTx := new(MockTxManager)

	mockTx.On("WithTx", mock.Anything, mock.Anything).Return(errors.New("db error")).Run(func(args mock.Arguments) {
		fn := args.Get(1).(func(postgres.Querier) error)
		_ = fn(mockQuerier)
	})

	mockFileStore := new(MockFileStorage)
	svc := NewLeaseService(mockTx, zap.NewNop(), mockFileStore)
	tenantID := int32(5)

	mockQuerier.On("ListLeasesByTenant", mock.Anything, mock.Anything).Return(nil, errors.New("db error"))

	leases, err := svc.ListLeases(context.Background(), tenantID)

	assert.Error(t, err)
	assert.Nil(t, leases)
}

func FactoryBigInt(v int64) *big.Int {
	return big.NewInt(v)
}

type MockFileStorage struct {
	mock.Mock
}

func (m *MockFileStorage) Save(filename string, content []byte) (string, error) {
	args := m.Called(filename, content)
	return args.String(0), args.Error(1)
}

func (m *MockFileStorage) Get(filename string) ([]byte, error) {
	args := m.Called(filename)
	return args.Get(0).([]byte), args.Error(1)
}

func (m *MockFileStorage) Exists(filename string) bool {
	args := m.Called(filename)
	return args.Bool(0)
}
