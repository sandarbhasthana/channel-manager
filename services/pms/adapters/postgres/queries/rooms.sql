-- name: UpsertRoom :one
INSERT INTO pms.rooms (
    org_id, property_id, room_type_id, external_id, name, is_active
) VALUES (
    $1, $2, $3, $4, $5, $6
)
ON CONFLICT (org_id, property_id, external_id) DO UPDATE SET
    room_type_id = EXCLUDED.room_type_id,
    name = EXCLUDED.name,
    is_active = EXCLUDED.is_active,
    updated_at = now()
RETURNING *;

-- name: ListRoomsByProperty :many
SELECT * FROM pms.rooms
WHERE property_id = $1 AND is_active = TRUE
ORDER BY name ASC;

-- name: ListRoomsByRoomType :many
SELECT * FROM pms.rooms
WHERE room_type_id = $1 AND is_active = TRUE
ORDER BY name ASC;
