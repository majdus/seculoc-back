package service

import (
	"context"
	"fmt"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.uber.org/zap"

	"seculoc-back/internal/adapter/storage/postgres"
)

func TestDeleteProperty_Success(t *testing.T) {
	// Setup
	mockQuerier := new(MockQuerier)
	mockTx := new(MockTxManager)

	// Mock transaction execution
	mockTx.On("WithTx", mock.Anything, mock.Anything).Return(nil).Run(func(args mock.Arguments) {
		fn := args.Get(1).(func(postgres.Querier) error)
		_ = fn(mockQuerier)
	})

	svc := NewPropertyService(mockTx, zap.NewNop())
	ctx := context.Background()

	userID := int32(10)
	propertyID := int32(100)

	// Mock SoftDeleteProperty
	mockQuerier.On("SoftDeleteProperty", mock.Anything, mock.MatchedBy(func(arg postgres.SoftDeletePropertyParams) bool {
		return arg.ID == propertyID && arg.OwnerID.Int32 == userID
	})).Return(int32(100), nil)

	// Execute
	err := svc.DeleteProperty(ctx, userID, propertyID)

	// Assert
	assert.NoError(t, err)
}

func TestDeleteProperty_NotFound(t *testing.T) {
	// Setup
	mockQuerier := new(MockQuerier)
	mockTx := new(MockTxManager)

	mockTx.On("WithTx", mock.Anything, mock.Anything).Return(fmt.Errorf("property not found or access denied")).Run(func(args mock.Arguments) {
		fn := args.Get(1).(func(postgres.Querier) error)
		_ = fn(mockQuerier)
	})

	svc := NewPropertyService(mockTx, zap.NewNop())
	ctx := context.Background()

	userID := int32(10)
	propertyID := int32(999)

	// Mock SoftDeleteProperty -> ErrNoRows
	mockQuerier.On("SoftDeleteProperty", mock.Anything, mock.Anything).Return(int32(0), pgx.ErrNoRows)

	// Execute
	err := svc.DeleteProperty(ctx, userID, propertyID)

	// Assert
	assert.Error(t, err)
	assert.Equal(t, "property not found or access denied", err.Error())
}
