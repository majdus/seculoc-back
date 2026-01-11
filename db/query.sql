-- name: CreateUser :one
INSERT INTO users (
  email,
  password_hash,
  first_name,
  last_name,
  phone_number
) VALUES (
  $1, $2, $3, $4, $5
)
RETURNING *;

-- name: GetUserByEmail :one
SELECT * FROM users
WHERE email = $1 LIMIT 1;

-- name: GetUserById :one
SELECT * FROM users
WHERE id = $1 LIMIT 1;

-- name: CreateProperty :one
INSERT INTO properties (
  owner_id,
  address,
  rental_type,
  details
) VALUES (
  $1, $2, $3, $4
)
RETURNING *;

-- name: ListPropertiesByOwner :many
SELECT * FROM properties
WHERE owner_id = $1
ORDER BY created_at DESC;

-- name: CountPropertiesByOwner :one
SELECT COUNT(*) FROM properties
WHERE owner_id = $1 AND is_active = true;

-- name: CountPropertiesByOwnerAndType :one
SELECT COUNT(*) FROM properties
WHERE owner_id = $1 AND rental_type = $2 AND is_active = true;

-- name: UpdateSubscriptionLimit :exec
UPDATE subscriptions
SET max_properties_limit = max_properties_limit + $2
WHERE user_id = $1 AND status = 'active';

-- name: CreateSolvencyCheck :one
INSERT INTO solvency_checks (
    user_id, status
) VALUES (
    $1, 'pending'
)
RETURNING id, user_id, status, created_at;

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
