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
		maxProps = 1
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

// IncreaseLimit allows Serenity and Premium users to add property slots for 9.90/unit.
func (s *SubscriptionService) IncreaseLimit(ctx context.Context, userID int32, additionalProperties int32) error {
	log := logger.FromContext(ctx)

	return s.txManager.WithTx(ctx, func(q postgres.Querier) error {
		// 1. Get Subscription
		sub, err := q.GetUserSubscription(ctx, pgtype.Int4{Int32: userID, Valid: true})
		if err != nil {
			return err
		}

		// 2. Eligibility Check
		if sub.PlanType != postgres.SubPlanSerenity && sub.PlanType != postgres.SubPlanPremium {
			return fmt.Errorf("plan not eligible for limit increase (current: %s)", sub.PlanType)
		}

		// 3. Cost Calculation
		// 9.90 EUR per property.
		cost := 9.90 * float64(additionalProperties)

		// 4. Update Limit
		err = q.UpdateSubscriptionLimit(ctx, postgres.UpdateSubscriptionLimitParams{
			UserID:             pgtype.Int4{Int32: userID, Valid: true},
			MaxPropertiesLimit: pgtype.Int4{Int32: additionalProperties, Valid: true},
		})
		if err != nil {
			return err
		}

		// 5. Log Billing Event (Future: Insert into Transactions/Invoice)
		log.Info("subscription limit increased",
			zap.Int("user_id", int(userID)),
			zap.Int("added_slots", int(additionalProperties)),
			zap.Float64("additional_monthly_cost", cost),
		)

		return nil
	})
}
