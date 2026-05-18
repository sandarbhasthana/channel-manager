package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	platformauth "github.com/channel-manager/channel-manager/platform/auth"
	platformdb "github.com/channel-manager/channel-manager/platform/db"
	"github.com/channel-manager/channel-manager/services/channel/adapters/postgres/pgstore"
	"github.com/channel-manager/channel-manager/services/channel/domain"
)

// ChannelRepository implements ports.ChannelRepository.
type ChannelRepository struct {
	pool *platformdb.Pool
}

// NewChannelRepository creates a new Postgres-backed channel repository.
func NewChannelRepository(pool *platformdb.Pool) *ChannelRepository {
	return &ChannelRepository{pool: pool}
}

func (r *ChannelRepository) Create(ctx context.Context, ch domain.Channel) (domain.Channel, error) {
	tc, err := platformauth.FromContext(ctx)
	if err != nil {
		return domain.Channel{}, fmt.Errorf("channel/ch_repo: %w", err)
	}
	orgID, _ := uuid.Parse(tc.OrgID)
	id, _ := uuid.Parse(ch.ID)
	propID, _ := uuid.Parse(ch.PropertyID)
	connID, _ := uuid.Parse(ch.ConnectionID)

	var result domain.Channel
	txErr := r.pool.WithTenant(ctx, tc.OrgID, func(ctx context.Context, tx pgx.Tx) error {
		row, err := pgstore.New(tx).CreateChannel(ctx, pgstore.CreateChannelParams{
			ID:                 id,
			OrgID:              orgID,
			PropertyID:         propID,
			ConnectionID:       connID,
			Provider:           ch.Provider,
			ExternalPropertyID: ch.ExternalPropertyID,
			Status:             ch.Status,
		})
		if err != nil {
			return err
		}
		result = channelToDomain(row)
		return nil
	})
	if txErr != nil {
		return domain.Channel{}, fmt.Errorf("channel/ch_repo: create: %w", txErr)
	}
	return result, nil
}

func (r *ChannelRepository) GetByID(ctx context.Context, id string) (domain.Channel, error) {
	tc, err := platformauth.FromContext(ctx)
	if err != nil {
		return domain.Channel{}, fmt.Errorf("channel/ch_repo: %w", err)
	}
	uid, _ := uuid.Parse(id)
	var result domain.Channel
	txErr := r.pool.WithTenant(ctx, tc.OrgID, func(ctx context.Context, tx pgx.Tx) error {
		row, err := pgstore.New(tx).GetChannelByID(ctx, uid)
		if err != nil {
			return err
		}
		result = channelToDomain(row)
		return nil
	})
	if txErr != nil {
		return domain.Channel{}, fmt.Errorf("channel/ch_repo: get: %w", txErr)
	}
	return result, nil
}

func (r *ChannelRepository) ListByProperty(ctx context.Context, propertyID string) ([]domain.Channel, error) {
	tc, err := platformauth.FromContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("channel/ch_repo: %w", err)
	}
	propID, _ := uuid.Parse(propertyID)
	var results []domain.Channel
	txErr := r.pool.WithTenant(ctx, tc.OrgID, func(ctx context.Context, tx pgx.Tx) error {
		rows, err := pgstore.New(tx).ListChannelsByProperty(ctx, propID)
		if err != nil {
			return err
		}
		results = make([]domain.Channel, 0, len(rows))
		for _, row := range rows {
			results = append(results, channelToDomain(row))
		}
		return nil
	})
	if txErr != nil {
		return nil, fmt.Errorf("channel/ch_repo: list: %w", txErr)
	}
	return results, nil
}

func (r *ChannelRepository) UpdateStatus(ctx context.Context, id string, status string, lastError string) error {
	tc, err := platformauth.FromContext(ctx)
	if err != nil {
		return fmt.Errorf("channel/ch_repo: %w", err)
	}
	uid, _ := uuid.Parse(id)
	var le *string
	if lastError != "" {
		le = &lastError
	}
	return r.pool.WithTenant(ctx, tc.OrgID, func(ctx context.Context, tx pgx.Tx) error {
		return pgstore.New(tx).UpdateChannelStatus(ctx, pgstore.UpdateChannelStatusParams{ID: uid, Status: status, LastError: le})
	})
}

func (r *ChannelRepository) UpdateLastSync(ctx context.Context, id string, lastSyncAt time.Time) error {
	tc, err := platformauth.FromContext(ctx)
	if err != nil {
		return fmt.Errorf("channel/ch_repo: %w", err)
	}
	uid, _ := uuid.Parse(id)
	return r.pool.WithTenant(ctx, tc.OrgID, func(ctx context.Context, tx pgx.Tx) error {
		return pgstore.New(tx).UpdateChannelLastSync(ctx, pgstore.UpdateChannelLastSyncParams{
			ID:         uid,
			LastSyncAt: pgtype.Timestamptz{Time: lastSyncAt, Valid: true},
		})
	})
}

func (r *ChannelRepository) Delete(ctx context.Context, id string) error {
	tc, err := platformauth.FromContext(ctx)
	if err != nil {
		return fmt.Errorf("channel/ch_repo: %w", err)
	}
	uid, _ := uuid.Parse(id)
	return r.pool.WithTenant(ctx, tc.OrgID, func(ctx context.Context, tx pgx.Tx) error {
		return pgstore.New(tx).DeleteChannel(ctx, uid)
	})
}

// channelToDomain maps a pgstore Channel row to the domain model.
func channelToDomain(row pgstore.Channel) domain.Channel {
	c := domain.Channel{
		ID:                 row.ID.String(),
		OrgID:              row.OrgID.String(),
		PropertyID:         row.PropertyID.String(),
		ConnectionID:       row.ConnectionID.String(),
		Provider:           row.Provider,
		ExternalPropertyID: row.ExternalPropertyID,
		Status:             row.Status,
		CreatedAt:          row.CreatedAt,
		UpdatedAt:          row.UpdatedAt,
	}
	if row.LastError != nil {
		c.LastError = *row.LastError
	}
	if row.LastSyncAt.Valid {
		t := row.LastSyncAt.Time
		c.LastSyncAt = &t
	}
	return c
}
