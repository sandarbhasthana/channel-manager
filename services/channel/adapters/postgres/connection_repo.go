// Package postgres implements channel ports backed by Postgres.
package postgres

import (
	"context"
	"encoding/json"
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

// ConnectionRepository implements ports.ConnectionRepository.
type ConnectionRepository struct {
	pool *platformdb.Pool
}

// NewConnectionRepository creates a new Postgres-backed connection repository.
func NewConnectionRepository(pool *platformdb.Pool) *ConnectionRepository {
	return &ConnectionRepository{pool: pool}
}

func (r *ConnectionRepository) Create(ctx context.Context, conn domain.Connection) (domain.Connection, error) {
	tc, err := platformauth.FromContext(ctx)
	if err != nil {
		return domain.Connection{}, fmt.Errorf("channel/conn_repo: %w", err)
	}

	orgID, err := uuid.Parse(tc.OrgID)
	if err != nil {
		return domain.Connection{}, fmt.Errorf("channel/conn_repo: invalid org_id: %w", err)
	}

	id, err := uuid.Parse(conn.ID)
	if err != nil {
		return domain.Connection{}, fmt.Errorf("channel/conn_repo: invalid id: %w", err)
	}

	cfgBytes, err := json.Marshal(conn.Config)
	if err != nil {
		return domain.Connection{}, fmt.Errorf("channel/conn_repo: marshal config: %w", err)
	}

	var result domain.Connection
	txErr := r.pool.WithTenant(ctx, tc.OrgID, func(ctx context.Context, tx pgx.Tx) error {
		q := pgstore.New(tx)
		var secretRef *string
		if conn.SecretRef != "" {
			secretRef = &conn.SecretRef
		}
		row, err := q.CreateConnection(ctx, pgstore.CreateConnectionParams{
			ID:        id,
			OrgID:     orgID,
			Provider:  conn.Provider,
			Name:      conn.Name,
			Status:    conn.Status,
			SecretRef: secretRef,
			Config:    cfgBytes,
		})
		if err != nil {
			return err
		}
		result = connectionToDomain(row)
		return nil
	})
	if txErr != nil {
		return domain.Connection{}, fmt.Errorf("channel/conn_repo: create: %w", txErr)
	}
	return result, nil
}

func (r *ConnectionRepository) GetByID(ctx context.Context, id string) (domain.Connection, error) {
	tc, err := platformauth.FromContext(ctx)
	if err != nil {
		return domain.Connection{}, fmt.Errorf("channel/conn_repo: %w", err)
	}

	uid, err := uuid.Parse(id)
	if err != nil {
		return domain.Connection{}, fmt.Errorf("channel/conn_repo: invalid id: %w", err)
	}

	var result domain.Connection
	txErr := r.pool.WithTenant(ctx, tc.OrgID, func(ctx context.Context, tx pgx.Tx) error {
		q := pgstore.New(tx)
		row, err := q.GetConnectionByID(ctx, uid)
		if err != nil {
			return err
		}
		result = connectionToDomain(row)
		return nil
	})
	if txErr != nil {
		return domain.Connection{}, fmt.Errorf("channel/conn_repo: get: %w", txErr)
	}
	return result, nil
}

func (r *ConnectionRepository) ListByOrg(ctx context.Context, orgID string) ([]domain.Connection, error) {
	tc, err := platformauth.FromContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("channel/conn_repo: %w", err)
	}

	uid, err := uuid.Parse(orgID)
	if err != nil {
		return nil, fmt.Errorf("channel/conn_repo: invalid org_id: %w", err)
	}

	var results []domain.Connection
	txErr := r.pool.WithTenant(ctx, tc.OrgID, func(ctx context.Context, tx pgx.Tx) error {
		q := pgstore.New(tx)
		rows, err := q.ListConnectionsByOrg(ctx, uid)
		if err != nil {
			return err
		}
		results = make([]domain.Connection, 0, len(rows))
		for _, row := range rows {
			results = append(results, connectionToDomain(row))
		}
		return nil
	})
	if txErr != nil {
		return nil, fmt.Errorf("channel/conn_repo: list: %w", txErr)
	}
	return results, nil
}

func (r *ConnectionRepository) UpdateStatus(ctx context.Context, id string, status string, lastError string) error {
	tc, err := platformauth.FromContext(ctx)
	if err != nil {
		return fmt.Errorf("channel/conn_repo: %w", err)
	}
	uid, _ := uuid.Parse(id)
	var le *string
	if lastError != "" {
		le = &lastError
	}
	return r.pool.WithTenant(ctx, tc.OrgID, func(ctx context.Context, tx pgx.Tx) error {
		return pgstore.New(tx).UpdateConnectionStatus(ctx, pgstore.UpdateConnectionStatusParams{ID: uid, Status: status, LastError: le})
	})
}

func (r *ConnectionRepository) UpdateName(ctx context.Context, id string, name string) error {
	tc, err := platformauth.FromContext(ctx)
	if err != nil {
		return fmt.Errorf("channel/conn_repo: %w", err)
	}
	uid, _ := uuid.Parse(id)
	return r.pool.WithTenant(ctx, tc.OrgID, func(ctx context.Context, tx pgx.Tx) error {
		return pgstore.New(tx).UpdateConnectionName(ctx, pgstore.UpdateConnectionNameParams{ID: uid, Name: name})
	})
}

func (r *ConnectionRepository) UpdateLastSync(ctx context.Context, id string, lastSyncAt time.Time) error {
	tc, err := platformauth.FromContext(ctx)
	if err != nil {
		return fmt.Errorf("channel/conn_repo: %w", err)
	}
	uid, _ := uuid.Parse(id)
	return r.pool.WithTenant(ctx, tc.OrgID, func(ctx context.Context, tx pgx.Tx) error {
		return pgstore.New(tx).UpdateConnectionLastSync(ctx, pgstore.UpdateConnectionLastSyncParams{
			ID:         uid,
			LastSyncAt: pgtype.Timestamptz{Time: lastSyncAt, Valid: true},
		})
	})
}

func (r *ConnectionRepository) Delete(ctx context.Context, id string) error {
	tc, err := platformauth.FromContext(ctx)
	if err != nil {
		return fmt.Errorf("channel/conn_repo: %w", err)
	}
	uid, _ := uuid.Parse(id)
	return r.pool.WithTenant(ctx, tc.OrgID, func(ctx context.Context, tx pgx.Tx) error {
		return pgstore.New(tx).DeleteConnection(ctx, uid)
	})
}

// connectionToDomain maps a pgstore Connection row to the domain model.
func connectionToDomain(row pgstore.Connection) domain.Connection {
	c := domain.Connection{
		ID:        row.ID.String(),
		OrgID:     row.OrgID.String(),
		Provider:  row.Provider,
		Name:      row.Name,
		Status:    row.Status,
		CreatedAt: row.CreatedAt,
		UpdatedAt: row.UpdatedAt,
	}
	if row.SecretRef != nil {
		c.SecretRef = *row.SecretRef
	}
	if row.LastError != nil {
		c.LastError = *row.LastError
	}
	if row.LastSyncAt.Valid {
		t := row.LastSyncAt.Time
		c.LastSyncAt = &t
	}
	// Config — unmarshal JSON if present.
	if len(row.Config) > 0 {
		_ = json.Unmarshal(row.Config, &c.Config)
	}
	return c
}
