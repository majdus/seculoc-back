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

func (s *PropertyService) CreateProperty(ctx context.Context, userID int32, address string, rentalType string, detailsJSON string) (*postgres.Property, error) {
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
		prop, err = q.CreateProperty(ctx, postgres.CreatePropertyParams{
			OwnerID:    pgtype.Int4{Int32: userID, Valid: true},
			Address:    address,
			RentalType: pType,
			Details:    []byte(detailsJSON),
		})
		if err != nil {
			return err
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
