package service

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"go.uber.org/zap"

	"seculoc-back/internal/adapter/storage/postgres"
	"seculoc-back/internal/platform/logger"
)

// TxManager defines the interface for executing transactions.
type TxManager interface {
	WithTx(ctx context.Context, fn func(postgres.Querier) error) error
}

// SubscriptionService handles subscription logic.
type SubscriptionService struct {
	txManager TxManager
	// Optional: we could keep a global logger if needed, but we prefer context logger
}

// NewSubscriptionService creates a new SubscriptionService.
func NewSubscriptionService(txManager TxManager, l *zap.Logger) *SubscriptionService {
	return &SubscriptionService{
		txManager: txManager,
	}
}

// SubscribeUser subscribes a user to a plan.
func (s *SubscriptionService) SubscribeUser(ctx context.Context, userID int32, plan string, freq string) error {
	log := logger.FromContext(ctx)

	// Determine plan details (mock logic for demo)
	var planType postgres.SubPlan
	var maxProps int32
	var amount int32

	switch plan {
	case "premium":
		planType = postgres.SubPlanPremium
		maxProps = 5
		amount = 2990 // 29.90
	case "serenity":
		planType = postgres.SubPlanSerenity
		maxProps = 1
		amount = 990
	default:
		planType = postgres.SubPlanDiscovery
		maxProps = 0
		amount = 0
	}

	var frequency postgres.BillingFreq
	if freq == "yearly" {
		frequency = postgres.BillingFreqYearly
	} else {
		frequency = postgres.BillingFreqMonthly
	}

	err := s.txManager.WithTx(ctx, func(q postgres.Querier) error {
		// 1. Create Subscription

		_, err := q.CreateSubscription(ctx, postgres.CreateSubscriptionParams{
			UserID:    pgtype.Int4{Int32: userID, Valid: true},
			PlanType:  planType,
			Frequency: postgres.NullBillingFreq{BillingFreq: frequency, Valid: true},
			StartDate: pgtype.Date{Time: time.Now(), Valid: true},
			// EndDate can be null
			MaxPropertiesLimit: pgtype.Int4{Int32: maxProps, Valid: true},
		})
		if err != nil {
			return fmt.Errorf("failed to create subscription: %w", err)
		}

		// 2. Create Initial Credit Transaction (e.g., if plan gives credits)
		// Ignoring logic for free credits for now, just creating a transaction record
		if amount > 0 {
			_, err = q.CreateCreditTransaction(ctx, postgres.CreateCreditTransactionParams{
				UserID:          pgtype.Int4{Int32: userID, Valid: true},
				Amount:          amount,
				TransactionType: "plan_purchase",
				Description:     pgtype.Text{String: fmt.Sprintf("Purchase of %s plan", plan), Valid: true},
			})
			if err != nil {
				return fmt.Errorf("failed to create transaction: %w", err)
			}
		}

		log.Info("subscription created",
			zap.Int("user_id", int(userID)),
			zap.String("plan", plan),
			zap.Int("amount", int(amount)),
		)
		return nil
	})

	if err != nil {
		log.Error("subscription transaction failed",
			zap.Int("user_id", int(userID)),
			zap.String("plan", plan),
			zap.Error(err),
		)
		return err
	}

	return nil
}
