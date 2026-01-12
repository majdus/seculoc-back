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
func (s *UserService) Register(ctx context.Context, email, password, firstName, lastName, phone string) (*postgres.User, error) {
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

		params := postgres.CreateUserParams{
			Email:        email,
			PasswordHash: hashedPassword,
			FirstName:    pgtype.Text{String: firstName, Valid: firstName != ""},
			LastName:     pgtype.Text{String: lastName, Valid: lastName != ""},
			PhoneNumber:  pgtype.Text{String: phone, Valid: phone != ""},
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

// Login authenticates a user by email and password.
func (s *UserService) Login(ctx context.Context, email, password string) (*postgres.User, error) {
	log := logger.FromContext(ctx)
	var user postgres.User

	err := s.txManager.WithTx(ctx, func(q postgres.Querier) error {
		var err error
		user, err = q.GetUserByEmail(ctx, email)
		if err != nil {
			if err == pgx.ErrNoRows {
				// Don't log email for security in some contexts, but here warn is fine
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

	log.Info("user logged in successfully", zap.Int("user_id", int(user.ID)))
	return &user, nil
}
