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
		if p.ConnectionID != "" && ext != "" {
			existing, err := q.GetPropertyByExternal(ctx, pgstore.GetPropertyByExternalParams{
				ConnectionID: connID,
				ExternalID:   extPtr,
			})
			if err == nil {
				row, err := q.UpdateProperty(ctx, pgstore.UpdatePropertyParams{
					ID:       existing.ID,
					Name:     p.Name,
					Timezone: tz,
					Currency: currency,
					Address:  addressJSON(p.City, p.Country),
					IsActive: p.IsActive,
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

func (r *PropertyRepository) GetByExternalID(ctx context.Context, connectionID, externalID string) (domain.Property, error) {
	tc, err := platformauth.FromContext(ctx)
	if err != nil {
		return domain.Property{}, fmt.Errorf("pms/prop_repo: %w", err)
	}
	var result domain.Property
	err = r.pool.WithTenant(ctx, tc.OrgID, func(ctx context.Context, tx pgx.Tx) error {
		ext := externalID
		row, err := pgstore.New(tx).GetPropertyByExternal(ctx, pgstore.GetPropertyByExternalParams{
			ConnectionID: pgtypeUUID(connectionID),
			ExternalID:   &ext,
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
