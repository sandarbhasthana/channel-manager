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
func NewEnforcer(pool *pgxpool.Pool) (*casbin.Enforcer, error) {
	m, err := model.NewModelFromString(casbinModelText)
	if err != nil {
		return nil, fmt.Errorf("auth: parse casbin model: %w", err)
	}
	e, err := casbin.NewEnforcer(m, NewPGAdapter(pool))
	if err != nil {
		return nil, fmt.Errorf("auth: new enforcer: %w", err)
	}
	return e, nil
}
