-- name: CreateUser :one
INSERT INTO users (
  email,
  password_hash,
  first_name,
  last_name,
  phone_number,
  last_context_used
) VALUES (
  $1, $2, $3, $4, $5, $6
)
RETURNING *;

-- name: UpdateLastContext :exec
UPDATE users
SET last_context_used = $2
WHERE id = $1;

-- name: UpdateUserPromotion :exec
UPDATE users
SET password_hash = $2,
    first_name = $3,
    last_name = $4,
    phone_number = $5,
    is_provisional = FALSE
WHERE id = $1;

-- name: GetUserByEmail :one
SELECT * FROM users
WHERE email = $1 LIMIT 1;

-- name: GetUserById :one
SELECT * FROM users
WHERE id = $1 LIMIT 1;

-- name: GetProperty :one
SELECT id, owner_id, name, address, rental_type, details, rent_amount, deposit_amount, vacancy_credits, is_active, created_at FROM properties
WHERE id = $1 LIMIT 1;

-- name: CreateProperty :one
INSERT INTO properties (
  owner_id, name, address, rental_type, details, rent_amount, deposit_amount
) VALUES (
  $1, $2, $3, $4, $5, $6, $7
)
RETURNING id, owner_id, name, address, rental_type, details, rent_amount, deposit_amount, vacancy_credits, is_active, created_at;

-- name: UpdateProperty :one
UPDATE properties
SET 
  name = COALESCE(NULLIF($3, ''), name),
  address = COALESCE(NULLIF($4, ''), address),
  rental_type = COALESCE(NULLIF($5, '')::property_type, rental_type),
  details = COALESCE($6, details),
  rent_amount = COALESCE($7, rent_amount),
  deposit_amount = COALESCE($8, deposit_amount)
WHERE id = $1 AND owner_id = $2
RETURNING id, owner_id, name, address, rental_type, details, rent_amount, deposit_amount, vacancy_credits, is_active, created_at;

-- name: DecreasePropertyCredits :exec
UPDATE properties
SET vacancy_credits = vacancy_credits - 1
WHERE id = $1 AND vacancy_credits > 0;


-- name: ListPropertiesByOwner :many
SELECT id, owner_id, name, address, rental_type, details, rent_amount, deposit_amount, vacancy_credits, is_active, created_at FROM properties
WHERE owner_id = $1
ORDER BY created_at DESC;

-- name: CountPropertiesByOwner :one
SELECT COUNT(*) FROM properties
WHERE owner_id = $1 AND is_active = true;

-- name: CountPropertiesByOwnerAndType :one
SELECT COUNT(*) FROM properties
WHERE owner_id = $1 AND rental_type = $2 AND is_active = true;

-- name: SoftDeleteProperty :one
UPDATE properties
SET is_active = false
WHERE id = $1 AND owner_id = $2 AND is_active = true
RETURNING id;

-- name: UpdateSubscriptionLimit :exec
UPDATE subscriptions
SET max_properties_limit = max_properties_limit + $2
WHERE user_id = $1 AND status = 'active';

-- name: CreateSolvencyCheck :one
INSERT INTO solvency_checks (
    initiator_owner_id, candidate_id, token, property_id, status, credit_source
) VALUES (
    $1, $2, $3, $4, 'pending', $5
)
RETURNING *;

-- name: GetSolvencyCheckByID :one
SELECT * FROM solvency_checks
WHERE id = $1;

-- name: CancelSolvencyCheck :exec
UPDATE solvency_checks
SET status = 'cancelled'
WHERE id = $1;

-- name: GetSolvencyCheckByToken :one
SELECT 
    sc.id, sc.initiator_owner_id, sc.candidate_id, sc.token, sc.property_id, sc.status, sc.created_at,
    u.email as candidate_email, u.first_name as candidate_first_name, u.last_name as candidate_last_name,
    p.address as property_address, p.rent_amount as property_rent_amount, p.name as property_name
FROM solvency_checks sc
JOIN users u ON sc.candidate_id = u.id
JOIN properties p ON sc.property_id = p.id
WHERE sc.token = $1
 LIMIT 1;

-- name: UpdateSolvencyCheckResult :exec
UPDATE solvency_checks
SET status = $2, score_result = $3, report_url = $4
WHERE id = $1;

-- name: ListSolvencyChecksByOwner :many
SELECT sc.*, u.email as candidate_email, u.first_name as candidate_first_name, u.last_name as candidate_last_name, p.address as property_address
FROM solvency_checks sc
JOIN users u ON sc.candidate_id = u.id
JOIN properties p ON sc.property_id = p.id
WHERE sc.initiator_owner_id = $1
ORDER BY sc.created_at DESC;

-- name: ListSolvencyChecksByProperty :many
SELECT sc.*, u.email as candidate_email, u.first_name as candidate_first_name, u.last_name as candidate_last_name
FROM solvency_checks sc
JOIN users u ON sc.candidate_id = u.id
WHERE sc.property_id = $1
ORDER BY sc.created_at DESC;

-- name: CleanupProvisionalUsers :exec
DELETE FROM users 
WHERE is_provisional = TRUE 
AND created_at < NOW() - INTERVAL '30 days';

-- name: CreateSubscription :one
INSERT INTO subscriptions (
    user_id, plan_type, frequency, start_date, end_date, max_properties_limit
) VALUES (
    $1, $2, $3, $4, $5, $6
)
RETURNING *;

-- name: CreateCreditTransaction :one
INSERT INTO credit_transactions (
    user_id, amount, transaction_type, description
) VALUES (
    $1, $2, $3, $4
)
RETURNING *;

-- name: IncreasePropertyCredits :exec
UPDATE properties
SET vacancy_credits = vacancy_credits + 1
WHERE id = $1;

-- name: GetUserSubscription :one
SELECT * FROM subscriptions
WHERE user_id = $1 AND status = 'active'
ORDER BY created_at DESC
LIMIT 1;

-- name: GetUserCreditBalance :one
SELECT current_balance::int FROM view_user_credit_balance
WHERE user_id = $1;

-- name: GetPropertyForUpdate :one
SELECT * FROM properties
WHERE id = $1 FOR UPDATE;

-- name: GetUserForUpdate :one
SELECT * FROM users
WHERE id = $1 FOR UPDATE;

-- name: GetUserCreditBalanceForUpdate :one
SELECT current_balance::int FROM view_user_credit_balance
WHERE user_id = $1;

-- name: HasReceivedInitialBonus :one
SELECT EXISTS(
    SELECT 1 FROM credit_transactions
    WHERE user_id = $1 AND transaction_type = 'initial_free'
);

-- name: CreateInvitation :one
INSERT INTO lease_invitations (property_id, owner_id, tenant_email, token, expires_at)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: GetInvitationByToken :one
SELECT * FROM lease_invitations
WHERE token = $1 LIMIT 1;

-- name: UpdateInvitationStatus :exec
UPDATE lease_invitations
SET status = $2
WHERE id = $1;

-- name: CountLeasesByTenant :one
SELECT COUNT(*) FROM leases
WHERE tenant_id = $1 AND lease_status != 'terminated';

-- name: CountBookingsByTenant :one
SELECT COUNT(*) FROM seasonal_bookings
WHERE tenant_id = $1 AND booking_status = 'confirmed';

-- name: CreateLease :one
INSERT INTO leases (
    property_id, tenant_id, start_date, rent_amount, deposit_amount, lease_status
) VALUES (
    $1, $2, $3, $4, $5, 'draft'
)
RETURNING *;

-- name: ListLeasesByTenant :many
SELECT 
    l.id, l.property_id, l.tenant_id, l.start_date, l.end_date, l.rent_amount, l.deposit_amount, l.lease_status, l.contract_url, l.created_at,
    p.address as property_address, p.rental_type
FROM leases l
JOIN properties p ON l.property_id = p.id
WHERE l.tenant_id = $1
ORDER BY l.created_at DESC;
