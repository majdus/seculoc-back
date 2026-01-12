package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"go.uber.org/zap"

	"seculoc-back/internal/adapter/storage/postgres"
	"seculoc-back/internal/platform/logger"
)

// GenerateSecureToken generates a random token for invitations.
func GenerateSecureToken() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

// InviteTenant creates an invitation for a tenant to join a property.
func (s *UserService) InviteTenant(ctx context.Context, ownerID int32, propertyID int32, email string) (*postgres.LeaseInvitation, error) {
	log := logger.FromContext(ctx)

	// 1. Validate permissions (User must be owner of the property)
	// Ideally we check this. For now, assuming caller checked or we check inside Tx.

	var invitation postgres.LeaseInvitation
	token, err := GenerateSecureToken()
	if err != nil {
		return nil, err
	}

	err = s.txManager.WithTx(ctx, func(q postgres.Querier) error {
		// Verify property ownership
		prop, err := q.GetProperty(ctx, int32(propertyID))
		if err != nil {
			return fmt.Errorf("property not found")
		}
		if prop.OwnerID.Int32 != ownerID {
			return fmt.Errorf("unauthorized: user is not the owner of this property")
		}

		// Create Invitation
		// Expires in 7 days
		expiresAt := time.Now().Add(7 * 24 * time.Hour)

		params := postgres.CreateInvitationParams{
			PropertyID:  int32(propertyID),
			OwnerID:     int32(ownerID),
			TenantEmail: email,
			Token:       token,
			ExpiresAt:   pgtype.Timestamp{Time: expiresAt, Valid: true},
		}

		invitation, err = q.CreateInvitation(ctx, params)
		if err != nil {
			log.Error("failed to create invitation", zap.Error(err))
			return err
		}
		return nil
	})

	if err != nil {
		return nil, err
	}

	// TODO: Send Email (Mocked for now)
	log.Info("invitation created (email mocked)", zap.String("email", email), zap.String("token", token))

	return &invitation, nil
}

// AcceptInvitation allows a user to accept an invitation using a token.
func (s *UserService) AcceptInvitation(ctx context.Context, token string, userID int32) error {

	return s.txManager.WithTx(ctx, func(q postgres.Querier) error {
		// 1. Get Invitation
		inv, err := q.GetInvitationByToken(ctx, token)
		if err != nil {
			if err == pgx.ErrNoRows {
				return fmt.Errorf("invalid token")
			}
			return err
		}

		// 2. Validate Status and Expiry
		if inv.Status.String != "pending" { // Assuming status is handled as text in SQLC or we need to check generated type.
			// SQL Schema says DEFAULT 'pending'.
			// postgres.LeaseInvitation.Status is likely pgtype.Text or string?
			// Let's assume pgtype.Text based on other models.
			return fmt.Errorf("invitation is not pending")
		}
		// Check expiry
		if inv.ExpiresAt.Time.Before(time.Now()) {
			return fmt.Errorf("invitation expired")
		}

		// 3. Link User (Create Lease Draft)
		// For MVP, establishing link. Rent/Deposit set to 0 or placeholders as we don't have them in invitation.
		// Wait, RentAmount and DepositAmount are NOT NULL in schema.
		// We need values. Maybe get from Property details if available, or set 0 (placeholder).
		// Let's set 0 for now as 'draft'.

		// Correction: Using dummy values for now.
		zero := pgtype.Numeric{}
		zero.Scan("0")

		_, err = q.CreateLease(ctx, postgres.CreateLeaseParams{
			PropertyID:    pgtype.Int4{Int32: inv.PropertyID, Valid: true},
			TenantID:      pgtype.Int4{Int32: userID, Valid: true},
			StartDate:     pgtype.Date{Time: time.Now(), Valid: true},
			RentAmount:    zero,
			DepositAmount: zero,
		})
		if err != nil {
			return fmt.Errorf("failed to create lease: %w", err)
		}

		// 4. Update Invitation Status
		err = q.UpdateInvitationStatus(ctx, postgres.UpdateInvitationStatusParams{
			ID:     inv.ID,
			Status: pgtype.Text{String: "accepted", Valid: true}, // Check specific type
		})

		return err
	})
}
