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

func TestCreateProperty_Success(t *testing.T) {
	// Setup
	mockQuerier := new(MockQuerier)
	svc := NewPropertyService(mockQuerier, zap.NewNop())
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

	// Mock 2: Count Properties (Current = 2)
	mockQuerier.On("CountPropertiesByOwner", mock.Anything, pgtype.Int4{Int32: userID, Valid: true}).Return(int64(2), nil)

	// Mock 3: Create
	expectedProp := postgres.Property{
		ID:         1,
		OwnerID:    pgtype.Int4{Int32: userID, Valid: true},
		Address:    "123 Street",
		RentalType: postgres.PropertyTypeLongTerm,
	}
	mockQuerier.On("CreateProperty", mock.Anything, mock.MatchedBy(func(arg postgres.CreatePropertyParams) bool {
		return arg.Address == "123 Street" && arg.RentalType == postgres.PropertyTypeLongTerm
	})).Return(expectedProp, nil)

	// Execute
	prop, err := svc.CreateProperty(ctx, userID, "123 Street", "long_term", "{}")

	// Assert
	assert.NoError(t, err)
	assert.Equal(t, int32(1), prop.ID)
}

func TestCreateProperty_QuotaExceeded(t *testing.T) {
	// Setup
	core, observedLogs := observer.New(zap.WarnLevel)
	testLogger := zap.New(core)
	mockQuerier := new(MockQuerier)
	svc := NewPropertyService(mockQuerier, testLogger)
	ctx := context.Background()
	// Inject logger
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
	mockQuerier.On("CountPropertiesByOwner", mock.Anything, pgtype.Int4{Int32: userID, Valid: true}).Return(int64(1), nil)

	// Execute
	_, err := svc.CreateProperty(ctx, userID, "123 Street", "long_term", "{}")

	// Assert
	assert.Error(t, err)
	assert.Equal(t, "property quota exceeded for current plan", err.Error())

	// Verify Log
	logs := observedLogs.FilterMessage("create property failed: quota exceeded")
	assert.Equal(t, 1, logs.Len())
}

func TestCreateProperty_NoSubscription(t *testing.T) {
	// Setup
	mockQuerier := new(MockQuerier)
	svc := NewPropertyService(mockQuerier, zap.NewNop())
	ctx := context.Background()
	userID := int32(1)

	// Mock 1: No active subscription
	mockQuerier.On("GetUserSubscription", mock.Anything, pgtype.Int4{Int32: userID, Valid: true}).Return(postgres.Subscription{}, pgx.ErrNoRows)

	// Execute
	_, err := svc.CreateProperty(ctx, userID, "123 Street", "long_term", "{}")

	// Assert
	assert.Error(t, err)
	assert.Equal(t, "user has no active subscription", err.Error())
}

func TestListProperties(t *testing.T) {
	// Setup
	mockQuerier := new(MockQuerier)
	svc := NewPropertyService(mockQuerier, zap.NewNop())
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
