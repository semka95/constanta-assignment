-- name: CreatePayment :one
INSERT INTO payments(
    user_id, email, amount, currency
) VALUES (
    $1, $2, $3, $4
)
RETURNING *;

-- name: UpdatePaymentStatus :execrows
UPDATE payments
SET payment_status = $2,
    updated_at = NOW()
WHERE id = $1 AND payment_status NOT IN ('success', 'failure');

-- name: GetPaymentStatusByID :one
SELECT payment_status FROM payments
WHERE id = $1;

-- name: ListUserPaymentsByID :many
SELECT * FROM payments
WHERE user_id = $1 AND id > $2
LIMIT $3;

-- name: ListUserPaymentsByEmail :many
SELECT * FROM payments
WHERE email = $1 AND id > $2
LIMIT $3;

-- name: DiscardPayment :execrows
DELETE FROM payments 
WHERE id = $1 AND payment_status NOT IN ('success', 'failure') ;