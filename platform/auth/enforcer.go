package auth

import (
	_ "embed"
	"fmt"

	casbin "github.com/casbin/casbin/v3"
	"github.com/casbin/casbin/v3/model"
	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed casbin_model.conf
var casbinModelText string

// NewEnforcer creates a Casbin enforcer wired to the tenancy.casbin_rule
// Postgres table. The RBAC model is embedded from casbin_model.conf at
// compile time; runtime policy changes are persisted through pgAdapter.
//
// Auto-save is enabled (the default) so AddPolicy / RemovePolicy calls on
// the returned enforcer are immediately reflected in the database.
//
// A SyncedEnforcer, not a plain Enforcer: the policy model is both read
// (Enforce, in the Connect-RPC interceptor) and written (RoleBinder.Ensure, in
// the HTTP middleware) from concurrent request goroutines. A plain
// *casbin.Enforcer is not safe for that and races on the underlying model maps.
func NewEnforcer(pool *pgxpool.Pool) (*casbin.SyncedEnforcer, error) {
	m, err := model.NewModelFromString(casbinModelText)
	if err != nil {
		return nil, fmt.Errorf("auth: parse casbin model: %w", err)
	}
	e, err := casbin.NewSyncedEnforcer(m, NewPGAdapter(pool))
	if err != nil {
		return nil, fmt.Errorf("auth: new enforcer: %w", err)
	}
	return e, nil
}
