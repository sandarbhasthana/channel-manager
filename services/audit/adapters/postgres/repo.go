// Package postgres implements ports.AuditRepository against audit.audit_events.
//
// The table is append-only: RLS grants INSERT and SELECT and defines no
// UPDATE or DELETE policy, so those operations are denied to the app role.
// This repository therefore exposes no mutation beyond Append.
package postgres

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	platformauth "github.com/channel-manager/channel-manager/platform/auth"
	platformdb "github.com/channel-manager/channel-manager/platform/db"
	"github.com/channel-manager/channel-manager/services/audit/domain"
)

// Repository implements ports.AuditRepository.
type Repository struct {
	pool *platformdb.Pool
}

// NewRepository creates an audit repository backed by pool.
func NewRepository(pool *platformdb.Pool) *Repository {
	return &Repository{pool: pool}
}

const insertSQL = `
INSERT INTO audit.audit_events
    (id, org_id, actor_id, actor_type, action, resource_type, resource_id,
     request_id, trace_id, before, after, metadata)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
RETURNING id, created_at`

const selectCols = `
    id, org_id, actor_id, actor_type, action, resource_type, resource_id,
    request_id, trace_id, before, after, metadata, created_at`

// Append writes an audit entry. OrgID must be set by the caller; it is used
// both for the row value and to scope the RLS transaction, and the insert
// policy rejects any mismatch between the two.
func (r *Repository) Append(ctx context.Context, entry *domain.AuditEntry) error {
	if entry.OrgID == "" {
		return fmt.Errorf("audit: org_id is required")
	}
	if entry.Action == "" || entry.ResourceType == "" {
		return fmt.Errorf("audit: action and resource_type are required")
	}
	if entry.ActorType == "" {
		entry.ActorType = domain.ActorSystem
	}
	if entry.ID == "" {
		entry.ID = uuid.NewString()
	}

	orgID, err := uuid.Parse(entry.OrgID)
	if err != nil {
		return fmt.Errorf("audit: invalid org_id: %w", err)
	}
	id, err := uuid.Parse(entry.ID)
	if err != nil {
		return fmt.Errorf("audit: invalid id: %w", err)
	}

	metadata := entry.Metadata
	if len(metadata) == 0 {
		metadata = []byte("{}")
	}

	return r.pool.WithTenant(ctx, entry.OrgID, func(ctx context.Context, tx pgx.Tx) error {
		return tx.QueryRow(ctx, insertSQL,
			id,
			orgID,
			nullableText(entry.ActorID),
			string(entry.ActorType),
			entry.Action,
			entry.ResourceType,
			entry.ResourceID,
			nullableText(entry.RequestID),
			nullableText(entry.TraceID),
			nullableJSON(entry.Before),
			nullableJSON(entry.After),
			metadata,
		).Scan(&entry.ID, &entry.CreatedAt)
	})
}

// ListByResource returns entries for one resource, newest first.
func (r *Repository) ListByResource(ctx context.Context, resourceType, resourceID string) ([]domain.AuditEntry, error) {
	const q = `SELECT` + selectCols + `
FROM audit.audit_events
WHERE resource_type = $1 AND resource_id = $2
ORDER BY created_at DESC`
	return r.query(ctx, q, resourceType, resourceID)
}

// ListByActor returns entries produced by one actor, newest first.
func (r *Repository) ListByActor(ctx context.Context, actorID string) ([]domain.AuditEntry, error) {
	const q = `SELECT` + selectCols + `
FROM audit.audit_events
WHERE actor_id = $1
ORDER BY created_at DESC`
	return r.query(ctx, q, actorID)
}

// ListByOrg returns a page of entries for an org, newest first.
func (r *Repository) ListByOrg(ctx context.Context, orgID string, limit, offset int) ([]domain.AuditEntry, error) {
	if limit <= 0 {
		limit = 50
	}
	const q = `SELECT` + selectCols + `
FROM audit.audit_events
ORDER BY created_at DESC
LIMIT $1 OFFSET $2`
	return r.queryWithOrg(ctx, orgID, q, limit, offset)
}

// query runs a read scoped to the org in the caller's tenant context.
//
// The reads below carry no explicit org_id predicate: RLS restricts every
// SELECT to current_setting('app.current_org_id'), so a filter here would be
// redundant and could drift from the policy.
func (r *Repository) query(ctx context.Context, sql string, args ...any) ([]domain.AuditEntry, error) {
	tc, err := platformauth.FromContext(ctx)
	if err != nil {
		return nil, err
	}
	return r.queryWithOrg(ctx, tc.OrgID, sql, args...)
}

func (r *Repository) queryWithOrg(ctx context.Context, orgID, sql string, args ...any) ([]domain.AuditEntry, error) {
	var out []domain.AuditEntry
	err := r.pool.WithTenant(ctx, orgID, func(ctx context.Context, tx pgx.Tx) error {
		rows, err := tx.Query(ctx, sql, args...)
		if err != nil {
			return err
		}
		defer rows.Close()

		for rows.Next() {
			var (
				e                           domain.AuditEntry
				actorID, requestID, traceID *string
				actorType                   string
				before, after, metadata     []byte
			)
			if err := rows.Scan(
				&e.ID, &e.OrgID, &actorID, &actorType, &e.Action,
				&e.ResourceType, &e.ResourceID, &requestID, &traceID,
				&before, &after, &metadata, &e.CreatedAt,
			); err != nil {
				return err
			}
			e.ActorID = deref(actorID)
			e.ActorType = domain.ActorType(actorType)
			e.RequestID = deref(requestID)
			e.TraceID = deref(traceID)
			e.Before = before
			e.After = after
			e.Metadata = metadata
			out = append(out, e)
		}
		return rows.Err()
	})
	if err != nil {
		return nil, fmt.Errorf("audit: query: %w", err)
	}
	return out, nil
}

func nullableText(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func nullableJSON(b []byte) any {
	if len(b) == 0 {
		return nil
	}
	return b
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
