package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/spf13/viper"
	"go.uber.org/zap"

	"seculoc-back/internal/adapter/storage/postgres"
	"seculoc-back/internal/platform/email"
	"seculoc-back/internal/platform/logger"
)

type SolvencyService struct {
	txManager   TxManager
	emailSender email.EmailSender
}

// ErrInsufficientCredits is returned when no credits are available.
type ErrInsufficientCredits struct {
	GlobalBalance   int32
	PropertyBalance int32
}

func (e *ErrInsufficientCredits) Error() string {
	return "insufficient credits for solvency check"
}

func NewSolvencyService(txManager TxManager, emailSender email.EmailSender, l *zap.Logger) *SolvencyService {
	return &SolvencyService{
		txManager:   txManager,
		emailSender: emailSender,
	}
}

type InitiateCheckParams struct {
	UserID             int32
	CandidateEmail     string
	CandidateFirstName string
	CandidateLastName  string
	CandidatePhone     string
	PropertyID         int32
}

func (s *SolvencyService) InitiateCheck(ctx context.Context, params InitiateCheckParams) (*postgres.SolvencyCheck, error) {
	log := logger.FromContext(ctx)

	var check postgres.SolvencyCheck

	err := s.txManager.WithTx(ctx, func(q postgres.Querier) error {
		// 1. Verify Ownership & Get Property (with Lock)
		prop, err := q.GetPropertyForUpdate(ctx, params.PropertyID)
		if err != nil {
			if err == pgx.ErrNoRows {
				return fmt.Errorf("property not found or access denied")
			}
			return err
		}

		if !prop.OwnerID.Valid || prop.OwnerID.Int32 != params.UserID {
			return fmt.Errorf("property not found or access denied")
		}

		// 2. Hybrid Credit Deduction
		usedSource := "global"
		var globalBalance int32
		if prop.VacancyCredits > 0 {
			err = q.DecreasePropertyCredits(ctx, params.PropertyID)
			if err != nil {
				return fmt.Errorf("failed to deduct property credit: %w", err)
			}
			usedSource = "property"
		} else {
			// Fallback to Global Credits (with Lock on User)
			_, err = q.GetUserForUpdate(ctx, params.UserID)
			if err != nil {
				return fmt.Errorf("failed to lock user for credit deduction: %w", err)
			}

			balance, err := q.GetUserCreditBalanceForUpdate(ctx, pgtype.Int4{Int32: params.UserID, Valid: true})
			if err != nil {
				if err == pgx.ErrNoRows {
					balance = 0
				} else {
					return err
				}
			}
			globalBalance = balance

			if balance <= 0 {
				return &ErrInsufficientCredits{
					GlobalBalance:   globalBalance,
					PropertyBalance: prop.VacancyCredits,
				}
			}

			// Consume Global Credit
			_, err = q.CreateCreditTransaction(ctx, postgres.CreateCreditTransactionParams{
				UserID:          pgtype.Int4{Int32: params.UserID, Valid: true},
				Amount:          -1,
				TransactionType: "check_usage",
				Description:     pgtype.Text{String: "Solvency Check Request (Global Wallet)", Valid: true},
			})
			if err != nil {
				return fmt.Errorf("failed to deduct global credit: %w", err)
			}
		}

		// 3. Find or Create Candidate (User)
		var candidateID int32
		cand, err := q.GetUserByEmail(ctx, params.CandidateEmail)
		if err == nil {
			candidateID = cand.ID
		} else if err == pgx.ErrNoRows {
			// Create Provisional User
			newCand, err := q.CreateUser(ctx, postgres.CreateUserParams{
				Email:           params.CandidateEmail,
				FirstName:       pgtype.Text{String: params.CandidateFirstName, Valid: params.CandidateFirstName != ""},
				LastName:        pgtype.Text{String: params.CandidateLastName, Valid: params.CandidateLastName != ""},
				PhoneNumber:     pgtype.Text{String: params.CandidatePhone, Valid: params.CandidatePhone != ""},
				LastContextUsed: pgtype.Text{String: "tenant", Valid: true},
			})
			if err != nil {
				return fmt.Errorf("failed to create provisional candidate: %w", err)
			}
			candidateID = newCand.ID
		} else {
			return err
		}

		// 4. Generate Token (Secure, Opaque)
		bytes := make([]byte, 32)
		if _, err := rand.Read(bytes); err != nil {
			return fmt.Errorf("failed to generate secure token: %w", err)
		}
		tokenValue := hex.EncodeToString(bytes)

		// 5. Create Solvency Check
		check, err = q.CreateSolvencyCheck(ctx, postgres.CreateSolvencyCheckParams{
			InitiatorOwnerID: pgtype.Int4{Int32: params.UserID, Valid: true},
			CandidateID:      pgtype.Int4{Int32: candidateID, Valid: true},
			Token:            pgtype.Text{String: tokenValue, Valid: true},
			PropertyID:       pgtype.Int4{Int32: params.PropertyID, Valid: true},
			CreditSource:     pgtype.Text{String: usedSource, Valid: true},
		})
		if err != nil {
			return fmt.Errorf("failed to create solvency check: %w", err)
		}

		log.Info("solvency check initiated",
			zap.Int("user_id", int(params.UserID)),
			zap.Int("check_id", int(check.ID)),
			zap.String("candidate", params.CandidateEmail),
			zap.String("credit_source", usedSource),
		)
		return nil
	})

	if err != nil {
		return nil, err
	}

	// 6. Send Email Invitation (OUTSIDE TRANSACTION)
	baseURL := viper.GetString("FRONTEND_URL")
	if baseURL == "" {
		baseURL = "https://seculoc.com" // Fallback
	}
	invitationLink := fmt.Sprintf("%s/check/%s", baseURL, check.Token.String)
	emailErr := s.emailSender.SendInvitation(ctx, params.CandidateEmail, invitationLink)
	if emailErr != nil {
		// Log but don't fail the check creation. The check is already in DB.
		// Owner can resend it later from history if needed.
		logger.FromContext(ctx).Error("failed to send invitation email",
			zap.Error(emailErr),
			zap.Int32("check_id", check.ID),
			zap.String("email", params.CandidateEmail),
		)
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

type SolvencyCheckEnriched struct {
	postgres.SolvencyCheck
	CandidateEmail     string
	CandidateFirstName string
	CandidateLastName  string
	PropertyAddress    string
	PropertyRent       float64
	PropertyName       string
}

// GetCheckByToken retrieves a solvency check by its unique token.
func (s *SolvencyService) GetCheckByToken(ctx context.Context, token string) (*SolvencyCheckEnriched, error) {
	var check SolvencyCheckEnriched
	err := s.txManager.WithTx(ctx, func(q postgres.Querier) error {
		row, err := q.GetSolvencyCheckByToken(ctx, pgtype.Text{String: token, Valid: true})
		if err != nil {
			return err
		}
		check.SolvencyCheck = postgres.SolvencyCheck{
			ID:               row.ID,
			InitiatorOwnerID: row.InitiatorOwnerID,
			CandidateID:      row.CandidateID,
			Token:            row.Token,
			PropertyID:       row.PropertyID,
			Status:           row.Status,
			CreatedAt:        row.CreatedAt,
		}
		check.CandidateEmail = row.CandidateEmail
		check.CandidateFirstName = row.CandidateFirstName.String
		check.CandidateLastName = row.CandidateLastName.String
		check.PropertyAddress = row.PropertyAddress
		if row.PropertyRentAmount.Valid {
			f, _ := row.PropertyRentAmount.Float64Value()
			check.PropertyRent = f.Float64
		} else {
			check.PropertyRent = 0
		}
		check.PropertyName = row.PropertyName.String
		return nil
	})
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("check not found")
		}
		return nil, err
	}
	return &check, nil
}

// ListChecksForOwner returns all checks initiated by an owner.
func (s *SolvencyService) ListChecksForOwner(ctx context.Context, ownerID int32) ([]postgres.ListSolvencyChecksByOwnerRow, error) {
	var checks []postgres.ListSolvencyChecksByOwnerRow
	err := s.txManager.WithTx(ctx, func(q postgres.Querier) error {
		var err error
		checks, err = q.ListSolvencyChecksByOwner(ctx, pgtype.Int4{Int32: ownerID, Valid: true})
		return err
	})
	return checks, err
}

// ListChecksForProperty returns all checks for a specific property.
func (s *SolvencyService) ListChecksForProperty(ctx context.Context, propertyID int32) ([]postgres.ListSolvencyChecksByPropertyRow, error) {
	var checks []postgres.ListSolvencyChecksByPropertyRow
	err := s.txManager.WithTx(ctx, func(q postgres.Querier) error {
		var err error
		checks, err = q.ListSolvencyChecksByProperty(ctx, pgtype.Int4{Int32: propertyID, Valid: true})
		return err
	})
	return checks, err
}

type TransactionData struct {
	Amount      float64   `json:"amount"`
	Description string    `json:"description"`
	Date        time.Time `json:"date"`
}

// ProcessOpenBankingResult analyzes transactions and updates the solvency check status.
func (s *SolvencyService) ProcessOpenBankingResult(ctx context.Context, token string, transactions []TransactionData) error {
	log := logger.FromContext(ctx)

	check, err := s.GetCheckByToken(ctx, token)
	if err != nil {
		return err
	}

	if check.Status.SolvencyStatus != postgres.SolvencyStatusPending {
		return fmt.Errorf("check already processed")
	}

	// 1. Fetch Property to get Rent Amount
	var rentAmount float64
	err = s.txManager.WithTx(ctx, func(q postgres.Querier) error {
		prop, err := q.GetProperty(ctx, check.PropertyID.Int32)
		if err != nil {
			return err
		}
		if prop.RentAmount.Valid {
			f, _ := prop.RentAmount.Float64Value()
			rentAmount = f.Float64
		}
		return nil
	})
	if err != nil {
		return err
	}

	// 2. Simple Salary Detection (Heuristic: regular inbound amounts)
	// In a real app, this would be much more sophisticated (looking for "SALARY", "VIR", etc.)
	var totalMonthlyIncome float64
	// Simple mock logic: sum positive transactions and divide by months (let's assume 3 months of data)
	for _, tx := range transactions {
		if tx.Amount > 0 {
			totalMonthlyIncome += tx.Amount
		}
	}
	// Assume 3 months of data provided
	avgMonthlyIncome := totalMonthlyIncome / 3.0

	// 3. Apply 3x Rent Rule
	status := postgres.SolvencyStatusRejected
	if avgMonthlyIncome >= (rentAmount * 3.0) {
		status = postgres.SolvencyStatusApproved
	}

	// 4. Update Result
	err = s.txManager.WithTx(ctx, func(q postgres.Querier) error {
		return q.UpdateSolvencyCheckResult(ctx, postgres.UpdateSolvencyCheckResultParams{
			ID:          check.ID,
			Status:      postgres.NullSolvencyStatus{SolvencyStatus: status, Valid: true},
			ScoreResult: pgtype.Int4{Int32: int32(avgMonthlyIncome), Valid: true},
			ReportUrl:   pgtype.Text{String: "http://report-url.pdf", Valid: true}, // Mock
		})
	})

	log.Info("solvency check processed",
		zap.Int("check_id", int(check.ID)),
		zap.String("status", string(status)),
		zap.Float64("income", avgMonthlyIncome),
		zap.Float64("rent", rentAmount))

	return err
}

// CancelCheck cancels a pending solvency check and refunds the credit
func (s *SolvencyService) CancelCheck(ctx context.Context, checkID int32, ownerID int32) error {
	log := logger.FromContext(ctx).With(zap.Int32("check_id", checkID))

	return s.txManager.WithTx(ctx, func(q postgres.Querier) error {
		// 1. Fetch check and verify ownership/status
		check, err := q.GetSolvencyCheckByID(ctx, checkID)
		if err != nil {
			return fmt.Errorf("failed to fetch check: %w", err)
		}

		if check.InitiatorOwnerID.Int32 != ownerID {
			return fmt.Errorf("unauthorized: you don't own this check")
		}

		if check.Status.SolvencyStatus != postgres.SolvencyStatusPending {
			return fmt.Errorf("cannot cancel check with status: %s", check.Status.SolvencyStatus)
		}

		// 2. Mark as cancelled
		err = q.CancelSolvencyCheck(ctx, checkID)
		if err != nil {
			return fmt.Errorf("failed to cancel check: %w", err)
		}

		// 3. Refund credit
		source := check.CreditSource.String
		if source == "property" {
			err = q.IncreasePropertyCredits(ctx, check.PropertyID.Int32)
			if err != nil {
				return fmt.Errorf("failed to refund property credit: %w", err)
			}
			log.Info("refunded property credit", zap.Int32("property_id", check.PropertyID.Int32))
		} else if source == "global" {
			_, err = q.CreateCreditTransaction(ctx, postgres.CreateCreditTransactionParams{
				UserID:          check.InitiatorOwnerID,
				Amount:          1,
				TransactionType: "refund",
				Description:     pgtype.Text{String: fmt.Sprintf("Refund for cancelled solvency check #%d", checkID), Valid: true},
			})
			if err != nil {
				return fmt.Errorf("failed to refund global credit: %w", err)
			}
			log.Info("refunded global credit")
		}

		return nil
	})
}
