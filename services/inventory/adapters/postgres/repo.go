// Package postgres implements ports.InventoryRepository backed by Postgres.
// Every operation runs inside a db.Pool.WithTenant transaction so that the
// RLS policy (org_id::text = current_setting('app.current_org_id', true))
// is always satisfied.
package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	platformauth "github.com/channel-manager/channel-manager/platform/auth"
	platformdb "github.com/channel-manager/channel-manager/platform/db"
	"github.com/channel-manager/channel-manager/services/inventory/adapters/postgres/pgstore"
	"github.com/channel-manager/channel-manager/services/inventory/domain"
)

// Repository implements ports.InventoryRepository.
type Repository struct {
	pool *platformdb.Pool
}

// NewRepository creates a new Postgres-backed inventory repository.
func NewRepository(pool *platformdb.Pool) *Repository {
	return &Repository{pool: pool}
}

// ListByRange returns all inventory days for a room-type within [from, to] inclusive.
// The org_id is resolved from the TenantContext in ctx.
func (r *Repository) ListByRange(ctx context.Context, roomTypeID string, from, to time.Time) ([]domain.InventoryDay, error) {
	tc, err := platformauth.FromContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("inventory/repo: %w", err)
	}

	rtID, err := uuid.Parse(roomTypeID)
	if err != nil {
		return nil, fmt.Errorf("inventory/repo: invalid room_type_id: %w", err)
	}

	var days []domain.InventoryDay
	txErr := r.pool.WithTenant(ctx, tc.OrgID, func(ctx context.Context, tx pgx.Tx) error {
		q := pgstore.New(tx)
		rows, err := q.ListInventoryDays(ctx, pgstore.ListInventoryDaysParams{
			RoomTypeID: rtID,
			FromDate:   from,
			ToDate:     to,
		})
		if err != nil {
			return err
		}
		days = make([]domain.InventoryDay, 0, len(rows))
		for _, row := range rows {
			days = append(days, toDomain(row))
		}
		return nil
	})
	if txErr != nil {
		return nil, fmt.Errorf("inventory/repo: list: %w", txErr)
	}
	return days, nil
}

// UpsertBatch writes a slice of inventory days inside a single tenant-scoped
// transaction. All days must belong to the org resolved from ctx.
func (r *Repository) UpsertBatch(ctx context.Context, days []domain.InventoryDay) error {
	tc, err := platformauth.FromContext(ctx)
	if err != nil {
		return fmt.Errorf("inventory/repo: %w", err)
	}

	orgID, err := uuid.Parse(tc.OrgID)
	if err != nil {
		return fmt.Errorf("inventory/repo: invalid org_id: %w", err)
	}

	return r.pool.WithTenant(ctx, tc.OrgID, func(ctx context.Context, tx pgx.Tx) error {
		q := pgstore.New(tx)
		for _, day := range days {
			rtID, err := uuid.Parse(day.RoomTypeID)
			if err != nil {
				return fmt.Errorf("inventory/repo: invalid room_type_id %q: %w", day.RoomTypeID, err)
			}
			_, err = q.UpsertInventoryDay(ctx, pgstore.UpsertInventoryDayParams{
				OrgID:      orgID,
				RoomTypeID: rtID,
				StayDate:   day.StayDate,
				Available:  int32(day.Available), //nolint:gosec
				Sold:       int32(day.Sold),       //nolint:gosec
				Blocked:    int32(day.Blocked),    //nolint:gosec
				StopSell:   day.StopSell,
				MinStay:    day.MinStay,
				MaxStay:    day.MaxStay,
				Cta:        day.CTA,
				Ctd:        day.CTD,
			})
			if err != nil {
				return fmt.Errorf("inventory/repo: upsert day %s: %w", day.StayDate.Format("2006-01-02"), err)
			}
		}
		return nil
	})
}

// toDomain maps a pgstore list row to the domain model.
func toDomain(row pgstore.ListInventoryDaysRow) domain.InventoryDay {
	return domain.InventoryDay{
		OrgID:      row.OrgID.String(),
		RoomTypeID: row.RoomTypeID.String(),
		StayDate:   row.StayDate,
		Available:  int(row.Available),
		Sold:       int(row.Sold),
		Blocked:    int(row.Blocked),
		StopSell:   row.StopSell,
		MinStay:    row.MinStay,
		MaxStay:    row.MaxStay,
		CTA:        row.Cta,
		CTD:        row.Ctd,
		Version:    row.Version,
		UpdatedAt:  row.UpdatedAt,
	}
}
