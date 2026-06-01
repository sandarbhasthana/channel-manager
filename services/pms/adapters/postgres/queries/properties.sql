-- name: InsertProperty :one
INSERT INTO pms.properties (
    id, org_id, connection_id, external_id, name, timezone, currency, address, is_active
) VALUES (
    @id, @org_id, @connection_id, @external_id, @name, @timezone, @currency, @address, @is_active
)
RETURNING id, org_id, connection_id, external_id, name, timezone, currency, address, is_active, created_at, updated_at;

-- name: UpdateProperty :one
UPDATE pms.properties
   SET name = @name,
       timezone = @timezone,
       currency = @currency,
       address = @address,
       is_active = @is_active,
       connection_id = @connection_id,
       updated_at = now()
 WHERE id = @id
RETURNING id, org_id, connection_id, external_id, name, timezone, currency, address, is_active, created_at, updated_at;

-- name: GetProperty :one
SELECT id, org_id, connection_id, external_id, name, timezone, currency, address, is_active, created_at, updated_at
  FROM pms.properties
 WHERE id = @id;

-- name: GetPropertyByExternal :one
SELECT id, org_id, connection_id, external_id, name, timezone, currency, address, is_active, created_at, updated_at
  FROM pms.properties
 WHERE org_id = @org_id
   AND external_id = @external_id;

-- name: ListPropertiesByConnection :many
SELECT id, org_id, connection_id, external_id, name, timezone, currency, address, is_active, created_at, updated_at
  FROM pms.properties
 WHERE connection_id = @connection_id
   AND is_active = TRUE
 ORDER BY name;

-- name: ListPropertiesByOrg :many
SELECT id, org_id, connection_id, external_id, name, timezone, currency, address, is_active, created_at, updated_at
  FROM pms.properties
 WHERE org_id = @org_id
   AND is_active = TRUE
 ORDER BY name;
