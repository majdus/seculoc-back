package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"go.uber.org/zap"

	"seculoc-back/internal/adapter/storage/postgres"
)

type FileStorage interface {
	Save(filename string, content []byte) (string, error)
	Get(filename string) ([]byte, error)
	Exists(filename string) bool
}

type LeaseService struct {
	txManager TxManager
	logger    *zap.Logger
	storage   FileStorage
}

func NewLeaseService(txManager TxManager, logger *zap.Logger, storage FileStorage) *LeaseService {
	return &LeaseService{txManager: txManager, logger: logger, storage: storage}
}

type LeaseDTO struct {
	ID              int32   `json:"id"`
	PropertyID      int32   `json:"property_id"`
	PropertyAddress string  `json:"property_address"`
	RentalType      string  `json:"rental_type"`
	StartDate       string  `json:"start_date"`
	EndDate         string  `json:"end_date,omitempty"`
	RentAmount      float64 `json:"rent_amount"`
	ChargesAmount   float64 `json:"charges_amount"` // Added
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
			charges, _ := l.ChargesAmount.Float64Value()
			deposit, _ := l.DepositAmount.Float64Value()

			dto := LeaseDTO{
				ID:              l.ID,
				PropertyID:      l.PropertyID.Int32,
				PropertyAddress: l.PropertyAddress,
				RentalType:      string(l.RentalType),
				StartDate:       l.StartDate.Time.Format("2006-01-02"),
				RentAmount:      rent.Float64,
				ChargesAmount:   charges.Float64,
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

	if leasesDTO == nil {
		leasesDTO = []LeaseDTO{}
	}

	return leasesDTO, nil
}

type DraftLeaseRequest struct {
	PropertyID int32       `json:"property_id" binding:"required"`
	TenantInfo TenantDraft `json:"tenant_info" binding:"required"`
	Terms      LeaseTerms  `json:"terms" binding:"required"`
	Clauses    []string    `json:"clauses"`
}

type TenantDraft struct {
	FirstName string `json:"first_name" binding:"required"`
	LastName  string `json:"last_name" binding:"required"`
	Email     string `json:"email" binding:"required,email"`
	Phone     string `json:"phone"`
}

type LeaseTerms struct {
	StartDate     string  `json:"start_date" binding:"required"` // YYYY-MM-DD
	EndDate       string  `json:"end_date"`                      // YYYY-MM-DD (Optional)
	RentAmount    float64 `json:"rent_amount" binding:"required"`
	ChargesAmount float64 `json:"charges_amount"`
	DepositAmount float64 `json:"deposit_amount" binding:"required"`
	PaymentDay    int     `json:"payment_day" binding:"required,min=1,max=31"`
}

func (s *LeaseService) CreateDraft(ctx context.Context, req DraftLeaseRequest, ownerID int32) (int32, string, error) {
	var leaseID int32
	var token string

	err := s.txManager.WithTx(ctx, func(q postgres.Querier) error {
		// 1. Verify Ownership
		prop, err := q.GetProperty(ctx, req.PropertyID)
		if err != nil {
			return fmt.Errorf("property not found: %w", err)
		}
		if prop.OwnerID.Int32 != ownerID {
			return fmt.Errorf("unauthorized: user does not own this property")
		}

		// 2. Parse Dates
		start, err := time.Parse("2006-01-02", req.Terms.StartDate)
		if err != nil {
			return fmt.Errorf("invalid start date: %w", err)
		}
		var end time.Time
		if req.Terms.EndDate != "" {
			end, err = time.Parse("2006-01-02", req.Terms.EndDate)
			if err != nil {
				return fmt.Errorf("invalid end date: %w", err)
			}
		}

		// 3. Prepare JSON Clauses
		clausesJSON, err := json.Marshal(req.Clauses)
		if err != nil {
			return fmt.Errorf("failed to marshal clauses: %w", err)
		}

		// 4. Create Draft Lease
		lease, err := q.CreateDraftLease(ctx, postgres.CreateDraftLeaseParams{
			PropertyID:     pgtype.Int4{Int32: req.PropertyID, Valid: true},
			StartDate:      pgtype.Date{Time: start, Valid: true},
			EndDate:        pgtype.Date{Time: end, Valid: !end.IsZero()},
			RentAmount:     numeric(req.Terms.RentAmount),
			ChargesAmount:  numeric(req.Terms.ChargesAmount),
			DepositAmount:  numeric(req.Terms.DepositAmount),
			PaymentDay:     pgtype.Int4{Int32: int32(req.Terms.PaymentDay), Valid: true},
			SpecialClauses: clausesJSON,
		})
		if err != nil {
			return fmt.Errorf("failed to create draft lease: %w", err)
		}
		leaseID = lease.ID

		// 5. Create Invitation
		token = generateToken()
		expiresAt := time.Now().Add(7 * 24 * time.Hour)

		_, err = q.CreateInvitationWithLease(ctx, postgres.CreateInvitationWithLeaseParams{
			PropertyID:  req.PropertyID,
			LeaseID:     pgtype.Int4{Int32: leaseID, Valid: true},
			OwnerID:     ownerID,
			TenantEmail: req.TenantInfo.Email,
			Token:       token,
			ExpiresAt:   pgtype.Timestamp{Time: expiresAt, Valid: true},
		})
		if err != nil {
			return fmt.Errorf("failed to create invitation: %w", err)
		}

		return nil
	})

	return leaseID, token, err
}

func numeric(f float64) pgtype.Numeric {
	s := fmt.Sprintf("%.2f", f)
	var n pgtype.Numeric
	err := n.Scan(s)
	if err != nil {
		return pgtype.Numeric{Valid: false}
	}
	return n
}

func generateToken() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
