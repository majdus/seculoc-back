package service

import (
	"context"
	"seculoc-back/internal/adapter/storage/postgres"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/mock"
)

type MockTxManager struct {
	mock.Mock
}

func (m *MockTxManager) WithTx(ctx context.Context, fn func(postgres.Querier) error) error {
	args := m.Called(ctx, fn)
	return args.Error(0)
}

type MockQuerier struct {
	mock.Mock
}

func (m *MockQuerier) CancelSolvencyCheck(ctx context.Context, id int32) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockQuerier) CleanupProvisionalUsers(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *MockQuerier) CountBookingsByTenant(ctx context.Context, tenantID pgtype.Int4) (int64, error) {
	args := m.Called(ctx, tenantID)
	return args.Get(0).(int64), args.Error(1)
}

func (m *MockQuerier) CountLeasesByTenant(ctx context.Context, tenantID pgtype.Int4) (int64, error) {
	args := m.Called(ctx, tenantID)
	return args.Get(0).(int64), args.Error(1)
}

func (m *MockQuerier) CountPropertiesByOwner(ctx context.Context, ownerID pgtype.Int4) (int64, error) {
	args := m.Called(ctx, ownerID)
	return args.Get(0).(int64), args.Error(1)
}

func (m *MockQuerier) CountPropertiesByOwnerAndType(ctx context.Context, arg postgres.CountPropertiesByOwnerAndTypeParams) (int64, error) {
	args := m.Called(ctx, arg)
	return args.Get(0).(int64), args.Error(1)
}

func (m *MockQuerier) CreateCreditTransaction(ctx context.Context, arg postgres.CreateCreditTransactionParams) (postgres.CreditTransaction, error) {
	args := m.Called(ctx, arg)
	return args.Get(0).(postgres.CreditTransaction), args.Error(1)
}

func (m *MockQuerier) CreateInvitation(ctx context.Context, arg postgres.CreateInvitationParams) (postgres.LeaseInvitation, error) {
	args := m.Called(ctx, arg)
	return args.Get(0).(postgres.LeaseInvitation), args.Error(1)
}

func (m *MockQuerier) CreateLease(ctx context.Context, arg postgres.CreateLeaseParams) (postgres.Lease, error) {
	args := m.Called(ctx, arg)
	return args.Get(0).(postgres.Lease), args.Error(1)
}

func (m *MockQuerier) CreateProperty(ctx context.Context, arg postgres.CreatePropertyParams) (postgres.Property, error) {
	args := m.Called(ctx, arg)
	return args.Get(0).(postgres.Property), args.Error(1)
}

func (m *MockQuerier) CreateSolvencyCheck(ctx context.Context, arg postgres.CreateSolvencyCheckParams) (postgres.SolvencyCheck, error) {
	args := m.Called(ctx, arg)
	return args.Get(0).(postgres.SolvencyCheck), args.Error(1)
}

func (m *MockQuerier) CreateSubscription(ctx context.Context, arg postgres.CreateSubscriptionParams) (postgres.Subscription, error) {
	args := m.Called(ctx, arg)
	if args.Get(0) == nil {
		return postgres.Subscription{}, args.Error(1)
	}
	return args.Get(0).(postgres.Subscription), args.Error(1)
}

func (m *MockQuerier) CreateUser(ctx context.Context, arg postgres.CreateUserParams) (postgres.User, error) {
	args := m.Called(ctx, arg)
	return args.Get(0).(postgres.User), args.Error(1)
}

func (m *MockQuerier) DecreasePropertyCredits(ctx context.Context, id int32) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockQuerier) IncreasePropertyCredits(ctx context.Context, id int32) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockQuerier) GetInvitationByToken(ctx context.Context, token string) (postgres.LeaseInvitation, error) {
	args := m.Called(ctx, token)
	return args.Get(0).(postgres.LeaseInvitation), args.Error(1)
}

func (m *MockQuerier) GetProperty(ctx context.Context, id int32) (postgres.Property, error) {
	args := m.Called(ctx, id)
	return args.Get(0).(postgres.Property), args.Error(1)
}

func (m *MockQuerier) GetPropertyForUpdate(ctx context.Context, id int32) (postgres.Property, error) {
	args := m.Called(ctx, id)
	return args.Get(0).(postgres.Property), args.Error(1)
}

func (m *MockQuerier) GetSolvencyCheckByID(ctx context.Context, id int32) (postgres.SolvencyCheck, error) {
	args := m.Called(ctx, id)
	return args.Get(0).(postgres.SolvencyCheck), args.Error(1)
}

func (m *MockQuerier) GetSolvencyCheckByToken(ctx context.Context, token pgtype.Text) (postgres.GetSolvencyCheckByTokenRow, error) {
	args := m.Called(ctx, token)
	return args.Get(0).(postgres.GetSolvencyCheckByTokenRow), args.Error(1)
}

func (m *MockQuerier) GetUserByEmail(ctx context.Context, email string) (postgres.User, error) {
	args := m.Called(ctx, email)
	return args.Get(0).(postgres.User), args.Error(1)
}

func (m *MockQuerier) GetUserById(ctx context.Context, id int32) (postgres.User, error) {
	args := m.Called(ctx, id)
	return args.Get(0).(postgres.User), args.Error(1)
}

func (m *MockQuerier) GetUserForUpdate(ctx context.Context, id int32) (postgres.User, error) {
	args := m.Called(ctx, id)
	return args.Get(0).(postgres.User), args.Error(1)
}

func (m *MockQuerier) GetUserCreditBalance(ctx context.Context, userID pgtype.Int4) (int32, error) {
	args := m.Called(ctx, userID)
	return args.Get(0).(int32), args.Error(1)
}

func (m *MockQuerier) GetUserCreditBalanceForUpdate(ctx context.Context, userID pgtype.Int4) (int32, error) {
	args := m.Called(ctx, userID)
	return args.Get(0).(int32), args.Error(1)
}

func (m *MockQuerier) GetUserSubscription(ctx context.Context, userID pgtype.Int4) (postgres.Subscription, error) {
	args := m.Called(ctx, userID)
	return args.Get(0).(postgres.Subscription), args.Error(1)
}

func (m *MockQuerier) HasReceivedInitialBonus(ctx context.Context, userID pgtype.Int4) (bool, error) {
	args := m.Called(ctx, userID)
	return args.Get(0).(bool), args.Error(1)
}

func (m *MockQuerier) ListLeasesByTenant(ctx context.Context, tenantID pgtype.Int4) ([]postgres.ListLeasesByTenantRow, error) {
	args := m.Called(ctx, tenantID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]postgres.ListLeasesByTenantRow), args.Error(1)
}

func (m *MockQuerier) ListPropertiesByOwner(ctx context.Context, ownerID pgtype.Int4) ([]postgres.Property, error) {
	args := m.Called(ctx, ownerID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]postgres.Property), args.Error(1)
}

func (m *MockQuerier) ListSolvencyChecksByOwner(ctx context.Context, initiatorOwnerID pgtype.Int4) ([]postgres.ListSolvencyChecksByOwnerRow, error) {
	args := m.Called(ctx, initiatorOwnerID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]postgres.ListSolvencyChecksByOwnerRow), args.Error(1)
}

func (m *MockQuerier) ListSolvencyChecksByProperty(ctx context.Context, propertyID pgtype.Int4) ([]postgres.ListSolvencyChecksByPropertyRow, error) {
	args := m.Called(ctx, propertyID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]postgres.ListSolvencyChecksByPropertyRow), args.Error(1)
}

func (m *MockQuerier) SoftDeleteProperty(ctx context.Context, arg postgres.SoftDeletePropertyParams) (int32, error) {
	args := m.Called(ctx, arg)
	return args.Get(0).(int32), args.Error(1)
}

func (m *MockQuerier) UpdateInvitationStatus(ctx context.Context, arg postgres.UpdateInvitationStatusParams) error {
	args := m.Called(ctx, arg)
	return args.Error(0)
}

func (m *MockQuerier) UpdateLastContext(ctx context.Context, arg postgres.UpdateLastContextParams) error {
	args := m.Called(ctx, arg)
	return args.Error(0)
}

func (m *MockQuerier) UpdateProperty(ctx context.Context, arg postgres.UpdatePropertyParams) (postgres.Property, error) {
	args := m.Called(ctx, arg)
	return args.Get(0).(postgres.Property), args.Error(1)
}

func (m *MockQuerier) UpdateSolvencyCheckResult(ctx context.Context, arg postgres.UpdateSolvencyCheckResultParams) error {
	args := m.Called(ctx, arg)
	return args.Error(0)
}

func (m *MockQuerier) UpdateSubscriptionLimit(ctx context.Context, arg postgres.UpdateSubscriptionLimitParams) error {
	args := m.Called(ctx, arg)
	return args.Error(0)
}

func (m *MockQuerier) GetLease(ctx context.Context, id int32) (postgres.Lease, error) {
	args := m.Called(ctx, id)
	return args.Get(0).(postgres.Lease), args.Error(1)
}

func (m *MockQuerier) UpdateLeaseContractURL(ctx context.Context, arg postgres.UpdateLeaseContractURLParams) error {
	args := m.Called(ctx, arg)
	return args.Error(0)
}

func (m *MockQuerier) UpdateUserPromotion(ctx context.Context, arg postgres.UpdateUserPromotionParams) error {
	args := m.Called(ctx, arg)
	return args.Error(0)
}

func (m *MockQuerier) CreateDraftLease(ctx context.Context, arg postgres.CreateDraftLeaseParams) (postgres.Lease, error) {
	args := m.Called(ctx, arg)
	return args.Get(0).(postgres.Lease), args.Error(1)
}

func (m *MockQuerier) UpdateLeaseTenant(ctx context.Context, arg postgres.UpdateLeaseTenantParams) error {
	args := m.Called(ctx, arg)
	return args.Error(0)
}

func (m *MockQuerier) CreateInvitationWithLease(ctx context.Context, arg postgres.CreateInvitationWithLeaseParams) (postgres.LeaseInvitation, error) {
	args := m.Called(ctx, arg)
	return args.Get(0).(postgres.LeaseInvitation), args.Error(1)
}

func (m *MockQuerier) GetInvitationByEmailAndProperty(ctx context.Context, arg postgres.GetInvitationByEmailAndPropertyParams) (postgres.LeaseInvitation, error) {
	args := m.Called(ctx, arg)
	return args.Get(0).(postgres.LeaseInvitation), args.Error(1)
}

func (m *MockQuerier) GetInvitationByLeaseID(ctx context.Context, leaseID pgtype.Int4) (postgres.LeaseInvitation, error) {
	args := m.Called(ctx, leaseID)
	return args.Get(0).(postgres.LeaseInvitation), args.Error(1)
}

func (m *MockQuerier) GetLeaseByPropertyAndStatus(ctx context.Context, arg postgres.GetLeaseByPropertyAndStatusParams) (postgres.Lease, error) {
	args := m.Called(ctx, arg)
	return args.Get(0).(postgres.Lease), args.Error(1)
}

type MockLeaseService struct {
	mock.Mock
}

func (m *MockLeaseService) GenerateAndSave(ctx context.Context, leaseID int32, userID int32) error {
	args := m.Called(ctx, leaseID, userID)
	return args.Error(0)
}
