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

	// Send Email
	inviteLink := fmt.Sprintf("%s/register?token=%s", s.frontendURL, token)
	err = s.emailSender.SendInvitation(ctx, email, inviteLink)
	if err != nil {
		log.Warn("failed to send invitation email", zap.Error(err))
		// We don't fail the request, but valid to warn.
	}

	log.Info("invitation created and sent", zap.String("email", email))

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
		if inv.Status.String != "pending" {
			return fmt.Errorf("invitation is not pending")
		}
		// Check expiry
		if inv.ExpiresAt.Time.Before(time.Now()) {
			return fmt.Errorf("invitation expired")
		}

		// 3. Get Property Details (Rent/Deposit)
		prop, err := q.GetProperty(ctx, inv.PropertyID)
		if err != nil {
			return fmt.Errorf("property not found")
		}

		// 4. Link User (Create Lease Draft)
		_, err = q.CreateLease(ctx, postgres.CreateLeaseParams{
			PropertyID:    pgtype.Int4{Int32: inv.PropertyID, Valid: true},
			TenantID:      pgtype.Int4{Int32: userID, Valid: true},
			StartDate:     pgtype.Date{Time: time.Now(), Valid: true},
			RentAmount:    prop.RentAmount,
			DepositAmount: prop.DepositAmount,
		})
		if err != nil {
			return fmt.Errorf("failed to create lease: %w", err)
		}

		// 5. Update Invitation Status
		err = q.UpdateInvitationStatus(ctx, postgres.UpdateInvitationStatusParams{
			ID:     inv.ID,
			Status: pgtype.Text{String: "accepted", Valid: true},
		})

		return err
	})
}

// InvitationDetailsDTO contains info for the public landing page
type InvitationDetailsDTO struct {
	Email           string  `json:"email"`
	PropertyAddress string  `json:"property_address"`
	RentAmount      float64 `json:"rent_amount"`
	DepositAmount   float64 `json:"deposit_amount"`
	OwnerName       string  `json:"owner_name"`
}

// GetInvitationDetails retrieves limited details for the invitation landing page.
func (s *UserService) GetInvitationDetails(ctx context.Context, token string) (*InvitationDetailsDTO, error) {
	var details InvitationDetailsDTO

	err := s.txManager.WithTx(ctx, func(q postgres.Querier) error {
		// 1. Get Inv
		inv, err := q.GetInvitationByToken(ctx, token)
		if err != nil {
			return err
		}

		if inv.Status.String != "pending" {
			return fmt.Errorf("invitation invalid or expired")
		}
		if inv.ExpiresAt.Time.Before(time.Now()) {
			return fmt.Errorf("invitation expired")
		}

		details.Email = inv.TenantEmail

		// 2. Get Property
		prop, err := q.GetProperty(ctx, inv.PropertyID)
		if err != nil {
			return err
		}
		details.PropertyAddress = prop.Address
		rent, _ := prop.RentAmount.Float64Value()
		deposit, _ := prop.DepositAmount.Float64Value()
		details.RentAmount = rent.Float64
		details.DepositAmount = deposit.Float64

		// 3. Get Owner Name
		owner, err := q.GetUserById(ctx, inv.OwnerID)
		if err != nil {
			return err
		}
		details.OwnerName = fmt.Sprintf("%s %s", owner.FirstName.String, owner.LastName.String)

		return nil
	})

	if err != nil {
		return nil, err
	}
	return &details, nil
}
