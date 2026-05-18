package postgres

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	platformauth "github.com/channel-manager/channel-manager/platform/auth"
	platformdb "github.com/channel-manager/channel-manager/platform/db"
	"github.com/channel-manager/channel-manager/services/channel/adapters/postgres/pgstore"
	"github.com/channel-manager/channel-manager/services/channel/domain"
)

// SyncJobRepository implements ports.SyncJobRepository.
type SyncJobRepository struct {
	pool *platformdb.Pool
}

// NewSyncJobRepository creates a new Postgres-backed sync job repository.
func NewSyncJobRepository(pool *platformdb.Pool) *SyncJobRepository {
	return &SyncJobRepository{pool: pool}
}

func (r *SyncJobRepository) Create(ctx context.Context, job domain.SyncJob) error {
	tc, err := platformauth.FromContext(ctx)
	if err != nil {
		return fmt.Errorf("channel/job_repo: %w", err)
	}

	id, _ := uuid.Parse(job.ID)
	orgID, _ := uuid.Parse(job.OrgID)
	connID, _ := uuid.Parse(job.ConnectionID)

	payloadBytes, err := json.Marshal(job.Payload)
	if err != nil {
		return fmt.Errorf("channel/job_repo: marshal payload: %w", err)
	}

	return r.pool.WithTenant(ctx, tc.OrgID, func(ctx context.Context, tx pgx.Tx) error {
		return pgstore.New(tx).CreateSyncJob(ctx, pgstore.CreateSyncJobParams{
			ID:           id,
			OrgID:        orgID,
			ConnectionID: connID,
			JobType:      string(job.JobType),
			Status:       string(job.Status),
			Payload:      payloadBytes,
			ScheduledAt:  job.ScheduledAt,
		})
	})
}

func (r *SyncJobRepository) GetByID(ctx context.Context, id string) (domain.SyncJob, error) {
	tc, err := platformauth.FromContext(ctx)
	if err != nil {
		return domain.SyncJob{}, fmt.Errorf("channel/job_repo: %w", err)
	}
	uid, _ := uuid.Parse(id)
	var result domain.SyncJob
	txErr := r.pool.WithTenant(ctx, tc.OrgID, func(ctx context.Context, tx pgx.Tx) error {
		row, err := pgstore.New(tx).GetSyncJobByID(ctx, uid)
		if err != nil {
			return err
		}
		result = syncJobToDomain(row)
		return nil
	})
	if txErr != nil {
		return domain.SyncJob{}, fmt.Errorf("channel/job_repo: get: %w", txErr)
	}
	return result, nil
}

func (r *SyncJobRepository) UpdateStatus(ctx context.Context, id string, status domain.SyncJobStatus, result any, lastError string) error {
	tc, err := platformauth.FromContext(ctx)
	if err != nil {
		return fmt.Errorf("channel/job_repo: %w", err)
	}
	uid, _ := uuid.Parse(id)

	var resultBytes []byte
	if result != nil {
		resultBytes, _ = json.Marshal(result)
	}

	var le *string
	if lastError != "" {
		le = &lastError
	}

	return r.pool.WithTenant(ctx, tc.OrgID, func(ctx context.Context, tx pgx.Tx) error {
		return pgstore.New(tx).UpdateSyncJobStatus(ctx, pgstore.UpdateSyncJobStatusParams{
			ID:        uid,
			Status:    string(status),
			Result:    resultBytes,
			LastError: le,
		})
	})
}

func (r *SyncJobRepository) ListRecentByConnection(ctx context.Context, connectionID string, limit int32) ([]domain.SyncJob, error) {
	tc, err := platformauth.FromContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("channel/job_repo: %w", err)
	}
	connID, err := uuid.Parse(connectionID)
	if err != nil {
		return nil, fmt.Errorf("channel/job_repo: invalid connection_id: %w", err)
	}
	if limit <= 0 {
		limit = 20
	}
	var out []domain.SyncJob
	txErr := r.pool.WithTenant(ctx, tc.OrgID, func(ctx context.Context, tx pgx.Tx) error {
		rows, err := pgstore.New(tx).ListRecentSyncJobsByConnection(ctx, pgstore.ListRecentSyncJobsByConnectionParams{
			ConnectionID: connID,
			Limit:        limit,
		})
		if err != nil {
			return err
		}
		out = make([]domain.SyncJob, 0, len(rows))
		for _, row := range rows {
			out = append(out, syncJobToDomain(row))
		}
		return nil
	})
	if txErr != nil {
		return nil, fmt.Errorf("channel/job_repo: list recent: %w", txErr)
	}
	return out, nil
}

func syncJobToDomain(row pgstore.SyncJob) domain.SyncJob {
	j := domain.SyncJob{
		ID:           row.ID.String(),
		OrgID:        row.OrgID.String(),
		ConnectionID: row.ConnectionID.String(),
		JobType:      domain.SyncJobType(row.JobType),
		Status:       domain.SyncJobStatus(row.Status),
		Attempts:     int(row.Attempts),
		ScheduledAt:  row.ScheduledAt,
		CreatedAt:    row.CreatedAt,
		UpdatedAt:    row.UpdatedAt,
	}
	if row.LastError != nil {
		j.LastError = *row.LastError
	}
	if row.StartedAt.Valid {
		t := row.StartedAt.Time
		j.StartedAt = &t
	}
	if row.FinishedAt.Valid {
		t := row.FinishedAt.Time
		j.FinishedAt = &t
	}
	return j
}
