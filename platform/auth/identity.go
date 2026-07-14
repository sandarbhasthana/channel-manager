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
	//
	// An empty role means "caller does not know the role" — which is the case
	// on the login path, where the callback has no membership information. It
	// must therefore seed 'member' on insert but leave an existing role alone,
	// otherwise every sign-in would silently demote an owner. The membership
	// webhook is the authority on role changes.
	return s.upsertMembership(ctx, localOrgID, u.ID, role)
}

// UpsertMembership creates or updates a user's membership of an organization.
//
// role may be empty, meaning "do not change an existing role"; a new row is
// seeded as 'member'. A non-empty role must already be normalised — see
// NormalizeRole — because tenancy.memberships has a CHECK constraint and an
// unrecognised WorkOS slug would fail the insert.
func (s *Store) UpsertMembership(ctx context.Context, localOrgID, userID, role string) error {
	return s.upsertMembership(ctx, localOrgID, userID, role)
}

func (s *Store) upsertMembership(ctx context.Context, localOrgID, userID, role string) error {
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
		 VALUES ($1::uuid, $2, COALESCE(NULLIF($3, ''), 'member'))
		 ON CONFLICT (org_id, user_id) DO UPDATE
		     SET role = CASE WHEN $3 = '' THEN memberships.role ELSE EXCLUDED.role END,
		         updated_at = now()`,
		localOrgID, userID, role,
	); err != nil {
		return fmt.Errorf("identity: upsert membership: %w", err)
	}
	return tx.Commit(ctx)
}

// DeleteMembership removes a user from an organization, revoking local access.
// Deleting a membership that does not exist is not an error.
func (s *Store) DeleteMembership(ctx context.Context, localOrgID, userID string) error {
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
		`DELETE FROM tenancy.memberships WHERE org_id = $1::uuid AND user_id = $2`,
		localOrgID, userID,
	); err != nil {
		return fmt.Errorf("identity: delete membership: %w", err)
	}
	return tx.Commit(ctx)
}

// UserExists reports whether a WorkOS user has been mirrored locally.
//
// tenancy.memberships.user_id has a foreign key onto tenancy.users, so a
// membership event that arrives before its user event cannot be persisted.
func (s *Store) UserExists(ctx context.Context, userID string) (bool, error) {
	var exists bool
	if err := s.pool.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM tenancy.users WHERE id = $1)`, userID,
	).Scan(&exists); err != nil {
		return false, fmt.Errorf("identity: user exists %q: %w", userID, err)
	}
	return exists, nil
}

// NormalizeRole maps a WorkOS role slug onto a role this platform grants
// permissions to, defaulting to RoleMember.
//
// Two reasons this is not a passthrough. tenancy.memberships has a CHECK
// constraint, so an arbitrary slug fails the insert. And an unrecognised slug
// should degrade to read-only rather than lock a legitimate member out.
func NormalizeRole(slug string) string {
	if KnownRole(slug) {
		return slug
	}
	return RoleMember
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
