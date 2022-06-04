-- name: CreateTransfer :one
INSERT INTO transfers(
    user_id, email, amount, currency
) VALUES (
    $1, $2, $3, $4
)
RETURNING *;

-- name: UpdateTransferStatus :execrows
UPDATE transfers
SET transfer_status = $2,
    updated_at = NOW()
WHERE id = $1 AND transfer_status NOT IN ('success', 'failure');

-- name: GetTransferStatusByID :one
SELECT transfer_status FROM transfers
WHERE id = $1;

-- name: ListUserTransfersByID :many
SELECT * FROM transfers
WHERE user_id = $1 AND id > $2
LIMIT $3;

-- name: ListUserTransfersByEmail :many
SELECT * FROM transfers
WHERE email = $1 AND id > $2
LIMIT $3;

-- name: DiscardTransfer :execrows
DELETE FROM transfers 
WHERE id = $1 AND transfer_status NOT IN ('success', 'failure') ;