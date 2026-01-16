package service

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"go.uber.org/zap"

	"seculoc-back/internal/adapter/storage/postgres"
	"seculoc-back/internal/platform/email"
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

type LeaseGenerator interface {
	GenerateAndSave(ctx context.Context, leaseID int32) error
}

type UserService struct {
	txManager    TxManager
	log          *zap.Logger
	emailSender  email.EmailSender
	frontendURL  string
	leaseService LeaseGenerator
}

func NewUserService(txManager TxManager, l *zap.Logger, emailSender email.EmailSender, frontendURL string, leaseService LeaseGenerator) *UserService {
	return &UserService{
		txManager:    txManager,
		log:          l,
		emailSender:  emailSender,
		frontendURL:  frontendURL,
		leaseService: leaseService,
	}
}

// Register creates a new user if the email is not already taken.
func (s *UserService) Register(ctx context.Context, email, password, firstName, lastName, phone, inviteToken string) (*postgres.User, error) {
	log := logger.FromContext(ctx)
	var user postgres.User

	var leaseID int32
	err := s.txManager.WithTx(ctx, func(q postgres.Querier) error {
		// 1. Check uniqueness or provisional status
		existingUser, err := q.GetUserByEmail(ctx, email)
		if err == nil {
			// If user exists, they must be provisional to proceed with registration
			if !existingUser.IsProvisional.Bool {
				log.Warn("registration failed: email already exists and not provisional", zap.String("email", email))
				return fmt.Errorf("user with email %s already exists", email)
			}
			// Promotion case: we will update this user instead of creating a new one
			log.Info("promoting provisional user", zap.String("email", email))
		} else if err != pgx.ErrNoRows {
			log.Error("registration check failed", zap.Error(err))
			return err
		}

		// 2. Prepare User Data
		hashedPassword := "hashed_" + password
		initialContext := "owner" // Default

		// 3. Create or Promote User
		if existingUser.ID != 0 {
			// Update provisional user
			// We need a query to "promote" the user. Let's assume UpdateUserPromotion exists or we use UpdateLastContext + something else.
			// Actually, let's add UpdateUserPromotion to query.sql too.
			err = q.UpdateUserPromotion(ctx, postgres.UpdateUserPromotionParams{
				ID:           existingUser.ID,
				PasswordHash: pgtype.Text{String: hashedPassword, Valid: true},
				FirstName:    pgtype.Text{String: firstName, Valid: firstName != ""},
				LastName:     pgtype.Text{String: lastName, Valid: lastName != ""},
				PhoneNumber:  pgtype.Text{String: phone, Valid: phone != ""},
			})
			if err != nil {
				return err
			}
			user, _ = q.GetUserById(ctx, existingUser.ID)
		} else {
			params := postgres.CreateUserParams{
				Email:           email,
				PasswordHash:    pgtype.Text{String: hashedPassword, Valid: true},
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
		}

		// 4. Handle Invitation (if present)
		if inviteToken != "" {
			// Find Invitation
			inv, err := q.GetInvitationByToken(ctx, inviteToken)
			if err != nil {
				if err == pgx.ErrNoRows {
					log.Warn("invalid invitation token during register", zap.String("token", inviteToken))
					return fmt.Errorf("invalid invitation token")
				}
				return err
			}

			// Validate Status/Expiry
			if inv.Status.String != "pending" {
				return fmt.Errorf("invitation is not pending")
			}
			if inv.ExpiresAt.Time.Before(time.Now()) {
				return fmt.Errorf("invitation expired")
			}

			// Get Property to get Rent/Deposit
			prop, err := q.GetProperty(ctx, inv.PropertyID)
			if err != nil {
				return fmt.Errorf("property not found for invitation")
			}

			// Create Lease
			// Assuming RentAmount and DepositAmount in Property are valid (handled in CreateProperty)
			// They are Nullable in DB? I set them as DECIMAL(10,2) in schema, default nullable.
			// pgtype.Numeric needs handling.

			lease, err := q.CreateLease(ctx, postgres.CreateLeaseParams{
				PropertyID:    pgtype.Int4{Int32: inv.PropertyID, Valid: true},
				TenantID:      pgtype.Int4{Int32: user.ID, Valid: true},
				StartDate:     pgtype.Date{Time: time.Now(), Valid: true},
				RentAmount:    prop.RentAmount,
				DepositAmount: prop.DepositAmount,
			})
			if err != nil {
				return fmt.Errorf("failed to link property: %w", err)
			}
			leaseID = lease.ID

			// Update Invitation Status
			err = q.UpdateInvitationStatus(ctx, postgres.UpdateInvitationStatusParams{
				ID:     inv.ID,
				Status: pgtype.Text{String: "accepted", Valid: true},
			})
			if err != nil {
				return err
			}

			// Update User Context to Tenant
			// We already created user with 'owner', let's update it or ideally set it in CreateUser.
			// Since we created user above, let's update it here.
			err = q.UpdateLastContext(ctx, postgres.UpdateLastContextParams{
				ID:              user.ID,
				LastContextUsed: pgtype.Text{String: "tenant", Valid: true},
			})
			if err != nil {
				return err
			}

			// Update local user object for return
			user.LastContextUsed = pgtype.Text{String: "tenant", Valid: true}
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	// Generate and Store Lease Document (Post-Transaction)
	if leaseID != 0 {
		// Use fire-and-forget or sync? Sync is safer for immediate availability.
		if err := s.leaseService.GenerateAndSave(ctx, leaseID); err != nil {
			log.Error("failed to generate lease document after register", zap.Error(err))
		}
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
	if user.PasswordHash.String != hashedInput {
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
		// In an Airbnb-style model, any user can potentially be an owner (create properties).
		// We shouldn't restrict this based on having existing properties.
		// However, knowing if they have properties is useful for UI hints, but the capability itself is broad.
		// For now, let's allow everyone to act as owner.
		caps.CanActAsOwner = true

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
// SwitchContext updates the user's preferred context and returns fresh auth data.
func (s *UserService) SwitchContext(ctx context.Context, userID int32, targetContext string) (*AuthResponse, error) {
	log := logger.FromContext(ctx)
	var authResp *AuthResponse

	// 1. Verify Capabilities & Update
	err := s.txManager.WithTx(ctx, func(q postgres.Querier) error {
		// Validations: Check if user HAS capability for targetContext
		var allowed bool
		if targetContext == "owner" {
			// Owner is allowed for everyone (Airbnb style)
			allowed = true
		} else if targetContext == "tenant" {
			// Check if they are actually a tenant (leases or bookings)
			countLeases, err := q.CountLeasesByTenant(ctx, pgtype.Int4{Int32: userID, Valid: true})
			if err != nil {
				return err
			}
			countBookings, err := q.CountBookingsByTenant(ctx, pgtype.Int4{Int32: userID, Valid: true})
			if err != nil {
				return err
			}
			allowed = (countLeases > 0) || (countBookings > 0)
		} else {
			return fmt.Errorf("invalid context: %s", targetContext)
		}

		if !allowed {
			return fmt.Errorf("user does not have capability to switch to %s", targetContext)
		}

		// Update DB
		err := q.UpdateLastContext(ctx, postgres.UpdateLastContextParams{
			ID:              userID,
			LastContextUsed: pgtype.Text{String: targetContext, Valid: true},
		})
		if err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		log.Warn("switch context failed", zap.Error(err))
		return nil, err
	}

	// 2. Fetch Fresh Data (Profile, Capabilities, etc.)
	// fetching user first
	user, err := s.GetUserByID(ctx, userID)
	if err != nil {
		return nil, err
	}

	// This will calculate capabilities fresh and return the response.
	// Note: GetFullAuthResponse respects LastContextUsed from DB, which we just updated.
	authResp, err = s.GetFullAuthResponse(ctx, user)
	if err != nil {
		return nil, err
	}

	// Sanity verify: Ensure returned context matches desired target
	if string(authResp.CurrentContext) != targetContext {
		log.Warn("switch context mismatch",
			zap.String("target", targetContext),
			zap.String("actual", string(authResp.CurrentContext)))
		// Proceeding anyway, but logging warning. Logic in GetFullAuthResponse should honor sticky state if valid.
	}

	return authResp, nil
}
