package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	platformauth "github.com/channel-manager/channel-manager/platform/auth"
	platformdb "github.com/channel-manager/channel-manager/platform/db"
	"github.com/channel-manager/channel-manager/services/pms/adapters/postgres/pgstore"
	"github.com/channel-manager/channel-manager/services/pms/domain"
)

// ConnectionRepository implements ports.ConnectionRepository.
type ConnectionRepository struct {
	pool *platformdb.Pool
}

// NewConnectionRepository creates a new repository.
func NewConnectionRepository(pool *platformdb.Pool) *ConnectionRepository {
	return &ConnectionRepository{pool: pool}
}

func (r *ConnectionRepository) Create(ctx context.Context, conn domain.Connection, credentials map[string]string) (domain.Connection, error) {
	tc, err := platformauth.FromContext(ctx)
	if err != nil {
		return domain.Connection{}, fmt.Errorf("pms/conn_repo: %w", err)
	}
	orgID, err := uuid.Parse(tc.OrgID)
	if err != nil {
		return domain.Connection{}, fmt.Errorf("pms/conn_repo: invalid org_id: %w", err)
	}
	id, err := uuid.Parse(conn.ID)
	if err != nil {
		id = uuid.New()
	}
	cfgBytes, _ := json.Marshal(conn.Config)
	if cfgBytes == nil {
		cfgBytes = []byte("{}")
	}
	var secretRef *string
	if conn.SecretRef != "" {
		secretRef = &conn.SecretRef
	}
	status := conn.Status
	if status == "" {
		status = "active"
	}

	var result domain.Connection
	err = r.pool.WithTenant(ctx, tc.OrgID, func(ctx context.Context, tx pgx.Tx) error {
		q := pgstore.New(tx)
		row, err := q.CreateConnection(ctx, pgstore.CreateConnectionParams{
			ID:        id,
			OrgID:     orgID,
			Provider:  conn.Provider,
			Name:      conn.Name,
			Status:    status,
			SecretRef: secretRef,
			Config:    cfgBytes,
		})
		if err != nil {
			return err
		}
		result = connectionToDomain(row)
		return nil
	})
	if err != nil {
		return domain.Connection{}, fmt.Errorf("pms/conn_repo: create: %w", err)
	}
	_ = credentials
	return result, nil
}

func (r *ConnectionRepository) GetByID(ctx context.Context, id string) (domain.Connection, error) {
	tc, err := platformauth.FromContext(ctx)
	if err != nil {
		return domain.Connection{}, fmt.Errorf("pms/conn_repo: %w", err)
	}
	connID, err := uuid.Parse(id)
	if err != nil {
		return domain.Connection{}, fmt.Errorf("pms/conn_repo: invalid id: %w", err)
	}
	var result domain.Connection
	err = r.pool.WithTenant(ctx, tc.OrgID, func(ctx context.Context, tx pgx.Tx) error {
		row, err := pgstore.New(tx).GetConnection(ctx, connID)
		if err != nil {
			return err
		}
		result = connectionToDomain(row)
		return nil
	})
	if err != nil {
		return domain.Connection{}, fmt.Errorf("pms/conn_repo: get: %w", err)
	}
	return result, nil
}

func (r *ConnectionRepository) ListByOrg(ctx context.Context, orgID string) ([]domain.Connection, error) {
	tc, err := platformauth.FromContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("pms/conn_repo: %w", err)
	}
	oid, err := uuid.Parse(orgID)
	if err != nil {
		return nil, fmt.Errorf("pms/conn_repo: invalid org_id: %w", err)
	}
	var out []domain.Connection
	err = r.pool.WithTenant(ctx, tc.OrgID, func(ctx context.Context, tx pgx.Tx) error {
		rows, err := pgstore.New(tx).ListConnectionsByOrg(ctx, oid)
		if err != nil {
			return err
		}
		out = make([]domain.Connection, 0, len(rows))
		for _, row := range rows {
			out = append(out, connectionToDomain(row))
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("pms/conn_repo: list: %w", err)
	}
	return out, nil
}

func (r *ConnectionRepository) UpdateStatus(ctx context.Context, id, status, lastError string) error {
	tc, err := platformauth.FromContext(ctx)
	if err != nil {
		return fmt.Errorf("pms/conn_repo: %w", err)
	}
	connID, err := uuid.Parse(id)
	if err != nil {
		return fmt.Errorf("pms/conn_repo: invalid id: %w", err)
	}
	var lastErr *string
	if lastError != "" {
		lastErr = &lastError
	}
	return r.pool.WithTenant(ctx, tc.OrgID, func(ctx context.Context, tx pgx.Tx) error {
		return pgstore.New(tx).UpdateConnectionStatus(ctx, pgstore.UpdateConnectionStatusParams{
			ID:        connID,
			Status:    status,
			LastError: lastErr,
		})
	})
}

func (r *ConnectionRepository) UpdateLastSync(ctx context.Context, id string, t time.Time) error {
	tc, err := platformauth.FromContext(ctx)
	if err != nil {
		return fmt.Errorf("pms/conn_repo: %w", err)
	}
	connID, err := uuid.Parse(id)
	if err != nil {
		return fmt.Errorf("pms/conn_repo: invalid id: %w", err)
	}
	return r.pool.WithTenant(ctx, tc.OrgID, func(ctx context.Context, tx pgx.Tx) error {
		return pgstore.New(tx).UpdateConnectionLastSync(ctx, pgstore.UpdateConnectionLastSyncParams{
			ID:         connID,
			LastSyncAt: pgtypeTimestamptz(t),
		})
	})
}

func (r *ConnectionRepository) Delete(ctx context.Context, id string) error {
	tc, err := platformauth.FromContext(ctx)
	if err != nil {
		return fmt.Errorf("pms/conn_repo: %w", err)
	}
	connID, err := uuid.Parse(id)
	if err != nil {
		return fmt.Errorf("pms/conn_repo: invalid id: %w", err)
	}
	return r.pool.WithTenant(ctx, tc.OrgID, func(ctx context.Context, tx pgx.Tx) error {
		return pgstore.New(tx).DeleteConnection(ctx, connID)
	})
}

func connectionToDomain(row pgstore.PmsConnection) domain.Connection {
	var cfg map[string]string
	_ = json.Unmarshal(row.Config, &cfg)
	var lastSync *time.Time
	if row.LastSyncAt.Valid {
		t := row.LastSyncAt.Time
		lastSync = &t
	}
	secretRef := ""
	if row.SecretRef != nil {
		secretRef = *row.SecretRef
	}
	lastErr := ""
	if row.LastError != nil {
		lastErr = *row.LastError
	}
	return domain.Connection{
		ID:         row.ID.String(),
		OrgID:      row.OrgID.String(),
		Provider:   row.Provider,
		Name:       row.Name,
		Status:     row.Status,
		SecretRef:  secretRef,
		Config:     cfg,
		LastSyncAt: lastSync,
		LastError:  lastErr,
		CreatedAt:  row.CreatedAt,
		UpdatedAt:  row.UpdatedAt,
	}
}
