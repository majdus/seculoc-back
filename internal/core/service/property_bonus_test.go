package service

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.uber.org/zap"

	"seculoc-back/internal/adapter/storage/postgres"
)

func TestCreateProperty_Discovery_GrantsBonus(t *testing.T) {
	// Setup
	mockTx := new(MockTxManager)
	mockQuerier := new(MockQuerier)
	svc := NewPropertyService(mockTx, zap.NewNop())
	ctx := context.Background()
	userID := int32(1)

	// Mock 1: Get Subscription (Discovery)
	activeSub := postgres.Subscription{
		ID:                 1,
		UserID:             pgtype.Int4{Int32: userID, Valid: true},
		Status:             pgtype.Text{String: "active", Valid: true},
		PlanType:           postgres.SubPlanDiscovery,
		MaxPropertiesLimit: pgtype.Int4{Int32: 1, Valid: true},
	}
	mockQuerier.On("GetUserSubscription", mock.Anything, pgtype.Int4{Int32: userID, Valid: true}).Return(activeSub, nil)

	// Mock 2: Count Properties (Count = 0)
	mockQuerier.On("CountPropertiesByOwnerAndType", mock.Anything, mock.MatchedBy(func(arg postgres.CountPropertiesByOwnerAndTypeParams) bool {
		return arg.OwnerID.Int32 == userID && arg.RentalType == postgres.PropertyTypeLongTerm
	})).Return(int64(0), nil)

	// Mock 3: Create Property
	expectedProp := postgres.Property{ID: 1, RentalType: postgres.PropertyTypeLongTerm}
	mockQuerier.On("CreateProperty", mock.Anything, mock.Anything).Return(expectedProp, nil)

	// Mock 4: HasReceivedInitialBonus (False)
	mockQuerier.On("HasReceivedInitialBonus", mock.Anything, pgtype.Int4{Int32: userID, Valid: true}).Return(false, nil)

	// Mock 5: CreateCreditTransaction (+3)
	mockQuerier.On("CreateCreditTransaction", mock.Anything, mock.MatchedBy(func(arg postgres.CreateCreditTransactionParams) bool {
		return arg.UserID.Int32 == userID &&
			arg.Amount == 3 &&
			arg.TransactionType == "initial_free"
	})).Return(postgres.CreditTransaction{}, nil)

	// WithTx
	mockTx.On("WithTx", mock.Anything, mock.Anything).Return(nil).Run(func(args mock.Arguments) {
		fn := args.Get(1).(func(postgres.Querier) error)
		_ = fn(mockQuerier)
	})

	// Execute
	_, err := svc.CreateProperty(ctx, userID, "", "123 Main St", "seasonal", "{}", 1000, 2000)

	// Assert
	assert.NoError(t, err)
}

func TestCreateProperty_Discovery_BonusAlreadyReceived(t *testing.T) {
	// Setup
	mockTx := new(MockTxManager)
	mockQuerier := new(MockQuerier)
	svc := NewPropertyService(mockTx, zap.NewNop())
	ctx := context.Background()
	userID := int32(1)

	// Mock 1: Get Subscription (Discovery)
	activeSub := postgres.Subscription{
		ID:                 1,
		UserID:             pgtype.Int4{Int32: userID, Valid: true},
		PlanType:           postgres.SubPlanDiscovery,
		MaxPropertiesLimit: pgtype.Int4{Int32: 1, Valid: true},
	}
	mockQuerier.On("GetUserSubscription", mock.Anything, pgtype.Int4{Int32: userID, Valid: true}).Return(activeSub, nil)

	// Mock 2: Count Properties (Count = 0) - Maybe they deleted previous one
	mockQuerier.On("CountPropertiesByOwnerAndType", mock.Anything, mock.Anything).Return(int64(0), nil)

	// Mock 3: Create Property
	mockQuerier.On("CreateProperty", mock.Anything, mock.Anything).Return(postgres.Property{ID: 2, RentalType: postgres.PropertyTypeLongTerm}, nil)

	// Mock 4: HasReceivedInitialBonus (True)
	mockQuerier.On("HasReceivedInitialBonus", mock.Anything, pgtype.Int4{Int32: userID, Valid: true}).Return(true, nil)

	// Expect NO Credit Transaction

	// WithTx
	mockTx.On("WithTx", mock.Anything, mock.Anything).Return(nil).Run(func(args mock.Arguments) {
		fn := args.Get(1).(func(postgres.Querier) error)
		_ = fn(mockQuerier)
	})

	// Execute
	_, err := svc.CreateProperty(ctx, userID, "", "Bonus St 2", "long_term", "{}", 0, 0)

	// Assert
	assert.NoError(t, err)
}
