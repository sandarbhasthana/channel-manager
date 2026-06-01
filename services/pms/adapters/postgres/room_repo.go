package postgres

import (
	"context"
	"fmt"

	platformauth "github.com/channel-manager/channel-manager/platform/auth"
	platformdb "github.com/channel-manager/channel-manager/platform/db"
	"github.com/channel-manager/channel-manager/services/pms/adapters/postgres/pgstore"
	"github.com/channel-manager/channel-manager/services/pms/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// RoomRepository implements ports.RoomRepository.
type RoomRepository struct {
	pool *platformdb.Pool
}

// NewRoomRepository returns a new repository.
func NewRoomRepository(pool *platformdb.Pool) *RoomRepository {
	return &RoomRepository{pool: pool}
}

func (r *RoomRepository) Upsert(ctx context.Context, rm domain.Room) (domain.Room, error) {
	orgID, err := uuid.Parse(rm.OrgID)
	if err != nil {
		return domain.Room{}, fmt.Errorf("invalid org ID: %w", err)
	}
	propertyID, err := uuid.Parse(rm.PropertyID)
	if err != nil {
		return domain.Room{}, fmt.Errorf("invalid property ID: %w", err)
	}
	roomTypeID, err := uuid.Parse(rm.RoomTypeID)
	if err != nil {
		return domain.Room{}, fmt.Errorf("invalid room type ID: %w", err)
	}
	var result domain.Room
	err = r.pool.WithTenant(ctx, rm.OrgID, func(ctx context.Context, tx pgx.Tx) error {
		row, err := pgstore.New(tx).UpsertRoom(ctx, pgstore.UpsertRoomParams{
			OrgID:      orgID,
			PropertyID: propertyID,
			RoomTypeID: roomTypeID,
			ExternalID: rm.ExternalID,
			Name:       rm.Name,
			IsActive:   rm.IsActive,
		})
		if err != nil {
			return err
		}
		result = mapRoom(row)
		return nil
	})
	
	if err != nil {
		return domain.Room{}, fmt.Errorf("upsert room: %w", err)
	}
	
	return result, nil
}

func (r *RoomRepository) ListByProperty(ctx context.Context, propertyID string) ([]domain.Room, error) {
	tc, err := platformauth.FromContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("pms/room_repo: %w", err)
	}
	pid, err := uuid.Parse(propertyID)
	if err != nil {
		return nil, fmt.Errorf("invalid property ID: %w", err)
	}
	var out []domain.Room
	err = r.pool.WithTenant(ctx, tc.OrgID, func(ctx context.Context, tx pgx.Tx) error {
		rows, err := pgstore.New(tx).ListRoomsByProperty(ctx, pid)
		if err != nil {
			return err
		}
		out = make([]domain.Room, len(rows))
		for i, row := range rows {
			out[i] = mapRoom(row)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("list rooms by property: %w", err)
	}
	return out, nil
}

func (r *RoomRepository) ListByRoomType(ctx context.Context, roomTypeID string) ([]domain.Room, error) {
	tc, err := platformauth.FromContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("pms/room_repo: %w", err)
	}
	rtid, err := uuid.Parse(roomTypeID)
	if err != nil {
		return nil, fmt.Errorf("invalid room type ID: %w", err)
	}
	var out []domain.Room
	err = r.pool.WithTenant(ctx, tc.OrgID, func(ctx context.Context, tx pgx.Tx) error {
		rows, err := pgstore.New(tx).ListRoomsByRoomType(ctx, rtid)
		if err != nil {
			return err
		}
		out = make([]domain.Room, len(rows))
		for i, row := range rows {
			out[i] = mapRoom(row)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("list rooms by room type: %w", err)
	}
	return out, nil
}

func mapRoom(row pgstore.PmsRoom) domain.Room {
	return domain.Room{
		ID:         row.ID.String(),
		OrgID:      row.OrgID.String(),
		PropertyID: row.PropertyID.String(),
		RoomTypeID: row.RoomTypeID.String(),
		ExternalID: row.ExternalID,
		Name:       row.Name,
		IsActive:   row.IsActive,
		CreatedAt:  row.CreatedAt,
		UpdatedAt:  row.UpdatedAt,
	}
}
