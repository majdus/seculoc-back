package service

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.uber.org/zap"

	"seculoc-back/internal/adapter/storage/postgres"
)

type mockEmailSender struct {
	mock.Mock
}

func (m *mockEmailSender) SendInvitation(ctx context.Context, toEmail, link string) error {
	args := m.Called(ctx, toEmail, link)
	return args.Error(0)
}

func TestCreateSolvencyCheck_PropertyCredits(t *testing.T) {
	mockTx := new(MockTxManager)
	mockQuerier := new(MockQuerier)
	mockEmail := new(mockEmailSender)
	svc := NewSolvencyService(mockTx, mockEmail, zap.NewNop())
	ctx := context.Background()
	userID := int32(1)
	propID := int32(10)
	email := "candidate@example.com"

	// Mock Tx
	mockTx.On("WithTx", mock.Anything, mock.Anything).Return(nil).Run(func(args mock.Arguments) {
		fn := args.Get(1).(func(postgres.Querier) error)

		// 1. Get Property For Update (Vacancy > 0)
		ownedProp := postgres.Property{
			ID:             propID,
			OwnerID:        pgtype.Int4{Int32: userID, Valid: true},
			VacancyCredits: 20,
		}
		mockQuerier.On("GetPropertyForUpdate", mock.Anything, propID).Return(ownedProp, nil)

		// 2. Decrease Property Credits
		mockQuerier.On("DecreasePropertyCredits", mock.Anything, propID).Return(nil)

		// 3. Find/Create Candidate
		mockQuerier.On("GetUserByEmail", mock.Anything, email).Return(postgres.User{}, pgx.ErrNoRows)
		mockQuerier.On("CreateUser", mock.Anything, mock.Anything).Return(postgres.User{ID: 200}, nil)

		// 4. Create Solvency Check
		expectedCheck := postgres.SolvencyCheck{
			ID:               100,
			InitiatorOwnerID: pgtype.Int4{Int32: userID, Valid: true},
			CandidateID:      pgtype.Int4{Int32: 200, Valid: true},
			Token:            pgtype.Text{String: "solv_candidate@example.com_10", Valid: true},
			Status:           postgres.NullSolvencyStatus{SolvencyStatus: postgres.SolvencyStatusPending, Valid: true},
		}
		mockQuerier.On("CreateSolvencyCheck", mock.Anything, mock.MatchedBy(func(arg postgres.CreateSolvencyCheckParams) bool {
			return arg.InitiatorOwnerID.Int32 == userID && arg.CandidateID.Int32 == 200 && arg.PropertyID.Int32 == propID
		})).Return(expectedCheck, nil)

		_ = fn(mockQuerier)
	})

	// 5. Send Email Invitation (OUTSIDE TX)
	mockEmail.On("SendInvitation", mock.Anything, email, mock.MatchedBy(func(link string) bool {
		return link == "https://seculoc.com/check/solv_candidate@example.com_10"
	})).Return(nil)

	check, err := svc.InitiateCheck(ctx, InitiateCheckParams{
		UserID:         userID,
		CandidateEmail: email,
		PropertyID:     propID,
	})
	assert.NoError(t, err)
	assert.Equal(t, int32(100), check.ID)
	mockEmail.AssertExpectations(t)
}

func TestCreateSolvencyCheck_GlobalCredits(t *testing.T) {
	mockTx := new(MockTxManager)
	mockQuerier := new(MockQuerier)
	mockEmail := new(mockEmailSender)
	svc := NewSolvencyService(mockTx, mockEmail, zap.NewNop())
	ctx := context.Background()
	userID := int32(1)
	propID := int32(10)
	email := "candidate@example.com"

	mockTx.On("WithTx", mock.Anything, mock.Anything).Return(nil).Run(func(args mock.Arguments) {
		fn := args.Get(1).(func(postgres.Querier) error)

		// 1. Get Property For Update (Vacancy = 0)
		ownedProp := postgres.Property{
			ID:             propID,
			OwnerID:        pgtype.Int4{Int32: userID, Valid: true},
			VacancyCredits: 0,
		}
		mockQuerier.On("GetPropertyForUpdate", mock.Anything, propID).Return(ownedProp, nil)

		// 2. Get Global Balance For Update (> 0)
		// Lock User First
		mockQuerier.On("GetUserForUpdate", mock.Anything, userID).Return(postgres.User{ID: userID}, nil)
		mockQuerier.On("GetUserCreditBalanceForUpdate", mock.Anything, pgtype.Int4{Int32: userID, Valid: true}).Return(int32(5), nil)

		// 3. Create Debit Transaction (-1)
		mockQuerier.On("CreateCreditTransaction", mock.Anything, mock.MatchedBy(func(arg postgres.CreateCreditTransactionParams) bool {
			return arg.UserID.Int32 == userID && arg.Amount == -1 && arg.TransactionType == "check_usage"
		})).Return(postgres.CreditTransaction{}, nil)

		// 4. Find/Create Candidate
		mockQuerier.On("GetUserByEmail", mock.Anything, email).Return(postgres.User{ID: 200}, nil)

		// 5. Create Solvency Check
		expectedCheck := postgres.SolvencyCheck{
			ID:    101,
			Token: pgtype.Text{String: "solv_candidate@example.com_10", Valid: true},
		}
		mockQuerier.On("CreateSolvencyCheck", mock.Anything, mock.Anything).Return(expectedCheck, nil)

		_ = fn(mockQuerier)
	})

	// 6. Send Email Invitation (OUTSIDE TX)
	mockEmail.On("SendInvitation", mock.Anything, email, mock.Anything).Return(nil)

	check, err := svc.InitiateCheck(ctx, InitiateCheckParams{
		UserID:         userID,
		CandidateEmail: email,
		PropertyID:     propID,
	})
	assert.NoError(t, err)
	assert.Equal(t, int32(101), check.ID)
	mockEmail.AssertExpectations(t)
}

func TestCreateSolvencyCheck_NotOwner(t *testing.T) {
	mockTx := new(MockTxManager)
	mockQuerier := new(MockQuerier)
	mockEmail := new(mockEmailSender)
	svc := NewSolvencyService(mockTx, mockEmail, zap.NewNop())
	ctx := context.Background()
	userID := int32(1)
	propID := int32(99)

	mockTx.On("WithTx", mock.Anything, mock.Anything).Return(errors.New("property not found or access denied")).Run(func(args mock.Arguments) {
		fn := args.Get(1).(func(postgres.Querier) error)

		// 1. Get Property For Update - Returns property owned by SOMEONE ELSE (ID 2)
		otherProp := postgres.Property{
			ID:      propID,
			OwnerID: pgtype.Int4{Int32: 2, Valid: true},
		}
		mockQuerier.On("GetPropertyForUpdate", mock.Anything, propID).Return(otherProp, nil)

		_ = fn(mockQuerier)
	})

	// Execute
	_, err := svc.InitiateCheck(ctx, InitiateCheckParams{
		UserID:         userID,
		CandidateEmail: "hacker@test.com",
		PropertyID:     propID,
	})

	// Assert
	assert.Error(t, err)
	assert.Equal(t, "property not found or access denied", err.Error())
}

func TestCreateSolvencyCheck_InsufficientCredits(t *testing.T) {
	mockTx := new(MockTxManager)
	mockQuerier := new(MockQuerier)
	mockEmail := new(mockEmailSender)
	svc := NewSolvencyService(mockTx, mockEmail, zap.NewNop())
	ctx := context.Background()
	userID := int32(1)

	// Mock Tx
	mockTx.On("WithTx", mock.Anything, mock.Anything).Return(&ErrInsufficientCredits{GlobalBalance: 0, PropertyBalance: 0}).Run(func(args mock.Arguments) {
		fn := args.Get(1).(func(postgres.Querier) error)

		// 1. Get Property For Update (Vacancy = 0)
		prop := postgres.Property{
			ID:             1,
			OwnerID:        pgtype.Int4{Int32: userID, Valid: true},
			VacancyCredits: 0,
		}
		mockQuerier.On("GetPropertyForUpdate", mock.Anything, int32(1)).Return(prop, nil)

		// 2. Get Global Balance For Update (0)
		// Lock User First
		mockQuerier.On("GetUserForUpdate", mock.Anything, userID).Return(postgres.User{ID: userID}, nil)
		mockQuerier.On("GetUserCreditBalanceForUpdate", mock.Anything, pgtype.Int4{Int32: userID, Valid: true}).Return(int32(0), nil)

		_ = fn(mockQuerier)
	})

	// Execute
	_, err := svc.InitiateCheck(ctx, InitiateCheckParams{
		UserID:         userID,
		CandidateEmail: "test@test.com",
		PropertyID:     1,
	})

	// Assert
	assert.Error(t, err)
	var insErr *ErrInsufficientCredits
	assert.True(t, errors.As(err, &insErr))
	assert.Equal(t, int32(0), insErr.GlobalBalance)
}

func TestPurchaseCredits_Success(t *testing.T) {
	mockTx := new(MockTxManager)
	mockQuerier := new(MockQuerier)
	mockEmail := new(mockEmailSender)
	svc := NewSolvencyService(mockTx, mockEmail, zap.NewNop())
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

func TestCancelCheck_Success_Property(t *testing.T) {
	mockTx := new(MockTxManager)
	mockQuerier := new(MockQuerier)
	svc := NewSolvencyService(mockTx, nil, zap.NewNop())
	ctx := context.Background()
	ownerID := int32(1)
	checkID := int32(100)
	propID := int32(10)

	mockTx.On("WithTx", mock.Anything, mock.Anything).Return(nil).Run(func(args mock.Arguments) {
		q := args.Get(1).(func(postgres.Querier) error)

		// 1. Get Check
		mockQuerier.On("GetSolvencyCheckByID", mock.Anything, checkID).Return(postgres.SolvencyCheck{
			ID:               checkID,
			InitiatorOwnerID: pgtype.Int4{Int32: ownerID, Valid: true},
			PropertyID:       pgtype.Int4{Int32: propID, Valid: true},
			Status:           postgres.NullSolvencyStatus{SolvencyStatus: postgres.SolvencyStatusPending, Valid: true},
			CreditSource:     pgtype.Text{String: "property", Valid: true},
		}, nil)

		// 2. Mark Cancelled
		mockQuerier.On("CancelSolvencyCheck", mock.Anything, checkID).Return(nil)

		// 3. Refund Property
		mockQuerier.On("IncreasePropertyCredits", mock.Anything, propID).Return(nil)

		_ = q(mockQuerier)
	})

	err := svc.CancelCheck(ctx, checkID, ownerID)
	assert.NoError(t, err)
	mockQuerier.AssertExpectations(t)
}

func TestCancelCheck_Success_Global(t *testing.T) {
	mockTx := new(MockTxManager)
	mockQuerier := new(MockQuerier)
	svc := NewSolvencyService(mockTx, nil, zap.NewNop())
	ctx := context.Background()
	ownerID := int32(1)
	checkID := int32(101)

	mockTx.On("WithTx", mock.Anything, mock.Anything).Return(nil).Run(func(args mock.Arguments) {
		q := args.Get(1).(func(postgres.Querier) error)

		// 1. Get Check
		mockQuerier.On("GetSolvencyCheckByID", mock.Anything, checkID).Return(postgres.SolvencyCheck{
			ID:               checkID,
			InitiatorOwnerID: pgtype.Int4{Int32: ownerID, Valid: true},
			Status:           postgres.NullSolvencyStatus{SolvencyStatus: postgres.SolvencyStatusPending, Valid: true},
			CreditSource:     pgtype.Text{String: "global", Valid: true},
		}, nil)

		// 2. Mark Cancelled
		mockQuerier.On("CancelSolvencyCheck", mock.Anything, checkID).Return(nil)

		// 3. Refund Global
		mockQuerier.On("CreateCreditTransaction", mock.Anything, mock.MatchedBy(func(arg postgres.CreateCreditTransactionParams) bool {
			return arg.UserID.Int32 == ownerID && arg.Amount == 1 && arg.TransactionType == "refund"
		})).Return(postgres.CreditTransaction{}, nil)

		_ = q(mockQuerier)
	})

	err := svc.CancelCheck(ctx, checkID, ownerID)
	assert.NoError(t, err)
	mockQuerier.AssertExpectations(t)
}

func TestCancelCheck_Unauthorized(t *testing.T) {
	mockTx := new(MockTxManager)
	mockQuerier := new(MockQuerier)
	svc := NewSolvencyService(mockTx, nil, zap.NewNop())
	ctx := context.Background()
	checkID := int32(100)

	mockTx.On("WithTx", mock.Anything, mock.Anything).Return(errors.New("unauthorized: you don't own this check")).Run(func(args mock.Arguments) {
		q := args.Get(1).(func(postgres.Querier) error)

		// Returns check owned by owner 2, but we try with owner 1
		mockQuerier.On("GetSolvencyCheckByID", mock.Anything, checkID).Return(postgres.SolvencyCheck{
			ID:               checkID,
			InitiatorOwnerID: pgtype.Int4{Int32: 2, Valid: true},
		}, nil)

		_ = q(mockQuerier)
	})

	err := svc.CancelCheck(ctx, checkID, 1)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unauthorized")
}

func TestCancelCheck_AlreadyProcessed(t *testing.T) {
	mockTx := new(MockTxManager)
	mockQuerier := new(MockQuerier)
	svc := NewSolvencyService(mockTx, nil, zap.NewNop())
	ctx := context.Background()
	ownerID := int32(1)
	checkID := int32(100)

	mockTx.On("WithTx", mock.Anything, mock.Anything).Return(errors.New("cannot cancel check with status: approved")).Run(func(args mock.Arguments) {
		q := args.Get(1).(func(postgres.Querier) error)

		mockQuerier.On("GetSolvencyCheckByID", mock.Anything, checkID).Return(postgres.SolvencyCheck{
			ID:               checkID,
			InitiatorOwnerID: pgtype.Int4{Int32: ownerID, Valid: true},
			Status:           postgres.NullSolvencyStatus{SolvencyStatus: postgres.SolvencyStatusApproved, Valid: true},
		}, nil)

		_ = q(mockQuerier)
	})

	err := svc.CancelCheck(ctx, checkID, ownerID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot cancel check with status")
}
