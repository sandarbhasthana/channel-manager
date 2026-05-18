-- name: CreateConnection :one
INSERT INTO connections
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
FROM connections
WHERE id = $1
LIMIT 1;

-- name: ListConnectionsByOrg :many
SELECT *
FROM connections
WHERE org_id = $1
ORDER BY created_at;

-- name: UpdateConnectionStatus :exec
UPDATE connections
SET status = $2, last_error = $3, updated_at = now()
WHERE id = $1;

-- name: UpdateConnectionName :exec
UPDATE connections
SET name = $2, updated_at = now()
WHERE id = $1;

-- name: UpdateConnectionLastSync :exec
UPDATE connections
SET last_sync_at = $2, updated_at = now()
WHERE id = $1;

-- name: DeleteConnection :exec
DELETE FROM connections WHERE id = $1;

-- name: CreateSyncJob :exec
INSERT INTO sync_jobs
    (
    id, org_id, connection_id, job_type, status, payload, scheduled_at
    )
VALUES
    (
        $1, $2, $3, $4, $5, $6, $7
);

-- name: GetSyncJobByID :one
SELECT *
FROM sync_jobs
WHERE id = $1
LIMIT 1;

-- name: UpdateSyncJobStatus :exec
UPDATE sync_jobs
SET status = $2, result = $3, last_error = $4, finished_at = now(), updated_at = now()
WHERE id = $1;

-- name: ListRecentSyncJobsByConnection :many
SELECT *
  FROM sync_jobs
 WHERE connection_id = $1
 ORDER BY created_at DESC
 LIMIT $2;
