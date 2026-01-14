package service

import (
	"context"

	"github.com/jackc/pgx/v5/pgtype"
	"go.uber.org/zap"

	"seculoc-back/internal/adapter/storage/postgres"
)

type LeaseService struct {
	txManager TxManager
	logger    *zap.Logger
}

func NewLeaseService(txManager TxManager, logger *zap.Logger) *LeaseService {
	return &LeaseService{txManager: txManager, logger: logger}
}

type LeaseDTO struct {
	ID              int32   `json:"id"`
	PropertyID      int32   `json:"property_id"`
	PropertyAddress string  `json:"property_address"`
	RentalType      string  `json:"rental_type"`
	StartDate       string  `json:"start_date"`
	EndDate         string  `json:"end_date,omitempty"`
	RentAmount      float64 `json:"rent_amount"`
	DepositAmount   float64 `json:"deposit_amount"`
	Status          string  `json:"status"`
	ContractURL     string  `json:"contract_url,omitempty"`
}

func (s *LeaseService) ListLeases(ctx context.Context, tenantID int32) ([]LeaseDTO, error) {
	var leasesDTO []LeaseDTO
	err := s.txManager.WithTx(ctx, func(q postgres.Querier) error {
		leases, err := q.ListLeasesByTenant(ctx, pgtype.Int4{Int32: tenantID, Valid: true})
		if err != nil {
			return err
		}

		for _, l := range leases {
			rent, _ := l.RentAmount.Float64Value()
			deposit, _ := l.DepositAmount.Float64Value()

			dto := LeaseDTO{
				ID:              l.ID,
				PropertyID:      l.PropertyID.Int32,
				PropertyAddress: l.PropertyAddress,
				RentalType:      string(l.RentalType),
				StartDate:       l.StartDate.Time.Format("2006-01-02"),
				RentAmount:      rent.Float64,
				DepositAmount:   deposit.Float64,
				Status:          l.LeaseStatus.String,
				ContractURL:     l.ContractUrl.String,
			}
			if l.EndDate.Valid {
				dto.EndDate = l.EndDate.Time.Format("2006-01-02")
			}

			leasesDTO = append(leasesDTO, dto)
		}
		return nil
	})

	if err != nil {
		s.logger.Error("failed to list leases", zap.Error(err))
		return nil, err
	}

	// Return empty slice instead of nil for better JSON serialization ([] vs null)
	if leasesDTO == nil {
		leasesDTO = []LeaseDTO{}
	}

	return leasesDTO, nil
}
