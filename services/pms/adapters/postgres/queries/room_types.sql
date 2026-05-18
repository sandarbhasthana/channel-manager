-- name: InsertRoomType :one
INSERT INTO pms.room_types (
    id, org_id, property_id, external_id, code, name, description, capacity, base_occupancy, is_active
) VALUES (
    @id, @org_id, @property_id, @external_id, @code, @name, @description, @capacity, @base_occupancy, @is_active
)
RETURNING id, org_id, property_id, external_id, code, name, description, capacity, base_occupancy, is_active, created_at, updated_at;

-- name: UpdateRoomType :one
UPDATE pms.room_types
   SET external_id = @external_id,
       name = @name,
       description = @description,
       capacity = @capacity,
       base_occupancy = @base_occupancy,
       is_active = @is_active,
       updated_at = now()
 WHERE id = @id
RETURNING id, org_id, property_id, external_id, code, name, description, capacity, base_occupancy, is_active, created_at, updated_at;

-- name: GetRoomTypeByCode :one
SELECT id, org_id, property_id, external_id, code, name, description, capacity, base_occupancy, is_active, created_at, updated_at
  FROM pms.room_types
 WHERE property_id = @property_id
   AND code = @code;

-- name: ListRoomTypesByProperty :many
SELECT id, org_id, property_id, external_id, code, name, description, capacity, base_occupancy, is_active, created_at, updated_at
  FROM pms.room_types
 WHERE property_id = @property_id
   AND is_active = TRUE
 ORDER BY name;

-- name: GetRoomTypeByExternal :one
SELECT id, org_id, property_id, external_id, code, name, description, capacity, base_occupancy, is_active, created_at, updated_at
  FROM pms.room_types
 WHERE property_id = @property_id
   AND external_id = @external_id;
