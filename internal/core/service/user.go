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

type UserContext string

const (
	ContextOwner  UserContext = "owner"
	ContextTenant UserContext = "tenant"
	ContextNone   UserContext = "none"
)

type Capabilities struct {
	CanActAsOwner  bool `json:"can_act_as_owner"`
	CanActAsTenant bool `json:"can_act_as_tenant"`
}

type SubscriptionDTO struct {
	PlanType           string `json:"plan_type"`
	Frequency          string `json:"frequency,omitempty"`
	Status             string `json:"status"`
	StartDate          string `json:"start_date,omitempty"`
	EndDate            string `json:"end_date,omitempty"`
	MaxPropertiesLimit int32  `json:"max_properties_limit"`
}

type UserProfile struct {
	Subscription  *SubscriptionDTO `json:"subscription"`
	CreditBalance int32            `json:"credit_balance"`
}

type AuthResponse struct {
	User           *postgres.User `json:"-"`
	CurrentContext UserContext    `json:"current_context"`
	Capabilities   Capabilities   `json:"capabilities"`
	Profile        UserProfile    `json:"user_profile"`
}

type UserService struct {
	txManager TxManager
	log       *zap.Logger
}

func NewUserService(txManager TxManager, l *zap.Logger) *UserService {
	return &UserService{
		txManager: txManager,
		log:       l,
	}
}

// Register creates a new user if the email is not already taken.
func (s *UserService) Register(ctx context.Context, email, password, firstName, lastName, phone, inviteToken string) (*postgres.User, error) {
	log := logger.FromContext(ctx)
	var user postgres.User

	err := s.txManager.WithTx(ctx, func(q postgres.Querier) error {
		// 1. Check uniqueness
		_, err := q.GetUserByEmail(ctx, email)
		if err == nil {
			log.Warn("registration failed: email already exists", zap.String("email", email))
			return fmt.Errorf("user with email %s already exists", email)
		}
		if err != pgx.ErrNoRows {
			log.Error("registration check failed", zap.Error(err))
			return err
		}

		// 2. Create User
		hashedPassword := "hashed_" + password

		// Determine Initial Context
		initialContext := "owner"
		if inviteToken != "" {
			// Validate Token existence (Minimal check here, more robust would be check status/validity)
			// Assuming caller might validate or we trust token implies tenant intent.
			// Let's check if invitation exists to be safe and set tenant context.
			_, err := q.GetInvitationByToken(ctx, inviteToken)
			if err == nil {
				initialContext = "tenant"
			} else {
				log.Error("invalid invitation token", zap.Error(err))
				return fmt.Errorf("invalid invitation token")
			}
		}

		params := postgres.CreateUserParams{
			Email:           email,
			PasswordHash:    hashedPassword,
			FirstName:       pgtype.Text{String: firstName, Valid: firstName != ""},
			LastName:        pgtype.Text{String: lastName, Valid: lastName != ""},
			PhoneNumber:     pgtype.Text{String: phone, Valid: phone != ""},
			LastContextUsed: pgtype.Text{String: initialContext, Valid: true},
		}

		user, err = q.CreateUser(ctx, params)
		if err != nil {
			log.Error("registration failed during creation", zap.Error(err))
			return err
		}
		return nil
	})

	if err != nil {
		return nil, err
	}

	log.Info("user registered successfully", zap.Int("user_id", int(user.ID)), zap.String("email", email))
	return &user, nil
}

// Login authenticates a user by email and password and returns context.
func (s *UserService) Login(ctx context.Context, email, password string) (*AuthResponse, error) {
	log := logger.FromContext(ctx)
	var user postgres.User
	err := s.txManager.WithTx(ctx, func(q postgres.Querier) error {
		var err error
		user, err = q.GetUserByEmail(ctx, email)
		if err != nil {
			if err == pgx.ErrNoRows {
				return fmt.Errorf("invalid credentials")
			}
			return err
		}
		return nil
	})

	if err != nil {
		if err.Error() == "invalid credentials" {
			log.Warn("login failed: user not found or invalid", zap.String("email", email))
			return nil, err
		}
		log.Error("login failed: db error", zap.Error(err))
		return nil, err
	}

	// 2. Validate Password
	hashedInput := "hashed_" + password
	if user.PasswordHash != hashedInput {
		log.Warn("login failed: invalid password", zap.String("email", email))
		return nil, fmt.Errorf("invalid credentials")
	}

	// 3. Get Full Response (Context, Capabilities, Profile)
	return s.GetFullAuthResponse(ctx, &user)
}

// GetFullAuthResponse constructs the full user state including context, capabilities, and profile.
// This is useful for Login and for refreshing state after key actions (e.g. Subscription).
func (s *UserService) GetFullAuthResponse(ctx context.Context, user *postgres.User) (*AuthResponse, error) {
	log := logger.FromContext(ctx)
	var caps Capabilities
	var currentContext UserContext = ContextNone
	var subscription postgres.Subscription
	var creditBalance int32

	err := s.txManager.WithTx(ctx, func(q postgres.Querier) error {
		// Check Owner Capability
		countProps, err := q.CountPropertiesByOwner(ctx, pgtype.Int4{Int32: user.ID, Valid: true})
		if err != nil {
			return err
		}
		caps.CanActAsOwner = countProps > 0

		// Check Tenant Capability
		countLeases, err := q.CountLeasesByTenant(ctx, pgtype.Int4{Int32: user.ID, Valid: true})
		if err != nil {
			return err
		}
		countBookings, err := q.CountBookingsByTenant(ctx, pgtype.Int4{Int32: user.ID, Valid: true})
		if err != nil {
			return err
		}
		caps.CanActAsTenant = (countLeases > 0) || (countBookings > 0)

		// Fetch Subscription
		subscription, err = q.GetUserSubscription(ctx, pgtype.Int4{Int32: user.ID, Valid: true})
		if err != nil && err != pgx.ErrNoRows {
			log.Warn("failed to fetch subscription", zap.Error(err))
		}

		// Fetch Credit Balance
		balance, err := q.GetUserCreditBalance(ctx, pgtype.Int4{Int32: user.ID, Valid: true})
		if err != nil && err != pgx.ErrNoRows {
			log.Warn("failed to fetch credit balance", zap.Error(err))
		}
		creditBalance = balance

		return nil
	})

	if err != nil {
		return nil, err
	}

	// Determine Context
	// 1. Sticky State: Use LastContextUsed from DB
	if user.LastContextUsed.Valid && user.LastContextUsed.String != "" {
		pref := UserContext(user.LastContextUsed.String)
		// For Owner: Always allow if preference is Owner (Intent-based)
		if pref == ContextOwner {
			currentContext = ContextOwner
		} else if pref == ContextTenant && caps.CanActAsTenant {
			// For Tenant: Must have valid link (Lease/Booking/Invitation)
			currentContext = ContextTenant
		}
	}

	// 2. Smart Default (Fallback if Sticky State not set or invalid)
	if currentContext == ContextNone {
		if caps.CanActAsOwner {
			currentContext = ContextOwner
		} else if caps.CanActAsTenant {
			currentContext = ContextTenant
		}
	}

	return &AuthResponse{
		User:           user,
		CurrentContext: currentContext,
		Capabilities:   caps,
		Profile: UserProfile{
			Subscription: func() *SubscriptionDTO {
				if subscription.ID == 0 {
					return nil
				}
				dto := &SubscriptionDTO{
					PlanType:           string(subscription.PlanType),
					Status:             subscription.Status.String,
					MaxPropertiesLimit: subscription.MaxPropertiesLimit.Int32,
				}
				if subscription.Frequency.Valid {
					dto.Frequency = string(subscription.Frequency.BillingFreq)
				}
				if subscription.StartDate.Valid {
					dto.StartDate = subscription.StartDate.Time.String()
				}
				if subscription.EndDate.Valid {
					dto.EndDate = subscription.EndDate.Time.String()
				}
				return dto
			}(),
			CreditBalance: creditBalance,
		},
	}, nil
}

// GetUserByID fetches a user by ID.
func (s *UserService) GetUserByID(ctx context.Context, userID int32) (*postgres.User, error) {
	var user postgres.User
	err := s.txManager.WithTx(ctx, func(q postgres.Querier) error {
		u, err := q.GetUserById(ctx, int32(userID))
		if err != nil {
			return err
		}
		user = u
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// SwitchContext updates the user's preferred context.
func (s *UserService) SwitchContext(ctx context.Context, userID int32, targetContext string) error {
	return s.txManager.WithTx(ctx, func(q postgres.Querier) error {
		// Validations could be added (Check if user HAS capability for targetContext)
		// For MVP, just update.

		err := q.UpdateLastContext(ctx, postgres.UpdateLastContextParams{
			ID:              userID,
			LastContextUsed: pgtype.Text{String: targetContext, Valid: true},
		})
		return err
	})
}
