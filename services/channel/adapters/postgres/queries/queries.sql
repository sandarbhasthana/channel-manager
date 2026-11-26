-- name: CreateConnection :one
INSERT INTO channel.connections
    (
    id, org_id, provider, name, status, secret_ref, config
    )
VALUES
    (
        $1, $2, $3, $4, $5, $6, $7
)
RETURNING *;

-- name: GetConnectionByID :one
SELECT *
FROM channel.connections
WHERE id = $1
LIMIT 1;

-- name: ListConnectionsByOrg :many
SELECT *
FROM channel.connections
WHERE org_id = $1
ORDER BY created_at;

-- name: UpdateConnectionStatus :exec
UPDATE channel.connections
SET status = $2, last_error = $3, updated_at = now()
WHERE id = $1;

-- name: UpdateConnectionName :exec
UPDATE channel.connections
SET name = $2, updated_at = now()
WHERE id = $1;

-- name: UpdateConnectionLastSync :exec
UPDATE channel.connections
SET last_sync_at = $2, updated_at = now()
WHERE id = $1;

-- name: DeleteConnection :exec
DELETE FROM channel.connections WHERE id = $1;

-- name: CreateSyncJob :exec
INSERT INTO channel.sync_jobs
    (
    id, org_id, connection_id, job_type, status, payload, scheduled_at
    )
VALUES
    (
        $1, $2, $3, $4, $5, $6, $7
);

-- name: GetSyncJobByID :one
SELECT *
FROM channel.sync_jobs
WHERE id = $1
LIMIT 1;

-- name: UpdateSyncJobStatus :exec
UPDATE channel.sync_jobs
SET status = $2, result = $3, last_error = $4, finished_at = now(), updated_at = now()
WHERE id = $1;

-- name: ListRecentSyncJobsByConnection :many
SELECT *
  FROM channel.sync_jobs
 WHERE connection_id = $1
 ORDER BY created_at DESC
 LIMIT $2;
