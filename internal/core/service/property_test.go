package service

import (
	"context"
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

func TestCreateProperty_Success(t *testing.T) {
	// Setup
	mockQuerier := new(MockQuerier)
	mockTx := new(MockTxManager)
	mockTx.On("WithTx", mock.Anything, mock.Anything).Return(nil).Run(func(args mock.Arguments) {
		fn := args.Get(1).(func(postgres.Querier) error)
		_ = fn(mockQuerier)
	})

	svc := NewPropertyService(mockTx, zap.NewNop())
	ctx := context.Background()

	userID := int32(1)

	// Mock 1: Get Subscription (Limit = 5)
	activeSub := postgres.Subscription{
		ID:                 1,
		UserID:             pgtype.Int4{Int32: userID, Valid: true},
		Status:             pgtype.Text{String: "active", Valid: true},
		MaxPropertiesLimit: pgtype.Int4{Int32: 5, Valid: true},
	}
	mockQuerier.On("GetUserSubscription", mock.Anything, pgtype.Int4{Int32: userID, Valid: true}).Return(activeSub, nil)

	// Mock 2: Count Properties (Only called for LongTerm)
	mockQuerier.On("CountPropertiesByOwnerAndType", mock.Anything, mock.MatchedBy(func(arg postgres.CountPropertiesByOwnerAndTypeParams) bool {
		return arg.OwnerID.Int32 == userID && arg.RentalType == postgres.PropertyTypeLongTerm
	})).Return(int64(2), nil) // 2 < 5 OK

	// Mock 3: Create
	expectedProp := postgres.Property{
		ID:         1,
		OwnerID:    pgtype.Int4{Int32: userID, Valid: true},
		Address:    "123 Street",
		RentalType: postgres.PropertyTypeLongTerm,
	}
	mockQuerier.On("CreateProperty", mock.Anything, mock.MatchedBy(func(arg postgres.CreatePropertyParams) bool {
		// Note from execution: Address was "123 Main St" vs "123 Street" in mock.
		// Amounts are 1000 and 2000.
		// arg.RentAmount is pgtype.Numeric.
		// Simplest check:
		r, _ := arg.RentAmount.Float64Value()
		d, _ := arg.DepositAmount.Float64Value()
		return arg.Address == "123 Main St" &&
			arg.RentalType == postgres.PropertyTypeLongTerm &&
			r.Float64 == 1000.0 &&
			d.Float64 == 2000.0
	})).Return(expectedProp, nil)

	// Execute
	prop, err := svc.CreateProperty(ctx, 1, "123 Main St", "long_term", "{}", 1000, 2000)

	// Assert
	assert.NoError(t, err)
	assert.Equal(t, int32(1), prop.ID)
}

func TestCreateProperty_Seasonal_AlwaysAllowed(t *testing.T) {
	// Setup
	mockQuerier := new(MockQuerier)
	mockTx := new(MockTxManager)
	mockTx.On("WithTx", mock.Anything, mock.Anything).Return(nil).Run(func(args mock.Arguments) {
		fn := args.Get(1).(func(postgres.Querier) error)
		_ = fn(mockQuerier)
	})

	svc := NewPropertyService(mockTx, zap.NewNop())
	ctx := context.Background()
	userID := int32(1)

	// Mock 1: Get Subscription (Limit = 0, e.g. Free Plan)
	activeSub := postgres.Subscription{
		ID:                 1,
		UserID:             pgtype.Int4{Int32: userID, Valid: true},
		Status:             pgtype.Text{String: "active", Valid: true},
		MaxPropertiesLimit: pgtype.Int4{Int32: 0, Valid: true},
	}
	mockQuerier.On("GetUserSubscription", mock.Anything, pgtype.Int4{Int32: userID, Valid: true}).Return(activeSub, nil)

	// No CountPropertiesByOwnerAndType call expected!

	// Mock 2: Create
	expectedProp := postgres.Property{
		ID:         1,
		RentalType: postgres.PropertyTypeSeasonal,
	}
	mockQuerier.On("CreateProperty", mock.Anything, mock.Anything).Return(expectedProp, nil)

	// Execute
	prop, err := svc.CreateProperty(ctx, userID, "Holiday Home", "seasonal", "{}", 0, 0)

	// Assert
	assert.NoError(t, err)
	assert.Equal(t, postgres.PropertyTypeSeasonal, prop.RentalType)
}

func TestCreateProperty_QuotaExceeded_LongTerm(t *testing.T) {
	// Setup
	core, observedLogs := observer.New(zap.WarnLevel)
	testLogger := zap.New(core)
	mockQuerier := new(MockQuerier)
	mockTx := new(MockTxManager)

	mockTx.On("WithTx", mock.Anything, mock.Anything).Return(fmt.Errorf("property quota exceeded for current plan")).Run(func(args mock.Arguments) {
		fn := args.Get(1).(func(postgres.Querier) error)
		_ = fn(mockQuerier)
	})

	svc := NewPropertyService(mockTx, testLogger)
	ctx := context.Background()
	ctx = context.WithValue(ctx, "logger", testLogger)

	userID := int32(1)

	// Mock 1: Get Subscription (Limit = 1)
	activeSub := postgres.Subscription{
		ID:                 1,
		UserID:             pgtype.Int4{Int32: userID, Valid: true},
		Status:             pgtype.Text{String: "active", Valid: true},
		MaxPropertiesLimit: pgtype.Int4{Int32: 1, Valid: true},
	}
	mockQuerier.On("GetUserSubscription", mock.Anything, pgtype.Int4{Int32: userID, Valid: true}).Return(activeSub, nil)

	// Mock 2: Count Properties (Current = 1) -> Reached Limit!
	mockQuerier.On("CountPropertiesByOwnerAndType", mock.Anything, mock.MatchedBy(func(arg postgres.CountPropertiesByOwnerAndTypeParams) bool {
		return arg.OwnerID.Int32 == userID && arg.RentalType == postgres.PropertyTypeLongTerm
	})).Return(int64(1), nil)

	// Execute
	_, err := svc.CreateProperty(ctx, userID, "123 Street", "long_term", "{}", 0, 0)

	// Assert
	assert.Error(t, err)
	assert.Equal(t, "property quota exceeded for current plan", err.Error())

	// Verify Log -- The service logs "quota exceeded" before returning the error.
	// Since WithTx bubbled the error, did the log happen?
	// Yes, inside the function passed to WithTx.
	logs := observedLogs.FilterMessage("quota exceeded")
	assert.Equal(t, 1, logs.Len())
}

func TestCreateProperty_NoSubscription(t *testing.T) {
	// Setup
	mockQuerier := new(MockQuerier)
	mockTx := new(MockTxManager)

	mockTx.On("WithTx", mock.Anything, mock.Anything).Return(fmt.Errorf("user has no active subscription")).Run(func(args mock.Arguments) {
		fn := args.Get(1).(func(postgres.Querier) error)
		_ = fn(mockQuerier)
	})

	svc := NewPropertyService(mockTx, zap.NewNop())
	ctx := context.Background()
	userID := int32(1)

	// Mock 1: No active subscription
	mockQuerier.On("GetUserSubscription", mock.Anything, pgtype.Int4{Int32: userID, Valid: true}).Return(postgres.Subscription{}, pgx.ErrNoRows)

	// Execute
	_, err := svc.CreateProperty(ctx, userID, "123 Street", "long_term", "{}", 0, 0)

	// Assert
	assert.Error(t, err)
	assert.Equal(t, "user has no active subscription", err.Error())
}

func TestListProperties(t *testing.T) {
	// Setup
	mockQuerier := new(MockQuerier)
	mockTx := new(MockTxManager)

	// We mock success but WithTx returns the error from callback.
	mockTx.On("WithTx", mock.Anything, mock.Anything).Return(nil).Run(func(args mock.Arguments) {
		fn := args.Get(1).(func(postgres.Querier) error)
		_ = fn(mockQuerier)
	})

	svc := NewPropertyService(mockTx, zap.NewNop())
	ctx := context.Background()
	userID := int32(1)

	expectedList := []postgres.Property{
		{ID: 1, Address: "A"},
		{ID: 2, Address: "B"},
	}
	mockQuerier.On("ListPropertiesByOwner", mock.Anything, pgtype.Int4{Int32: userID, Valid: true}).Return(expectedList, nil)

	list, err := svc.ListProperties(ctx, userID)
	assert.NoError(t, err)
	assert.Len(t, list, 2)
}
