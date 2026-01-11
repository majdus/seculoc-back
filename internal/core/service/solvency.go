package service

import (
	"context"
	"fmt"

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
		// 1. Check Credit Balance
		balance, err := q.GetUserCreditBalance(ctx, pgtype.Int4{Int32: userID, Valid: true})
		if err != nil {
			return err
		}

		if balance <= 0 {
			log.Warn("solvency check failed: insufficient credits", zap.Int("user_id", int(userID)), zap.Int("balance", int(balance)))
			return fmt.Errorf("insufficient credits for solvency check")
		}

		// 2. Consume Credit (Create Transaction)
		// Amount is -1 (Cost of 1 check)
		_, err = q.CreateCreditTransaction(ctx, postgres.CreateCreditTransactionParams{
			UserID:          pgtype.Int4{Int32: userID, Valid: true},
			Amount:          -1,
			TransactionType: "check_usage",
			Description:     pgtype.Text{String: "Solvency Check Request", Valid: true},
		})
		if err != nil {
			return fmt.Errorf("failed to deduct credit: %w", err)
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
		)
		return nil
	})

	if err != nil {
		return nil, err
	}

	return &check, nil
}
