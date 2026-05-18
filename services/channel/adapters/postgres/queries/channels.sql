-- name: CreateChannel :one
INSERT INTO channels (
    id, org_id, property_id, connection_id, provider, external_property_id, status
) VALUES (
    $1, $2, $3, $4, $5, $6, $7
) RETURNING *;

-- name: GetChannelByID :one
SELECT * FROM channels
WHERE id = $1 LIMIT 1;

-- name: ListChannelsByProperty :many
SELECT * FROM channels
WHERE property_id = $1
ORDER BY created_at;

-- name: UpdateChannelStatus :exec
UPDATE channels
SET status = $2, last_error = $3, updated_at = now()
WHERE id = $1;

-- name: UpdateChannelLastSync :exec
UPDATE channels
SET last_sync_at = $2, updated_at = now()
WHERE id = $1;

-- name: DeleteChannel :exec
DELETE FROM channels WHERE id = $1;
