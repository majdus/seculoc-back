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

	// 2. Check Quota
	// If limit is 0 (e.g. Free plan discovery potentially?), but schema says default 0.
	// Let's assume 0 means NO properties allowed, or infinite?
	// Usually 0 means 0. For infinite we'd use -1 or NULL.
	limit := sub.MaxPropertiesLimit.Int32

	currentCount, err := s.q.CountPropertiesByOwner(ctx, pgtype.Int4{Int32: userID, Valid: true})
	if err != nil {
		return nil, err
	}

	if currentCount >= int64(limit) {
		log.Warn("create property failed: quota exceeded",
			zap.Int("user_id", int(userID)),
			zap.Int("limit", int(limit)),
			zap.Int64("current_count", currentCount),
		)
		return nil, fmt.Errorf("property quota exceeded for current plan")
	}

	// 3. Create Property
	// Map string to Enum
	var pType postgres.PropertyType
	switch rentalType {
	case "long_term":
		pType = postgres.PropertyTypeLongTerm
	case "seasonal":
		pType = postgres.PropertyTypeSeasonal
	default:
		return nil, fmt.Errorf("invalid rental type: %s", rentalType)
	}

	// Details JSON (pgtype.Text for now, or []byte for JSONB? generated code uses []byte usually for JSONB, or pgtype.JSONB?)
	// Check models.go... sqlc usually generates []byte for JSONB if not overriden.
	// Let's stick with CreatePropertyParams from generated code.
	// We'll see what the compiler says about `Details`.

	prop, err := s.q.CreateProperty(ctx, postgres.CreatePropertyParams{
		OwnerID:    pgtype.Int4{Int32: userID, Valid: true},
		Address:    address,
		RentalType: pType,
		// Details: ... we need to check the generated struct
		Details: []byte(detailsJSON),
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
