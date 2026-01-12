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

type SolvencyService struct {
	txManager TxManager
	// We might need a logger if we want to log internal events effectively with fields
}

func NewSolvencyService(txManager TxManager, l *zap.Logger) *SolvencyService {
	return &SolvencyService{
		txManager: txManager,
	}
}

// RetrieveCheck checks credit balance, consumes a credit, and creates a solvency check request.
func (s *SolvencyService) RetrieveCheck(ctx context.Context, userID int32, candidateEmail string, propertyID int32) (*postgres.SolvencyCheck, error) {
	log := logger.FromContext(ctx)

	var check postgres.SolvencyCheck

	err := s.txManager.WithTx(ctx, func(q postgres.Querier) error {
		// 1. Verify Ownership & Get Property
		prop, err := q.GetProperty(ctx, propertyID)
		if err != nil {
			if err == pgx.ErrNoRows {
				return fmt.Errorf("property not found or access denied")
			}
			return err
		}

		if !prop.OwnerID.Valid || prop.OwnerID.Int32 != userID {
			return fmt.Errorf("property not found or access denied")
		}

		// 2. Hybrid Credit Deduction
		usedSource := "global"
		if prop.VacancyCredits > 0 {
			err = q.DecreasePropertyCredits(ctx, propertyID)
			if err != nil {
				return fmt.Errorf("failed to deduct property credit: %w", err)
			}
			usedSource = "property"
		} else {
			// Fallback to Global Credits
			balance, err := q.GetUserCreditBalance(ctx, pgtype.Int4{Int32: userID, Valid: true})
			if err != nil {
				if err == pgx.ErrNoRows {
					balance = 0
				} else {
					return err
				}
			}

			if balance <= 0 {
				log.Warn("solvency check failed: insufficient credits",
					zap.Int("user_id", int(userID)),
					zap.Int("prop_id", int(propertyID)),
					zap.Int32("vacancy_credits", prop.VacancyCredits),
					zap.Int("global_balance", int(balance)))
				return fmt.Errorf("insufficient credits for solvency check")
			}

			// Consume Global Credit
			_, err = q.CreateCreditTransaction(ctx, postgres.CreateCreditTransactionParams{
				UserID:          pgtype.Int4{Int32: userID, Valid: true},
				Amount:          -1,
				TransactionType: "check_usage",
				Description:     pgtype.Text{String: "Solvency Check Request (Global Wallet)", Valid: true},
			})
			if err != nil {
				return fmt.Errorf("failed to deduct global credit: %w", err)
			}
		}

		// 3. Create Solvency Check
		check, err = q.CreateSolvencyCheck(ctx, postgres.CreateSolvencyCheckParams{
			InitiatorOwnerID: pgtype.Int4{Int32: userID, Valid: true},
			CandidateEmail:   candidateEmail,
			PropertyID:       pgtype.Int4{Int32: propertyID, Valid: true},
		})
		if err != nil {
			return fmt.Errorf("failed to create solvency check: %w", err)
		}

		log.Info("solvency check initiated",
			zap.Int("user_id", int(userID)),
			zap.Int("check_id", int(check.ID)),
			zap.String("candidate", candidateEmail),
			zap.String("credit_source", usedSource),
		)
		return nil
	})

	if err != nil {
		return nil, err
	}

	return &check, nil
}

// PurchaseCredits allows users to buy credit packs (e.g., 20 checks).
func (s *SolvencyService) PurchaseCredits(ctx context.Context, userID int32, packType string) (int32, error) {
	log := logger.FromContext(ctx)

	var amount int32
	var cost float64

	switch packType {
	case "pack_20":
		amount = 20
		cost = 19.90 // Assigning arbitrary price for logic completeness, e.g. 1€/unit approx
	default:
		return 0, fmt.Errorf("invalid pack type: %s", packType)
	}

	err := s.txManager.WithTx(ctx, func(q postgres.Querier) error {
		_, err := q.CreateCreditTransaction(ctx, postgres.CreateCreditTransactionParams{
			UserID:          pgtype.Int4{Int32: userID, Valid: true},
			Amount:          amount,
			TransactionType: "pack_purchase",
			Description:     pgtype.Text{String: fmt.Sprintf("Purchase %s", packType), Valid: true},
		})
		if err != nil {
			return err
		}

		log.Info("credit pack purchased",
			zap.Int("user_id", int(userID)),
			zap.String("pack", packType),
			zap.Int("amount_added", int(amount)),
			zap.Float64("cost", cost),
		)
		return nil
	})

	if err != nil {
		return 0, err
	}

	return amount, nil
}
