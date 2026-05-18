-- name: InsertGuest :one
INSERT INTO reservations.guests (id, org_id, first_name, last_name, email, phone)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING id, org_id, first_name, last_name, email, phone, created_at, updated_at;

-- name: InsertReservation :one
INSERT INTO reservations.reservations (
    id, org_id, channel_connection_id, property_id, external_id, confirmation_code,
    primary_guest_id, status, check_in, check_out, adults, children,
    currency, total_amount_minor, notes, metadata
) VALUES (
    $1, $2, $3, $4, $5, $6,
    $7, $8, $9, $10, $11, $12,
    $13, $14, $15, $16
)
RETURNING id, org_id, channel_connection_id, property_id, external_id, confirmation_code,
          primary_guest_id, status, check_in, check_out, adults, children,
          currency, total_amount_minor, notes, metadata, booked_at, created_at, updated_at;

-- name: UpdateReservation :one
UPDATE reservations.reservations
   SET status = $2,
       check_in = $3,
       check_out = $4,
       adults = $5,
       children = $6,
       currency = $7,
       total_amount_minor = $8,
       notes = $9,
       metadata = $10,
       updated_at = now()
 WHERE id = $1
RETURNING id, org_id, channel_connection_id, property_id, external_id, confirmation_code,
          primary_guest_id, status, check_in, check_out, adults, children,
          currency, total_amount_minor, notes, metadata, booked_at, created_at, updated_at;

-- name: GetReservationByExternal :one
SELECT id, org_id, channel_connection_id, property_id, external_id, confirmation_code,
       primary_guest_id, status, check_in, check_out, adults, children,
       currency, total_amount_minor, notes, metadata, booked_at, created_at, updated_at
  FROM reservations.reservations
 WHERE channel_connection_id = $1
   AND external_id = $2;

-- name: GetReservation :one
SELECT id, org_id, channel_connection_id, property_id, external_id, confirmation_code,
       primary_guest_id, status, check_in, check_out, adults, children,
       currency, total_amount_minor, notes, metadata, booked_at, created_at, updated_at
  FROM reservations.reservations
 WHERE id = $1;

-- name: ListReservationsByProperty :many
SELECT id, org_id, channel_connection_id, property_id, external_id, confirmation_code,
       primary_guest_id, status, check_in, check_out, adults, children,
       currency, total_amount_minor, notes, metadata, booked_at, created_at, updated_at
  FROM reservations.reservations
 WHERE property_id = $1
 ORDER BY check_in DESC, created_at DESC;
