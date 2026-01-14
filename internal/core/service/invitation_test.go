package service

import (
	"context"
	"fmt"
	"math/big"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.uber.org/zap"

	"seculoc-back/internal/adapter/storage/postgres"
	"seculoc-back/internal/platform/email"
)

func TestInviteTenant_Success(t *testing.T) {
	// Setup
	mockTx := new(MockTxManager)
	mockQuerier := new(MockQuerier)
	emailSender := email.NewMockEmailSender(zap.NewNop())
	svc := NewUserService(mockTx, zap.NewNop(), emailSender, "http://test.com")

	mockTx.On("WithTx", mock.Anything, mock.Anything).Return(nil).Run(func(args mock.Arguments) {
		fn := args.Get(1).(func(postgres.Querier) error)
		_ = fn(mockQuerier)
	})

	// Mocks
	ownerID := int32(10)
	propID := int32(100)
	email := "tenant@example.com"

	// Mock Property Check
	mockQuerier.On("GetProperty", mock.Anything, propID).Return(postgres.Property{
		ID:      propID,
		OwnerID: pgtype.Int4{Int32: ownerID, Valid: true},
	}, nil)

	// Mock CreateInvitation
	mockQuerier.On("CreateInvitation", mock.Anything, mock.MatchedBy(func(arg postgres.CreateInvitationParams) bool {
		return arg.PropertyID == propID && arg.OwnerID == ownerID && arg.TenantEmail == email && arg.Token != ""
	})).Return(postgres.LeaseInvitation{
		ID:          1,
		TenantEmail: email,
		Token:       "secure-token",
		Status:      pgtype.Text{String: "pending", Valid: true},
	}, nil)

	// Execute
	inv, err := svc.InviteTenant(context.Background(), ownerID, propID, email)

	// Assert
	assert.NoError(t, err)
	assert.Equal(t, "secure-token", inv.Token)
	mockQuerier.AssertExpectations(t)
}

func TestAcceptInvitation_Success(t *testing.T) {
	// Setup
	mockQuerier := new(MockQuerier)
	mockTx := new(MockTxManager)
	emailSender := email.NewMockEmailSender(zap.NewNop())
	svc := NewUserService(mockTx, zap.NewNop(), emailSender, "http://test.com")

	mockTx.On("WithTx", mock.Anything, mock.Anything).Return(nil).Run(func(args mock.Arguments) {
		fn := args.Get(1).(func(postgres.Querier) error)
		_ = fn(mockQuerier)
	})

	token := "valid-token"
	userID := int32(55)

	// Mock GetInvitation
	mockQuerier.On("GetInvitationByToken", mock.Anything, token).Return(postgres.LeaseInvitation{
		ID:         1,
		PropertyID: 100,
		Status:     pgtype.Text{String: "pending", Valid: true},
		ExpiresAt:  pgtype.Timestamp{Time: time.Now().Add(24 * time.Hour), Valid: true},
	}, nil)

	// Mock GetProperty
	mockQuerier.On("GetProperty", mock.Anything, int32(100)).Return(postgres.Property{
		ID:            100,
		RentAmount:    pgtype.Numeric{Int: big.NewInt(1000), Valid: true},
		DepositAmount: pgtype.Numeric{Int: big.NewInt(2000), Valid: true},
	}, nil)

	// Mock CreateLease
	mockQuerier.On("CreateLease", mock.Anything, mock.Anything).Return(postgres.Lease{ID: 1}, nil)

	// Mock Update Status
	mockQuerier.On("UpdateInvitationStatus", mock.Anything, postgres.UpdateInvitationStatusParams{
		ID:     1,
		Status: pgtype.Text{String: "accepted", Valid: true},
	}).Return(nil)

	// Execute
	err := svc.AcceptInvitation(context.Background(), token, userID)

	// Assert
	assert.NoError(t, err)
	mockQuerier.AssertExpectations(t)
}

func TestAcceptInvitation_Expired(t *testing.T) {
	// Setup
	mockQuerier := new(MockQuerier)
	mockTx := new(MockTxManager)
	emailSender := email.NewMockEmailSender(zap.NewNop())
	svc := NewUserService(mockTx, zap.NewNop(), emailSender, "http://test.com")

	mockTx.On("WithTx", mock.Anything, mock.Anything).Return(fmt.Errorf("invitation expired")).Run(func(args mock.Arguments) {
		fn := args.Get(1).(func(postgres.Querier) error)
		_ = fn(mockQuerier)
	})

	token := "expired-token"

	// Mock GetInvitation
	mockQuerier.On("GetInvitationByToken", mock.Anything, token).Return(postgres.LeaseInvitation{
		ID:        1,
		Status:    pgtype.Text{String: "pending", Valid: true},
		ExpiresAt: pgtype.Timestamp{Time: time.Now().Add(-24 * time.Hour), Valid: true}, // Expired
	}, nil)

	// Execute
	err := svc.AcceptInvitation(context.Background(), token, 55)

	// Assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "expired")
}
