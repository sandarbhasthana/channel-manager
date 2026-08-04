package postgres

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	platformauth "github.com/channel-manager/channel-manager/platform/auth"
	platformdb "github.com/channel-manager/channel-manager/platform/db"
	"github.com/channel-manager/channel-manager/services/pms/adapters/postgres/pgstore"
	"github.com/channel-manager/channel-manager/services/pms/domain"
)

// PropertyRepository implements ports.PropertyRepository.
type PropertyRepository struct {
	pool *platformdb.Pool
}

// NewPropertyRepository creates a new repository.
func NewPropertyRepository(pool *platformdb.Pool) *PropertyRepository {
	return &PropertyRepository{pool: pool}
}

func (r *PropertyRepository) Upsert(ctx context.Context, p domain.Property) (domain.Property, error) {
	tc, err := platformauth.FromContext(ctx)
	if err != nil {
		return domain.Property{}, fmt.Errorf("pms/prop_repo: %w", err)
	}
	orgID, err := uuid.Parse(tc.OrgID)
	if err != nil {
		return domain.Property{}, fmt.Errorf("pms/prop_repo: invalid org_id: %w", err)
	}
	connID := pgtypeUUID(p.ConnectionID)
	ext := p.ExternalID
	var extPtr *string
	if ext != "" {
		extPtr = &ext
	}
	currency := p.DefaultCurrency
	if currency == "" {
		currency = "USD"
	}
	tz := p.Timezone
	if tz == "" {
		tz = "UTC"
	}

	var result domain.Property
	err = r.pool.WithTenant(ctx, tc.OrgID, func(ctx context.Context, tx pgx.Tx) error {
		q := pgstore.New(tx)
		if ext != "" {
			existing, err := q.GetPropertyByExternal(ctx, pgstore.GetPropertyByExternalParams{
				OrgID:      orgID,
				ExternalID: extPtr,
			})
			if err == nil {
				row, err := q.UpdateProperty(ctx, pgstore.UpdatePropertyParams{
					ID:           existing.ID,
					Name:         p.Name,
					Timezone:     tz,
					Currency:     currency,
					Address:      addressJSON(p.City, p.Country),
					IsActive:     p.IsActive,
					ConnectionID: connID,
				})
				if err != nil {
					return err
				}
				result = propertyToDomain(row)
				return nil
			}
		}
		id := uuid.New()
		if p.ID != "" {
			if parsed, err := uuid.Parse(p.ID); err == nil {
				id = parsed
			}
		}
		row, err := q.InsertProperty(ctx, pgstore.InsertPropertyParams{
			ID:           id,
			OrgID:        orgID,
			ConnectionID: connID,
			ExternalID:   extPtr,
			Name:         p.Name,
			Timezone:     tz,
			Currency:     currency,
			Address:      addressJSON(p.City, p.Country),
			IsActive:     p.IsActive,
		})
		if err != nil {
			return err
		}
		result = propertyToDomain(row)
		return nil
	})
	if err != nil {
		return domain.Property{}, fmt.Errorf("pms/prop_repo: upsert: %w", err)
	}
	return result, nil
}

func (r *PropertyRepository) GetByID(ctx context.Context, id string) (domain.Property, error) {
	tc, err := platformauth.FromContext(ctx)
	if err != nil {
		return domain.Property{}, fmt.Errorf("pms/prop_repo: %w", err)
	}
	pid, err := uuid.Parse(id)
	if err != nil {
		return domain.Property{}, fmt.Errorf("pms/prop_repo: invalid id: %w", err)
	}
	var result domain.Property
	err = r.pool.WithTenant(ctx, tc.OrgID, func(ctx context.Context, tx pgx.Tx) error {
		row, err := pgstore.New(tx).GetProperty(ctx, pid)
		if err != nil {
			return err
		}
		result = propertyToDomain(row)
		return nil
	})
	if err != nil {
		return domain.Property{}, fmt.Errorf("pms/prop_repo: get: %w", err)
	}
	return result, nil
}

// BookingEngineEnabled reports whether the direct sales channel is on for a
// property. Hand-written rather than added to the sqlc query set: it is a
// single boolean read the storefront needs, and threading a new column through
// the generated Property model would touch far more than this.
func (r *PropertyRepository) BookingEngineEnabled(ctx context.Context, id string) (bool, error) {
	tc, err := platformauth.FromContext(ctx)
	if err != nil {
		return false, fmt.Errorf("pms/prop_repo: %w", err)
	}
	var enabled bool
	err = r.pool.WithTenant(ctx, tc.OrgID, func(ctx context.Context, tx pgx.Tx) error {
		return tx.QueryRow(ctx, `
			SELECT booking_engine_enabled FROM pms.properties
			 WHERE org_id = $1::uuid AND id = $2::uuid`,
			tc.OrgID, id).Scan(&enabled)
	})
	if err != nil {
		return false, fmt.Errorf("pms/prop_repo: booking engine enabled: %w", err)
	}
	return enabled, nil
}

// GetChannelConfig reads a property's booking-engine routing config. The
// storefront exposes this to the booking engine (which reads route/percent to
// decide where to send its stay actions). Hand-written for the same reason as
// BookingEngineEnabled.
func (r *PropertyRepository) GetChannelConfig(ctx context.Context, id string) (domain.ChannelConfig, error) {
	tc, err := platformauth.FromContext(ctx)
	if err != nil {
		return domain.ChannelConfig{}, fmt.Errorf("pms/prop_repo: %w", err)
	}
	var cfg domain.ChannelConfig
	err = r.pool.WithTenant(ctx, tc.OrgID, func(ctx context.Context, tx pgx.Tx) error {
		return tx.QueryRow(ctx, `
			SELECT booking_engine_enabled, booking_route, booking_route_percent
			  FROM pms.properties
			 WHERE org_id = $1::uuid AND id = $2::uuid`,
			tc.OrgID, id).Scan(&cfg.Enabled, &cfg.Route, &cfg.Percent)
	})
	if err != nil {
		return domain.ChannelConfig{}, fmt.Errorf("pms/prop_repo: get channel config: %w", err)
	}
	return cfg, nil
}

// ListListings returns every active property in the caller's org with its
// booking-engine configuration attached. This is what the storefront serves to
// a direct booking engine, which needs the whole set in one read: it has to
// choose a property before it can ask about any particular one, so a
// per-property config call cannot bootstrap it.
//
// Inactive properties are excluded — a booking engine has no use for a property
// it cannot sell.
func (r *PropertyRepository) ListListings(ctx context.Context) ([]domain.PropertyListing, error) {
	tc, err := platformauth.FromContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("pms/prop_repo: %w", err)
	}
	var out []domain.PropertyListing
	err = r.pool.WithTenant(ctx, tc.OrgID, func(ctx context.Context, tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			SELECT id, COALESCE(external_id, ''), name, timezone, currency,
			       is_active, is_default,
			       booking_engine_enabled, booking_route, booking_route_percent
			  FROM pms.properties
			 WHERE org_id = $1::uuid AND is_active
			 ORDER BY is_default DESC, name`,
			tc.OrgID)
		if err != nil {
			return err
		}
		defer rows.Close()
		out = []domain.PropertyListing{}
		for rows.Next() {
			var p domain.PropertyListing
			if err := rows.Scan(
				&p.ID, &p.ExternalID, &p.Name, &p.Timezone, &p.DefaultCurrency,
				&p.IsActive, &p.IsDefault,
				&p.Channel.Enabled, &p.Channel.Route, &p.Channel.Percent,
			); err != nil {
				return err
			}
			out = append(out, p)
		}
		return rows.Err()
	})
	if err != nil {
		return nil, fmt.Errorf("pms/prop_repo: list listings: %w", err)
	}
	return out, nil
}

// SetDefault promotes one property to the org's default, demoting the incumbent
// in the same transaction. The properties_default_uniq partial index allows only
// one default per org, so clearing and setting cannot be split into two
// statements outside a transaction without risking a constraint violation.
//
// Setting a property that is already the default is a no-op rather than an
// error, which keeps the dashboard's star button idempotent.
func (r *PropertyRepository) SetDefault(ctx context.Context, id string) error {
	tc, err := platformauth.FromContext(ctx)
	if err != nil {
		return fmt.Errorf("pms/prop_repo: %w", err)
	}
	if _, err := uuid.Parse(id); err != nil {
		return fmt.Errorf("pms/prop_repo: invalid id: %w", err)
	}
	err = r.pool.WithTenant(ctx, tc.OrgID, func(ctx context.Context, tx pgx.Tx) error {
		// Demote first: the unique index is checked per-statement, so setting the
		// new default before clearing the old one would collide.
		if _, err := tx.Exec(ctx, `
			UPDATE pms.properties SET is_default = FALSE
			 WHERE org_id = $1::uuid AND is_default AND id <> $2::uuid`,
			tc.OrgID, id); err != nil {
			return err
		}
		tag, err := tx.Exec(ctx, `
			UPDATE pms.properties SET is_default = TRUE
			 WHERE org_id = $1::uuid AND id = $2::uuid AND is_active`,
			tc.OrgID, id)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return fmt.Errorf("property not found or inactive")
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("pms/prop_repo: set default: %w", err)
	}
	return nil
}

func (r *PropertyRepository) GetByExternalID(ctx context.Context, connectionID, externalID string) (domain.Property, error) {
	tc, err := platformauth.FromContext(ctx)
	if err != nil {
		return domain.Property{}, fmt.Errorf("pms/prop_repo: %w", err)
	}
	var result domain.Property
	err = r.pool.WithTenant(ctx, tc.OrgID, func(ctx context.Context, tx pgx.Tx) error {
		ext := externalID
		orgID, _ := uuid.Parse(tc.OrgID)
		row, err := pgstore.New(tx).GetPropertyByExternal(ctx, pgstore.GetPropertyByExternalParams{
			OrgID:      orgID,
			ExternalID: &ext,
		})
		if err != nil {
			return err
		}
		result = propertyToDomain(row)
		return nil
	})
	if err != nil {
		return domain.Property{}, fmt.Errorf("pms/prop_repo: get external: %w", err)
	}
	return result, nil
}

func (r *PropertyRepository) ListByConnection(ctx context.Context, connectionID string) ([]domain.Property, error) {
	tc, err := platformauth.FromContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("pms/prop_repo: %w", err)
	}
	var out []domain.Property
	err = r.pool.WithTenant(ctx, tc.OrgID, func(ctx context.Context, tx pgx.Tx) error {
		rows, err := pgstore.New(tx).ListPropertiesByConnection(ctx, pgtypeUUID(connectionID))
		if err != nil {
			return err
		}
		out = make([]domain.Property, 0, len(rows))
		for _, row := range rows {
			out = append(out, propertyToDomain(row))
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("pms/prop_repo: list by connection: %w", err)
	}
	return out, nil
}

func (r *PropertyRepository) ListByOrg(ctx context.Context, orgID string) ([]domain.Property, error) {
	tc, err := platformauth.FromContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("pms/prop_repo: %w", err)
	}
	oid, err := uuid.Parse(orgID)
	if err != nil {
		return nil, fmt.Errorf("pms/prop_repo: invalid org_id: %w", err)
	}
	var out []domain.Property
	err = r.pool.WithTenant(ctx, tc.OrgID, func(ctx context.Context, tx pgx.Tx) error {
		rows, err := pgstore.New(tx).ListPropertiesByOrg(ctx, oid)
		if err != nil {
			return err
		}
		out = make([]domain.Property, 0, len(rows))
		for _, row := range rows {
			out = append(out, propertyToDomain(row))
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("pms/prop_repo: list: %w", err)
	}
	return out, nil
}
