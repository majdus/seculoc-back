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

type UserService struct {
	q   postgres.Querier // We don't need TxManager for simple single-query ops? Actually explicit interfaces are better.
	log *zap.Logger
}

func NewUserService(q postgres.Querier, l *zap.Logger) *UserService {
	return &UserService{
		q:   q,
		log: l,
	}
}

// Register creates a new user if the email is not already taken.
// Note: Password hashing is omitted for this step but SHOULD be implemented.
func (s *UserService) Register(ctx context.Context, email, password, firstName, lastName, phone string) (*postgres.User, error) {
	log := logger.FromContext(ctx)

	// 1. Check uniqueness
	_, err := s.q.GetUserByEmail(ctx, email)
	if err == nil {
		// User found
		log.Warn("registration failed: email already exists", zap.String("email", email))
		return nil, fmt.Errorf("user with email %s already exists", email)
	}
	if err != pgx.ErrNoRows {
		// Real DB error
		log.Error("registration check failed", zap.Error(err))
		return nil, err
	}

	// 2. Create User
	// TODO: Hash Password (Argon2 or Bcrypt)
	hashedPassword := "hashed_" + password

	params := postgres.CreateUserParams{
		Email:        email,
		PasswordHash: hashedPassword,
		FirstName:    pgtype.Text{String: firstName, Valid: firstName != ""},
		LastName:     pgtype.Text{String: lastName, Valid: lastName != ""},
		PhoneNumber:  pgtype.Text{String: phone, Valid: phone != ""},
	}

	user, err := s.q.CreateUser(ctx, params)
	if err != nil {
		log.Error("registration failed during creation", zap.Error(err))
		return nil, err
	}

	log.Info("user registered successfully", zap.Int("user_id", int(user.ID)), zap.String("email", email))
	return &user, nil
}

// Login authenticates a user by email and password.
func (s *UserService) Login(ctx context.Context, email, password string) (*postgres.User, error) {
	log := logger.FromContext(ctx)

	// 1. Get User
	user, err := s.q.GetUserByEmail(ctx, email)
	if err != nil {
		if err == pgx.ErrNoRows {
			log.Warn("login failed: user not found", zap.String("email", email))
			return nil, fmt.Errorf("invalid credentials")
		}
		log.Error("login failed: db error", zap.Error(err))
		return nil, err
	}

	// 2. Refresh Hash (TODO: Use real crypto like bcrypt)
	hashedInput := "hashed_" + password

	// 3. Compare
	if user.PasswordHash != hashedInput {
		log.Warn("login failed: invalid password", zap.String("email", email))
		return nil, fmt.Errorf("invalid credentials")
	}

	log.Info("user logged in successfully", zap.Int("user_id", int(user.ID)))
	return &user, nil
}
