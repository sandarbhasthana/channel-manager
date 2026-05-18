// Package auth provides JWT verification, Casbin RBAC, identity syncing,
// and Connect-RPC middleware for the Channel Manager platform.
package auth

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	workos "github.com/workos/workos-go/v7"
)

// Store syncs WorkOS identities into the local tenancy schema and resolves
// WorkOS IDs to local UUIDs. organizations and users tables are not
// RLS-scoped; memberships are (enforced via app.current_org_id).
type Store struct{ pool *pgxpool.Pool }

// NewStore returns an identity Store backed by pool.
func NewStore(pool *pgxpool.Pool) *Store { return &Store{pool: pool} }

// ResolveOrgID returns the local UUID for the given WorkOS org_id.
// Returns pgx.ErrNoRows (wrapped) when the org has not been mirrored yet.
func (s *Store) ResolveOrgID(ctx context.Context, workosOrgID string) (string, error) {
	var id string
	err := s.pool.QueryRow(ctx,
		`SELECT id FROM tenancy.organizations WHERE workos_id = $1`,
		workosOrgID,
	).Scan(&id)
	if err != nil {
		return "", fmt.Errorf("identity: resolve org %q: %w", workosOrgID, err)
	}
	return id, nil
}

// ResolveWorkosOrgID returns the WorkOS org ID for the given local UUID.
func (s *Store) ResolveWorkosOrgID(ctx context.Context, localOrgID string) (string, error) {
	var workosID string
	err := s.pool.QueryRow(ctx,
		`SELECT workos_id FROM tenancy.organizations WHERE id = $1`,
		localOrgID,
	).Scan(&workosID)
	if err != nil {
		return "", fmt.Errorf("identity: resolve workos org %q: %w", localOrgID, err)
	}
	return workosID, nil
}

// UpsertOrg creates or updates the local mirror of a WorkOS organization and
// returns the local UUID. The slug is derived from the name; a more robust
// slugifier should be applied in production.
func (s *Store) UpsertOrg(ctx context.Context, workosID, name string) (string, error) {
	slug := strings.ToLower(strings.NewReplacer(" ", "-", "_", "-").Replace(name))
	var id string
	err := s.pool.QueryRow(ctx,
		`INSERT INTO tenancy.organizations (workos_id, name, slug)
		 VALUES ($1, $2, $3)
		 ON CONFLICT (workos_id) DO UPDATE
		     SET name = EXCLUDED.name, updated_at = now()
		 RETURNING id`,
		workosID, name, slug,
	).Scan(&id)
	if err != nil {
		return "", fmt.Errorf("identity: upsert org %q: %w", workosID, err)
	}
	return id, nil
}

// UpsertUser creates or updates the local mirror of a WorkOS user. If
// localOrgID is non-empty the membership is also upserted inside a
// tenant-scoped transaction (required by memberships RLS policy).
func (s *Store) UpsertUser(ctx context.Context, u *workos.User, localOrgID, role string) error {
	fullName := buildFullName(u.FirstName, u.LastName)

	// Upsert user — no RLS on users table.
	if _, err := s.pool.Exec(ctx,
		`INSERT INTO tenancy.users (id, email, full_name, default_org_id)
		 VALUES ($1, $2, $3, $4::uuid)
		 ON CONFLICT (id) DO UPDATE
		     SET email      = EXCLUDED.email,
		         full_name  = EXCLUDED.full_name,
		         updated_at = now()`,
		u.ID, u.Email, fullName, nilIfEmpty(localOrgID),
	); err != nil {
		return fmt.Errorf("identity: upsert user %q: %w", u.ID, err)
	}

	if localOrgID == "" {
		return nil
	}

	// Upsert membership — RLS requires setting app.current_org_id.
	if role == "" {
		role = "member"
	}
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("identity: begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err = tx.Exec(ctx,
		"SELECT set_config('app.current_org_id', $1, true)", localOrgID,
	); err != nil {
		return fmt.Errorf("identity: set tenant: %w", err)
	}
	if _, err = tx.Exec(ctx,
		`INSERT INTO tenancy.memberships (org_id, user_id, role)
		 VALUES ($1::uuid, $2, $3)
		 ON CONFLICT (org_id, user_id) DO UPDATE
		     SET role = EXCLUDED.role, updated_at = now()`,
		localOrgID, u.ID, role,
	); err != nil {
		return fmt.Errorf("identity: upsert membership: %w", err)
	}
	return tx.Commit(ctx)
}

// buildFullName concatenates first and last names.
func buildFullName(first, last *string) string {
	var parts []string
	if first != nil && *first != "" {
		parts = append(parts, *first)
	}
	if last != nil && *last != "" {
		parts = append(parts, *last)
	}
	return strings.Join(parts, " ")
}

// nilIfEmpty returns nil when s is empty (for nullable UUID columns).
func nilIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
