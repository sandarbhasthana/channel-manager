// Package postgres implements the booking-engine repository against the
// reservations and pms schemas. Queries are hand-written pgx (the booking
// engine is a read model over other services' tables), run inside WithTenant so
// RLS confines every row to the caller's org. org_id is also named explicitly
// in each predicate as defence in depth, so a role exempt from RLS degrades to
// "no rows" rather than another tenant's rows.
package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	platformauth "github.com/channel-manager/channel-manager/platform/auth"
	platformdb "github.com/channel-manager/channel-manager/platform/db"
	"github.com/channel-manager/channel-manager/services/bookingengine/domain"
	"github.com/channel-manager/channel-manager/services/bookingengine/ports"
)

// Repository reads direct reservations and booking-engine settings.
type Repository struct {
	pool *platformdb.Pool
}

// NewRepository creates a booking-engine repository backed by pool.
func NewRepository(pool *platformdb.Pool) *Repository {
	return &Repository{pool: pool}
}

var _ ports.Repository = (*Repository)(nil)

func orgID(ctx context.Context) (string, error) {
	tc, err := platformauth.FromContext(ctx)
	if err != nil {
		return "", err
	}
	return tc.OrgID, nil
}

// ListDirectReservations selects storefront-origin bookings for a property.
// "Direct" is metadata->>'source' = 'direct', the marker the storefront writes.
func (r *Repository) ListDirectReservations(ctx context.Context, propertyID string, limit, offset int) ([]domain.DirectReservation, error) {
	org, err := orgID(ctx)
	if err != nil {
		return nil, err
	}

	var out []domain.DirectReservation
	err = r.pool.WithTenant(ctx, org, func(ctx context.Context, tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			SELECT r.id::text, r.property_id::text, COALESCE(r.confirmation_code, ''),
			       TRIM(CONCAT(COALESCE(g.first_name, ''), ' ', COALESCE(g.last_name, ''))),
			       r.status, r.check_in, r.check_out,
			       r.total_amount_minor, r.currency, r.booked_at
			  FROM reservations.reservations r
			  LEFT JOIN reservations.guests g ON g.id = r.primary_guest_id
			 WHERE r.org_id = $1::uuid
			   AND r.property_id = $2::uuid
			   AND r.metadata->>'source' = $3
			 ORDER BY r.booked_at DESC, r.id
			 LIMIT $4 OFFSET $5`,
			org, propertyID, domain.Source, limit, offset)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var d domain.DirectReservation
			if err := rows.Scan(&d.ID, &d.PropertyID, &d.ConfirmationCode, &d.GuestName,
				&d.Status, &d.CheckIn, &d.CheckOut, &d.TotalMinor, &d.Currency, &d.BookedAt); err != nil {
				return err
			}
			out = append(out, d)
		}
		return rows.Err()
	})
	if err != nil {
		return nil, fmt.Errorf("bookingengine: list direct reservations: %w", err)
	}
	return out, nil
}

// GetSettings reads the property's booking-engine flag.
func (r *Repository) GetSettings(ctx context.Context, propertyID string) (domain.Settings, error) {
	org, err := orgID(ctx)
	if err != nil {
		return domain.Settings{}, err
	}

	s := domain.Settings{PropertyID: propertyID}
	err = r.pool.WithTenant(ctx, org, func(ctx context.Context, tx pgx.Tx) error {
		return tx.QueryRow(ctx, `
			SELECT booking_engine_enabled, booking_route, booking_route_percent
			  FROM pms.properties
			 WHERE org_id = $1::uuid AND id = $2::uuid`,
			org, propertyID).Scan(&s.DirectChannelEnabled, &s.Route, &s.Percent)
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Settings{}, domain.ErrPropertyNotFound
	}
	if err != nil {
		return domain.Settings{}, fmt.Errorf("bookingengine: get settings: %w", err)
	}
	return s, nil
}

// UpdateSettings writes the booking-engine configuration for a property — the
// direct-channel switch and the booking route/percentage — and returns it as
// persisted. Managed from the CM dashboard.
func (r *Repository) UpdateSettings(ctx context.Context, in domain.Settings) (domain.Settings, error) {
	org, err := orgID(ctx)
	if err != nil {
		return domain.Settings{}, err
	}

	s := domain.Settings{PropertyID: in.PropertyID}
	err = r.pool.WithTenant(ctx, org, func(ctx context.Context, tx pgx.Tx) error {
		return tx.QueryRow(ctx, `
			UPDATE pms.properties
			   SET booking_engine_enabled = $3,
			       booking_route          = $4,
			       booking_route_percent  = $5
			 WHERE org_id = $1::uuid AND id = $2::uuid
			 RETURNING booking_engine_enabled, booking_route, booking_route_percent`,
			org, in.PropertyID, in.DirectChannelEnabled, in.Route, in.Percent).
			Scan(&s.DirectChannelEnabled, &s.Route, &s.Percent)
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Settings{}, domain.ErrPropertyNotFound
	}
	if err != nil {
		return domain.Settings{}, fmt.Errorf("bookingengine: update settings: %w", err)
	}
	return s, nil
}
