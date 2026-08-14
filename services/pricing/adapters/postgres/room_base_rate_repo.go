package postgres

import (
	"context"
	"fmt"
	"math"

	"github.com/jackc/pgx/v5"

	platformauth "github.com/channel-manager/channel-manager/platform/auth"
	platformdb "github.com/channel-manager/channel-manager/platform/db"
)

// RoomBaseRate is a Channel Manager-stored base nightly rate for a room type,
// used when the live PMS quote is unavailable. The per-channel adjustment is
// applied on top of it. Amount is in major currency units (e.g. dollars).
type RoomBaseRate struct {
	RoomTypeID string  `json:"roomTypeId"`
	Amount     float64 `json:"amount"`
	Currency   string  `json:"currency"`
}

// RoomBaseRateRepository reads and writes CM-stored base rates.
type RoomBaseRateRepository struct {
	pool *platformdb.Pool
}

// NewRoomBaseRateRepository creates a repository backed by pool.
func NewRoomBaseRateRepository(pool *platformdb.Pool) *RoomBaseRateRepository {
	return &RoomBaseRateRepository{pool: pool}
}

// List returns the stored base rates for a property, scoped to the caller's org.
func (r *RoomBaseRateRepository) List(ctx context.Context, propertyID string) ([]RoomBaseRate, error) {
	tc, err := platformauth.FromContext(ctx)
	if err != nil {
		return nil, err
	}
	org := tc.OrgID

	out := make([]RoomBaseRate, 0)
	err = r.pool.WithTenant(ctx, org, func(ctx context.Context, tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			SELECT room_type_id::text, amount_minor, currency
			  FROM pricing.room_base_rates
			 WHERE org_id = $1::uuid AND property_id = $2::uuid
			 ORDER BY room_type_id`, org, propertyID)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var roomTypeID, currency string
			var minor int64
			if err := rows.Scan(&roomTypeID, &minor, &currency); err != nil {
				return err
			}
			out = append(out, RoomBaseRate{
				RoomTypeID: roomTypeID,
				Amount:     float64(minor) / 100,
				Currency:   currency,
			})
		}
		return rows.Err()
	})
	if err != nil {
		return nil, fmt.Errorf("pricing: list room base rates: %w", err)
	}
	return out, nil
}

// Upsert stores the given base rates for a property.
func (r *RoomBaseRateRepository) Upsert(ctx context.Context, propertyID string, rates []RoomBaseRate) error {
	if len(rates) == 0 {
		return nil
	}
	tc, err := platformauth.FromContext(ctx)
	if err != nil {
		return err
	}
	org := tc.OrgID

	return r.pool.WithTenant(ctx, org, func(ctx context.Context, tx pgx.Tx) error {
		for _, br := range rates {
			currency := br.Currency
			if len(currency) != 3 {
				currency = "USD"
			}
			// Round rather than truncate so a cent is never silently lost.
			minor := int64(math.Round(br.Amount * 100))
			if minor < 0 {
				minor = 0
			}
			if _, err := tx.Exec(ctx, `
				INSERT INTO pricing.room_base_rates
					(org_id, property_id, room_type_id, amount_minor, currency)
				VALUES ($1::uuid, $2::uuid, $3::uuid, $4, $5)
				ON CONFLICT (org_id, property_id, room_type_id)
				DO UPDATE SET amount_minor = EXCLUDED.amount_minor, currency = EXCLUDED.currency, updated_at = now()`,
				org, propertyID, br.RoomTypeID, minor, currency); err != nil {
				return fmt.Errorf("pricing: upsert room base rate: %w", err)
			}
		}
		return nil
	})
}
