package service

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"go.uber.org/zap"

	"seculoc-back/internal/adapter/storage/postgres"
	"seculoc-back/internal/platform/logger"
)

type PropertyService struct {
	q   postgres.Querier
	log *zap.Logger
}

func NewPropertyService(q postgres.Querier, l *zap.Logger) *PropertyService {
	return &PropertyService{
		q:   q,
		log: l,
	}
}

// CreateProperty creates a new property after verifying subscription quotas.
func (s *PropertyService) CreateProperty(ctx context.Context, userID int32, address string, rentalType string, detailsJSON string) (*postgres.Property, error) {
	log := logger.FromContext(ctx)

	// 1. Get User Subscription (to check quota)
	sub, err := s.q.GetUserSubscription(ctx, pgtype.Int4{Int32: userID, Valid: true})
	if err != nil {
		if err == pgx.ErrNoRows {
			log.Warn("create property failed: no subscription", zap.Int("user_id", int(userID)))
			return nil, fmt.Errorf("user has no active subscription")
		}
		return nil, err
	}

	// 2. Determine Property Type
	var pType postgres.PropertyType
	switch rentalType {
	case "long_term":
		pType = postgres.PropertyTypeLongTerm
	case "seasonal":
		pType = postgres.PropertyTypeSeasonal
	default:
		return nil, fmt.Errorf("invalid rental type: %s", rentalType)
	}

	// 3. Check Quota Logic
	// Rule: Seasonal is always allowed (Unlimited).
	// Rule: Long Term is restricted by plan limit.
	if pType == postgres.PropertyTypeLongTerm {
		limit := sub.MaxPropertiesLimit.Int32

		currentCount, err := s.q.CountPropertiesByOwnerAndType(ctx, postgres.CountPropertiesByOwnerAndTypeParams{
			OwnerID:    pgtype.Int4{Int32: userID, Valid: true},
			RentalType: pType,
		})
		if err != nil {
			return nil, err
		}

		if currentCount >= int64(limit) {
			log.Warn("create property failed: quota exceeded",
				zap.Int("user_id", int(userID)),
				zap.Int("limit", int(limit)),
				zap.Int64("current_count", currentCount),
				zap.String("type", rentalType),
			)
			return nil, fmt.Errorf("property quota exceeded for current plan")
		}
	} else {
		log.Debug("seasonal property creation - skipping quota check", zap.Int("user_id", int(userID)))
	}

	// 4. Create Property
	prop, err := s.q.CreateProperty(ctx, postgres.CreatePropertyParams{
		OwnerID:    pgtype.Int4{Int32: userID, Valid: true},
		Address:    address,
		RentalType: pType,
		Details:    []byte(detailsJSON),
	})
	if err != nil {
		log.Error("create property failed: db error", zap.Error(err))
		return nil, err
	}

	log.Info("property created", zap.Int("property_id", int(prop.ID)), zap.Int("user_id", int(userID)))
	return &prop, nil
}

func (s *PropertyService) ListProperties(ctx context.Context, userID int32) ([]postgres.Property, error) {
	return s.q.ListPropertiesByOwner(ctx, pgtype.Int4{Int32: userID, Valid: true})
}
