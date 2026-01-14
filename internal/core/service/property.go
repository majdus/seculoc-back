package service

import (
	"context"
	"fmt"

	"go.uber.org/zap"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"seculoc-back/internal/adapter/storage/postgres"
	"seculoc-back/internal/platform/logger"
)

type PropertyService struct {
	txManager TxManager
	log       *zap.Logger
}

func NewPropertyService(txManager TxManager, l *zap.Logger) *PropertyService {
	return &PropertyService{
		txManager: txManager,
		log:       l,
	}
}

func (s *PropertyService) CreateProperty(ctx context.Context, userID int32, address string, rentalType string, detailsJSON string, rentAmount, depositAmount float64) (*postgres.Property, error) {
	log := logger.FromContext(ctx)
	var prop postgres.Property

	err := s.txManager.WithTx(ctx, func(q postgres.Querier) error {
		// 1. Get User Subscription
		sub, err := q.GetUserSubscription(ctx, pgtype.Int4{Int32: userID, Valid: true})
		if err != nil {
			if err == pgx.ErrNoRows {
				return fmt.Errorf("user has no active subscription")
			}
			return err
		}

		// 2. Determine Type
		var pType postgres.PropertyType
		switch rentalType {
		case "long_term":
			pType = postgres.PropertyTypeLongTerm
		case "seasonal":
			pType = postgres.PropertyTypeSeasonal
		default:
			return fmt.Errorf("invalid rental type: %s", rentalType)
		}

		// 3. Quota
		if pType == postgres.PropertyTypeLongTerm {
			limit := sub.MaxPropertiesLimit.Int32
			currentCount, err := q.CountPropertiesByOwnerAndType(ctx, postgres.CountPropertiesByOwnerAndTypeParams{
				OwnerID:    pgtype.Int4{Int32: userID, Valid: true},
				RentalType: pType,
			})
			if err != nil {
				return err
			}

			if currentCount >= int64(limit) {
				log.Warn("quota exceeded", zap.Int("limit", int(limit)), zap.Int64("current", currentCount))
				return fmt.Errorf("property quota exceeded for current plan")
			}
		}

		// 4. Create
		rentNumeric := pgtype.Numeric{}
		rentNumeric.Scan(fmt.Sprintf("%f", rentAmount)) // Simplistic scan, better to use string if accurate

		depositNumeric := pgtype.Numeric{}
		depositNumeric.Scan(fmt.Sprintf("%f", depositAmount))

		prop, err = q.CreateProperty(ctx, postgres.CreatePropertyParams{
			OwnerID:       pgtype.Int4{Int32: userID, Valid: true},
			Address:       address,
			RentalType:    pType,
			Details:       []byte(detailsJSON),
			RentAmount:    rentNumeric,
			DepositAmount: depositNumeric,
		})
		if err != nil {
			return err
		}

		// 5. Initial Bonus (Discovery Plan + Long Term + First Time)
		if sub.PlanType == postgres.SubPlanDiscovery && pType == postgres.PropertyTypeLongTerm {
			hasBonus, err := q.HasReceivedInitialBonus(ctx, pgtype.Int4{Int32: userID, Valid: true})
			if err != nil {
				return err
			}

			if !hasBonus {
				// Grant 3 credits
				_, err = q.CreateCreditTransaction(ctx, postgres.CreateCreditTransactionParams{
					UserID:          pgtype.Int4{Int32: userID, Valid: true},
					Amount:          3,
					TransactionType: "initial_free",
					Description:     pgtype.Text{String: "Welcome Bonus: 3 credits", Valid: true},
				})
				if err != nil {
					return err
				}
				log.Info("initial bonus granted", zap.Int("user_id", int(userID)))
			}
		}

		return nil
	})

	if err != nil {
		log.Warn("create property failed", zap.Error(err))
		return nil, err
	}

	log.Info("property created", zap.Int("property_id", int(prop.ID)), zap.Int("user_id", int(userID)))
	return &prop, nil
}

func (s *PropertyService) ListProperties(ctx context.Context, userID int32) ([]postgres.Property, error) {
	var props []postgres.Property
	err := s.txManager.WithTx(ctx, func(q postgres.Querier) error {
		var err error
		props, err = q.ListPropertiesByOwner(ctx, pgtype.Int4{Int32: userID, Valid: true})
		return err
	})
	return props, err

}

func (s *PropertyService) DeleteProperty(ctx context.Context, userID int32, propertyID int32) error {
	log := logger.FromContext(ctx)

	err := s.txManager.WithTx(ctx, func(q postgres.Querier) error {
		// Attempt to soft delete.
		// If the property doesn't exist or doesn't belong to the user, no row will be returned/updated (depending on driver/sqlc behavior).
		// efficient way: RETURNING id will help us know if it matched.
		deletedID, err := q.SoftDeleteProperty(ctx, postgres.SoftDeletePropertyParams{
			ID:      propertyID,
			OwnerID: pgtype.Int4{Int32: userID, Valid: true},
		})
		if err != nil {
			if err == pgx.ErrNoRows {
				return fmt.Errorf("property not found or access denied")
			}
			return err
		}

		// If we got here, deletedID is set (it's a scalar if :one).
		// Wait, sqlc :one returns ErrNoRows if no rows are returned.
		// So the check above covers the "not found / not owner" case.

		log.Info("property soft deleted", zap.Int("property_id", int(deletedID)))
		return nil
	})

	if err != nil {
		log.Warn("delete property failed", zap.Error(err))
		return err
	}
	return nil
}
