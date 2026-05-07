// Package db provides the platform-level Postgres pool, tenant-scoped
// transaction helpers, and the migration runner used by all services.
//
// All runtime queries must go through WithTenant, which opens a
// transaction and sets app.current_org_id so RLS policies can enforce
// per-tenant isolation. Callers should never use the bare pool for
// tenant-scoped data.
package db

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Config holds the database connection configuration.
type Config struct {
	Host     string
	Port     int
	DBName   string
	User     string
	Password string
	SSLMode  string
}

// DSN returns a libpq-style PostgreSQL connection string.
func (c Config) DSN() string {
	return fmt.Sprintf(
		"host=%s port=%d dbname=%s user=%s password=%s sslmode=%s",
		c.Host, c.Port, c.DBName, c.User, c.Password, c.SSLMode,
	)
}

// URL returns a postgres:// URL form, suitable for libraries (e.g.
// golang-migrate) that expect a URL instead of a keyword DSN.
func (c Config) URL() string {
	return fmt.Sprintf(
		"postgres://%s:%s@%s:%d/%s?sslmode=%s",
		c.User, c.Password, c.Host, c.Port, c.DBName, c.SSLMode,
	)
}

// Pool wraps *pgxpool.Pool to give us a single, stable type to inject
// into services. We deliberately do not re-export pgx types so the
// surface area stays small.
type Pool struct {
	inner *pgxpool.Pool
}

// NewPool opens a pgx connection pool against the configured database.
func NewPool(ctx context.Context, cfg Config) (*Pool, error) {
	pcfg, err := pgxpool.ParseConfig(cfg.DSN())
	if err != nil {
		return nil, fmt.Errorf("db: parse config: %w", err)
	}
	pool, err := pgxpool.NewWithConfig(ctx, pcfg)
	if err != nil {
		return nil, fmt.Errorf("db: open pool: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("db: ping: %w", err)
	}
	return &Pool{inner: pool}, nil
}

// Close releases the underlying pool. Safe to call multiple times.
func (p *Pool) Close() {
	if p == nil || p.inner == nil {
		return
	}
	p.inner.Close()
}

// Inner exposes the raw pgxpool for advanced callers (sqlc-generated
// code, migration tooling). Domain code should prefer WithTenant.
func (p *Pool) Inner() *pgxpool.Pool { return p.inner }

// ErrNoTenant is returned by WithTenant when an empty org id is passed.
var ErrNoTenant = errors.New("db: org id is required for tenant-scoped transaction")

// TxFunc runs inside a tenant-scoped transaction. The supplied tx has
// app.current_org_id already set; any RLS-protected query executed via
// the tx will be filtered to the supplied org.
type TxFunc func(ctx context.Context, tx pgx.Tx) error

// WithTenant runs fn inside a transaction that has app.current_org_id
// set to orgID. The setting is scoped to the transaction (SET LOCAL
// semantics via set_config(..., true)), so it is automatically cleared
// on commit/rollback.
//
// Callers MUST use this for all reads and writes against tenant-scoped
// tables; otherwise RLS policies will reject the queries.
func (p *Pool) WithTenant(ctx context.Context, orgID string, fn TxFunc) error {
	if orgID == "" {
		return ErrNoTenant
	}
	tx, err := p.inner.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("db: begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx, "SELECT set_config('app.current_org_id', $1, true)", orgID); err != nil {
		return fmt.Errorf("db: set tenant: %w", err)
	}
	if err := fn(ctx, tx); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("db: commit: %w", err)
	}
	return nil
}

// Schemas lists every per-service schema in dependency order. The
// migration runner applies them in this order on `up` and reverses
// them on `down`.
var Schemas = []string{
	"tenancy",
	"ops",
	"audit",
	"inventory",
	"pricing",
	"pms",
	"channel",
	"mapping",
	"reservations",
}
