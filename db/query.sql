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
