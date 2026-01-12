package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.uber.org/zap"

	"seculoc-back/internal/adapter/storage/postgres"
)

func TestSubscribeUser_Discovery_SetsLimitOne(t *testing.T) {
	// Setup
	mockTx := new(MockTxManager)
	mockQuerier := new(MockQuerier)
	svc := NewSubscriptionService(mockTx, zap.NewNop())
	ctx := context.Background()
	userID := int32(50)

	// Expect CreateSubscription with MaxPropertiesLimit = 1
	mockQuerier.On("CreateSubscription", mock.Anything, mock.MatchedBy(func(arg postgres.CreateSubscriptionParams) bool {
		return arg.UserID.Int32 == userID &&
			arg.PlanType == postgres.SubPlanDiscovery &&
			arg.MaxPropertiesLimit.Int32 == 1 // Verify the change
	})).Return(postgres.Subscription{ID: 1}, nil)

	// No credit transaction for discovery plan (amount 0)

	// WithTx
	mockTx.On("WithTx", mock.Anything, mock.Anything).Return(nil).Run(func(args mock.Arguments) {
		fn := args.Get(1).(func(postgres.Querier) error)
		_ = fn(mockQuerier)
	})

	// Execute
	err := svc.SubscribeUser(ctx, userID, "discovery", "monthly")

	// Assert
	assert.NoError(t, err)
}
