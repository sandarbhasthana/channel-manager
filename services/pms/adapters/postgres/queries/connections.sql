-- name: CreateConnection :one
INSERT INTO pms.connections (
    id, org_id, provider, name, status, secret_ref, config
) VALUES (
    @id, @org_id, @provider, @name, @status, @secret_ref, @config
)
RETURNING id, org_id, provider, name, status, secret_ref, config, last_sync_at, last_error, created_at, updated_at;

-- name: GetConnection :one
SELECT id, org_id, provider, name, status, secret_ref, config, last_sync_at, last_error, created_at, updated_at
  FROM pms.connections
 WHERE id = @id;

-- name: ListConnectionsByOrg :many
SELECT id, org_id, provider, name, status, secret_ref, config, last_sync_at, last_error, created_at, updated_at
  FROM pms.connections
 WHERE org_id = @org_id
 ORDER BY created_at DESC;

-- name: UpdateConnectionStatus :exec
UPDATE pms.connections
   SET status = @status,
       last_error = @last_error,
       updated_at = now()
 WHERE id = @id;

-- name: UpdateConnectionLastSync :exec
UPDATE pms.connections
   SET last_sync_at = @last_sync_at,
       last_error = NULL,
       updated_at = now()
 WHERE id = @id;

-- name: DeleteConnection :exec
DELETE FROM pms.connections WHERE id = @id;
