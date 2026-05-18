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

// RoomTypeRepository implements ports.RoomTypeRepository.
type RoomTypeRepository struct {
	pool *platformdb.Pool
}

// NewRoomTypeRepository creates a new repository.
func NewRoomTypeRepository(pool *platformdb.Pool) *RoomTypeRepository {
	return &RoomTypeRepository{pool: pool}
}

func (r *RoomTypeRepository) Upsert(ctx context.Context, rt domain.RoomType) (domain.RoomType, error) {
	tc, err := platformauth.FromContext(ctx)
	if err != nil {
		return domain.RoomType{}, fmt.Errorf("pms/rt_repo: %w", err)
	}
	orgID, err := uuid.Parse(tc.OrgID)
	if err != nil {
		return domain.RoomType{}, fmt.Errorf("pms/rt_repo: invalid org_id: %w", err)
	}
	propID, err := uuid.Parse(rt.PropertyID)
	if err != nil {
		return domain.RoomType{}, fmt.Errorf("pms/rt_repo: invalid property_id: %w", err)
	}
	code := rt.Code
	if code == "" {
		code = rt.ExternalID
	}
	var extPtr *string
	if rt.ExternalID != "" {
		extPtr = &rt.ExternalID
	}
	capacity := int32(rt.MaxOccupancy) //nolint:gosec
	if capacity <= 0 {
		capacity = 2
	}
	baseOcc := int32(rt.BaseOccupancy) //nolint:gosec
	if baseOcc <= 0 {
		baseOcc = capacity
	}

	var result domain.RoomType
	err = r.pool.WithTenant(ctx, tc.OrgID, func(ctx context.Context, tx pgx.Tx) error {
		q := pgstore.New(tx)
		if existing, err := q.GetRoomTypeByCode(ctx, pgstore.GetRoomTypeByCodeParams{
			PropertyID: propID,
			Code:       code,
		}); err == nil {
			row, err := q.UpdateRoomType(ctx, pgstore.UpdateRoomTypeParams{
				ID:            existing.ID,
				ExternalID:    extPtr,
				Name:          rt.Name,
				Description:   nil,
				Capacity:      capacity,
				BaseOccupancy: baseOcc,
				IsActive:      rt.IsActive,
			})
			if err != nil {
				return err
			}
			result = roomTypeToDomain(row)
			return nil
		}
		id := uuid.New()
		if rt.ID != "" {
			if parsed, err := uuid.Parse(rt.ID); err == nil {
				id = parsed
			}
		}
		row, err := q.InsertRoomType(ctx, pgstore.InsertRoomTypeParams{
			ID:            id,
			OrgID:         orgID,
			PropertyID:    propID,
			ExternalID:    extPtr,
			Code:          code,
			Name:          rt.Name,
			Description:   nil,
			Capacity:      capacity,
			BaseOccupancy: baseOcc,
			IsActive:      rt.IsActive,
		})
		if err != nil {
			return err
		}
		result = roomTypeToDomain(row)
		return nil
	})
	if err != nil {
		return domain.RoomType{}, fmt.Errorf("pms/rt_repo: upsert: %w", err)
	}
	return result, nil
}

func (r *RoomTypeRepository) ListByProperty(ctx context.Context, propertyID string) ([]domain.RoomType, error) {
	tc, err := platformauth.FromContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("pms/rt_repo: %w", err)
	}
	pid, err := uuid.Parse(propertyID)
	if err != nil {
		return nil, fmt.Errorf("pms/rt_repo: invalid property_id: %w", err)
	}
	var out []domain.RoomType
	err = r.pool.WithTenant(ctx, tc.OrgID, func(ctx context.Context, tx pgx.Tx) error {
		rows, err := pgstore.New(tx).ListRoomTypesByProperty(ctx, pid)
		if err != nil {
			return err
		}
		out = make([]domain.RoomType, 0, len(rows))
		for _, row := range rows {
			out = append(out, roomTypeToDomain(row))
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("pms/rt_repo: list: %w", err)
	}
	return out, nil
}

func (r *RoomTypeRepository) GetByExternalID(ctx context.Context, propertyID, externalID string) (domain.RoomType, error) {
	tc, err := platformauth.FromContext(ctx)
	if err != nil {
		return domain.RoomType{}, fmt.Errorf("pms/rt_repo: %w", err)
	}
	pid, err := uuid.Parse(propertyID)
	if err != nil {
		return domain.RoomType{}, fmt.Errorf("pms/rt_repo: invalid property_id: %w", err)
	}
	var result domain.RoomType
	err = r.pool.WithTenant(ctx, tc.OrgID, func(ctx context.Context, tx pgx.Tx) error {
		ext := externalID
		row, err := pgstore.New(tx).GetRoomTypeByExternal(ctx, pgstore.GetRoomTypeByExternalParams{
			PropertyID: pid,
			ExternalID: &ext,
		})
		if err != nil {
			return err
		}
		result = roomTypeToDomain(row)
		return nil
	})
	if err != nil {
		return domain.RoomType{}, fmt.Errorf("pms/rt_repo: get external: %w", err)
	}
	return result, nil
}
