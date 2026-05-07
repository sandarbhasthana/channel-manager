package db

import (
	"errors"
	"fmt"
	"net/url"
	"path/filepath"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres" // pg driver
	_ "github.com/golang-migrate/migrate/v4/source/file"       // file source
)

// MigrateOptions configures a per-schema migration run.
type MigrateOptions struct {
	// MigrationsRoot is the directory that contains one sub-directory
	// per schema (e.g. /repo/migrations).
	MigrationsRoot string
	// Schema is the name of the sub-directory and the value used for
	// the migrations bookkeeping table (one table per schema).
	Schema string
	// DatabaseURL is the postgres:// URL to migrate against.
	DatabaseURL string
}

// migrator returns a configured *migrate.Migrate for a single schema.
// Each schema gets its own bookkeeping table (schema_migrations_<schema>)
// in the public schema so versions are tracked independently.
func (o MigrateOptions) migrator() (*migrate.Migrate, error) {
	if o.Schema == "" {
		return nil, errors.New("db: migrate: schema is required")
	}
	if o.MigrationsRoot == "" {
		return nil, errors.New("db: migrate: migrations root is required")
	}
	if o.DatabaseURL == "" {
		return nil, errors.New("db: migrate: database url is required")
	}

	// Per-schema migrations table — keep it in public so the runner can
	// always read it regardless of search_path.
	dburl, err := url.Parse(o.DatabaseURL)
	if err != nil {
		return nil, fmt.Errorf("db: migrate: parse url: %w", err)
	}
	q := dburl.Query()
	q.Set("x-migrations-table", "schema_migrations_"+o.Schema)
	dburl.RawQuery = q.Encode()

	src := "file://" + filepath.ToSlash(filepath.Join(o.MigrationsRoot, o.Schema))
	m, err := migrate.New(src, dburl.String())
	if err != nil {
		return nil, fmt.Errorf("db: migrate: init %s: %w", o.Schema, err)
	}
	return m, nil
}

// Up applies all pending migrations for the schema.
func Up(o MigrateOptions) error {
	m, err := o.migrator()
	if err != nil {
		return err
	}
	defer m.Close()
	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("db: migrate up %s: %w", o.Schema, err)
	}
	return nil
}

// Down rolls back every migration for the schema.
func Down(o MigrateOptions) error {
	m, err := o.migrator()
	if err != nil {
		return err
	}
	defer m.Close()
	if err := m.Down(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("db: migrate down %s: %w", o.Schema, err)
	}
	return nil
}

// Steps applies n migrations (positive=up, negative=down) for the
// schema. Useful for tests and partial rollbacks.
func Steps(o MigrateOptions, n int) error {
	m, err := o.migrator()
	if err != nil {
		return err
	}
	defer m.Close()
	if err := m.Steps(n); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("db: migrate steps %s: %w", o.Schema, err)
	}
	return nil
}

// Version returns the current version of the schema and a dirty flag.
// A non-existent migrations table is reported as version 0, dirty=false.
func Version(o MigrateOptions) (uint, bool, error) {
	m, err := o.migrator()
	if err != nil {
		return 0, false, err
	}
	defer m.Close()
	v, dirty, err := m.Version()
	if errors.Is(err, migrate.ErrNilVersion) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, fmt.Errorf("db: migrate version %s: %w", o.Schema, err)
	}
	return v, dirty, nil
}

// Force sets the schema's migration version without running anything.
// Use with care — only when manually recovering from a dirty state.
func Force(o MigrateOptions, version int) error {
	m, err := o.migrator()
	if err != nil {
		return err
	}
	defer m.Close()
	if err := m.Force(version); err != nil {
		return fmt.Errorf("db: migrate force %s: %w", o.Schema, err)
	}
	return nil
}
