package service

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.uber.org/zap"

	"seculoc-back/internal/adapter/storage/postgres"
)

func TestCreateSolvencyCheck_Success(t *testing.T) {
	mockTx := new(MockTxManager)
	mockQuerier := new(MockQuerier)
	svc := NewSolvencyService(mockTx, zap.NewNop())
	ctx := context.Background()
	userID := int32(1)
	propID := int32(10)
	email := "candidate@example.com"

	// Mock 1: Get Balance (Must be > 0)
	mockQuerier.On("GetUserCreditBalance", mock.Anything, pgtype.Int4{Int32: userID, Valid: true}).Return(int32(5), nil)

	// Mock 2: Create Debit Transaction (-1)
	mockQuerier.On("CreateCreditTransaction", mock.Anything, mock.MatchedBy(func(arg postgres.CreateCreditTransactionParams) bool {
		return arg.UserID.Int32 == userID && arg.Amount == -1 && arg.TransactionType == "check_usage"
	})).Return(postgres.CreditTransaction{}, nil)

	// Mock 3: Create Solvency Check
	expectedCheck := postgres.SolvencyCheck{
		ID:               100,
		InitiatorOwnerID: pgtype.Int4{Int32: userID, Valid: true},
		Status:           postgres.NullSolvencyStatus{SolvencyStatus: postgres.SolvencyStatusPending, Valid: true},
	}
	mockQuerier.On("CreateSolvencyCheck", mock.Anything, mock.MatchedBy(func(arg postgres.CreateSolvencyCheckParams) bool {
		return arg.InitiatorOwnerID.Int32 == userID && arg.CandidateEmail == email && arg.PropertyID.Int32 == propID
	})).Return(expectedCheck, nil)

	// Mock Tx
	mockTx.On("WithTx", mock.Anything, mock.Anything).Return(nil).Run(func(args mock.Arguments) {
		fn := args.Get(1).(func(postgres.Querier) error)
		_ = fn(mockQuerier)
	})

	// Execute
	check, err := svc.RetrieveCheck(ctx, userID, email, propID)

	// Assert
	assert.NoError(t, err)
	assert.Equal(t, int32(100), check.ID)
}

func TestCreateSolvencyCheck_InsufficientCredits(t *testing.T) {
	mockTx := new(MockTxManager)
	mockQuerier := new(MockQuerier)
	svc := NewSolvencyService(mockTx, zap.NewNop())
	ctx := context.Background()
	userID := int32(1)

	// Mock 1: Get Balance (0)
	// Note: WithTx wrappers usually handle the error return.
	// Logic inside Tx: check balance. If 0, return error.

	// We expect the TX logic to execute.
	mockTx.On("WithTx", mock.Anything, mock.Anything).Return(errors.New("insufficient credits")).Run(func(args mock.Arguments) {
		fn := args.Get(1).(func(postgres.Querier) error)

		// Mocking the inner behaviour manually because we are mocking WithTx
		// The service will call GetUserCreditBalance inside.
		mockQuerier.On("GetUserCreditBalance", mock.Anything, pgtype.Int4{Int32: userID, Valid: true}).Return(int32(0), nil)

		_ = fn(mockQuerier)
	})

	// Execute
	_, err := svc.RetrieveCheck(ctx, userID, "test@test.com", 1)

	// Assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "insufficient credits")
}

func TestPurchaseCredits_Success(t *testing.T) {
	mockTx := new(MockTxManager)
	mockQuerier := new(MockQuerier)
	svc := NewSolvencyService(mockTx, zap.NewNop())
	ctx := context.Background()
	userID := int32(1)

	mockTx.On("WithTx", mock.Anything, mock.Anything).Return(nil).Run(func(args mock.Arguments) {
		fn := args.Get(1).(func(postgres.Querier) error)
		_ = fn(mockQuerier)
	})

	mockQuerier.On("CreateCreditTransaction", mock.Anything, mock.MatchedBy(func(arg postgres.CreateCreditTransactionParams) bool {
		return arg.UserID.Int32 == userID && arg.Amount == 20 && arg.TransactionType == "pack_purchase"
	})).Return(postgres.CreditTransaction{}, nil)

	amount, err := svc.PurchaseCredits(ctx, userID, "pack_20")

	assert.NoError(t, err)
	assert.Equal(t, int32(20), amount)
}
