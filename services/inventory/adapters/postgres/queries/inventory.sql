-- name: ListInventoryDays :many
-- Returns all inventory days for a room type within a date range, ordered by date.
SELECT org_id,
       room_type_id,
       stay_date,
       available,
       sold,
       blocked,
       stop_sell,
       min_stay,
       max_stay,
       cta,
       ctd,
       version,
       updated_at
  FROM inventory.inventory_days
 WHERE room_type_id = @room_type_id
   AND stay_date BETWEEN @from_date AND @to_date
 ORDER BY stay_date;

-- name: UpsertInventoryDay :one
-- Inserts or updates a single inventory day, bumping the optimistic version counter.
INSERT INTO inventory.inventory_days (
    org_id, room_type_id, stay_date,
    available, sold, blocked,
    stop_sell, min_stay, max_stay,
    cta, ctd
) VALUES (
    @org_id, @room_type_id, @stay_date,
    @available, @sold, @blocked,
    @stop_sell, @min_stay, @max_stay,
    @cta, @ctd
)
ON CONFLICT (org_id, room_type_id, stay_date) DO UPDATE SET
    available  = EXCLUDED.available,
    sold       = EXCLUDED.sold,
    blocked    = EXCLUDED.blocked,
    stop_sell  = EXCLUDED.stop_sell,
    min_stay   = EXCLUDED.min_stay,
    max_stay   = EXCLUDED.max_stay,
    cta        = EXCLUDED.cta,
    ctd        = EXCLUDED.ctd,
    version    = inventory_days.version + 1,
    updated_at = now()
RETURNING org_id, room_type_id, stay_date, available, sold, blocked,
          stop_sell, min_stay, max_stay, cta, ctd, version, updated_at;
