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

-- name: GetUserByEmail :one
SELECT * FROM users
WHERE email = $1 LIMIT 1;

-- name: GetUserById :one
SELECT * FROM users
WHERE id = $1 LIMIT 1;

-- name: GetProperty :one
SELECT * FROM properties
WHERE id = $1 LIMIT 1;

-- name: CreateProperty :one
INSERT INTO properties (
  owner_id,
  address,
  rental_type,
  details,
  rent_amount,
  deposit_amount
) VALUES (
  $1, $2, $3, $4, $5, $6
)
RETURNING *;

-- name: DecreasePropertyCredits :exec
UPDATE properties
SET vacancy_credits = vacancy_credits - 1
WHERE id = $1 AND vacancy_credits > 0;


-- name: ListPropertiesByOwner :many
SELECT * FROM properties
WHERE owner_id = $1 AND is_active = true
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
    initiator_owner_id, candidate_email, property_id, status
) VALUES (
    $1, $2, $3, 'pending'
)
RETURNING *;

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

-- name: GetUserSubscription :one
SELECT * FROM subscriptions
WHERE user_id = $1 AND status = 'active'
ORDER BY created_at DESC
LIMIT 1;

-- name: GetUserCreditBalance :one
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
