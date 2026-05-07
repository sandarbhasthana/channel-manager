package auth

import (
	"context"
	"fmt"
	"strings"

	"github.com/casbin/casbin/v3/model"
	"github.com/casbin/casbin/v3/persist"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// pgAdapter implements casbin persist.Adapter and persist.BatchAdapter
// backed by the tenancy.casbin_rule table.
//
// LoadPolicy and SavePolicy bypass per-tenant RLS (current_org_id is left
// empty; the policy allows all rows in that case). AddPolicy / RemovePolicy /
// RemoveFilteredPolicy extract the domain from the rule and execute inside a
// tenant-scoped transaction so writes are correctly scoped.
type pgAdapter struct{ pool *pgxpool.Pool }

// NewPGAdapter returns a Casbin Postgres adapter backed by pool.
func NewPGAdapter(pool *pgxpool.Pool) *pgAdapter { return &pgAdapter{pool: pool} }

// domainFromRule extracts the org UUID from a casbin rule.
// Policy rules ("p"): sub, dom, obj, act → domain at index 1.
// Grouping rules ("g"): user, role, domain → domain at index 2.
func domainFromRule(sec string, rule []string) string {
	switch sec {
	case "p":
		if len(rule) > 1 {
			return rule[1]
		}
	case "g":
		if len(rule) > 2 {
			return rule[2]
		}
	}
	return ""
}

// execInTenant opens a TX, optionally sets app.current_org_id, runs fn, commits.
func (a *pgAdapter) execInTenant(ctx context.Context, orgID string, fn func(pgx.Tx) error) error {
	tx, err := a.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("casbin adapter: begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if orgID != "" {
		if _, err = tx.Exec(ctx, "SELECT set_config('app.current_org_id', $1, true)", orgID); err != nil {
			return fmt.Errorf("casbin adapter: set tenant: %w", err)
		}
	}
	if err = fn(tx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// insertRule inserts a single policy row, ignoring conflicts.
func insertRule(ctx context.Context, tx pgx.Tx, ptype string, rule []string) error {
	args := make([]interface{}, 7)
	args[0] = ptype
	for i := 0; i < 6 && i < len(rule); i++ {
		args[i+1] = rule[i]
	}
	_, err := tx.Exec(ctx,
		`INSERT INTO tenancy.casbin_rule (ptype, v0, v1, v2, v3, v4, v5)
		 VALUES ($1,$2,$3,$4,$5,$6,$7) ON CONFLICT DO NOTHING`,
		args...,
	)
	return err
}

// LoadPolicy loads all policy rules from Postgres into the Casbin model.
func (a *pgAdapter) LoadPolicy(m model.Model) error {
	ctx := context.Background()
	rows, err := a.pool.Query(ctx,
		`SELECT ptype, v0, v1, v2, v3, v4, v5 FROM tenancy.casbin_rule`)
	if err != nil {
		return fmt.Errorf("casbin adapter: load policy: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var ptype string
		vals := make([]*string, 6)
		dest := []interface{}{&ptype, &vals[0], &vals[1], &vals[2], &vals[3], &vals[4], &vals[5]}
		if err = rows.Scan(dest...); err != nil {
			return fmt.Errorf("casbin adapter: scan: %w", err)
		}
		parts := []string{ptype}
		for _, v := range vals {
			if v == nil {
				break
			}
			parts = append(parts, *v)
		}
		if err = persist.LoadPolicyLine(strings.Join(parts, ", "), m); err != nil {
			return err
		}
	}
	return rows.Err()
}

// SavePolicy clears the table then bulk-inserts every rule in the model.
func (a *pgAdapter) SavePolicy(m model.Model) error {
	ctx := context.Background()
	return a.execInTenant(ctx, "", func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, "DELETE FROM tenancy.casbin_rule"); err != nil {
			return err
		}
		for ptype, assertion := range m["p"] {
			for _, rule := range assertion.Policy {
				if err := insertRule(ctx, tx, ptype, rule); err != nil {
					return err
				}
			}
		}
		for ptype, assertion := range m["g"] {
			for _, rule := range assertion.Policy {
				if err := insertRule(ctx, tx, ptype, rule); err != nil {
					return err
				}
			}
		}
		return nil
	})
}

// AddPolicy inserts a single rule.
func (a *pgAdapter) AddPolicy(sec, ptype string, rule []string) error {
	ctx := context.Background()
	return a.execInTenant(ctx, domainFromRule(sec, rule), func(tx pgx.Tx) error {
		return insertRule(ctx, tx, ptype, rule)
	})
}

// RemovePolicy deletes a single exact rule using IS NOT DISTINCT FROM for NULL safety.
func (a *pgAdapter) RemovePolicy(sec, ptype string, rule []string) error {
	ctx := context.Background()
	return a.execInTenant(ctx, domainFromRule(sec, rule), func(tx pgx.Tx) error {
		args := make([]interface{}, 7)
		args[0] = ptype
		for i := 0; i < 6 && i < len(rule); i++ {
			args[i+1] = rule[i]
		}
		_, err := tx.Exec(ctx,
			`DELETE FROM tenancy.casbin_rule
			 WHERE ptype=$1
			   AND v0 IS NOT DISTINCT FROM $2
			   AND v1 IS NOT DISTINCT FROM $3
			   AND v2 IS NOT DISTINCT FROM $4
			   AND v3 IS NOT DISTINCT FROM $5
			   AND v4 IS NOT DISTINCT FROM $6
			   AND v5 IS NOT DISTINCT FROM $7`,
			args...,
		)
		return err
	})
}

// RemoveFilteredPolicy deletes rules whose columns (starting at fieldIndex) match fieldValues.
func (a *pgAdapter) RemoveFilteredPolicy(sec, ptype string, fieldIndex int, fieldValues ...string) error {
	ctx := context.Background()
	cols := []string{"v0", "v1", "v2", "v3", "v4", "v5"}
	args := []interface{}{ptype}
	var conditions []string
	var orgID string
	for i, v := range fieldValues {
		if v == "" {
			continue
		}
		col := cols[fieldIndex+i]
		args = append(args, v)
		conditions = append(conditions, fmt.Sprintf("%s = $%d", col, len(args)))
		if col == "v1" && sec == "p" {
			orgID = v
		}
		if col == "v2" && sec == "g" {
			orgID = v
		}
	}
	where := "ptype = $1"
	if len(conditions) > 0 {
		where += " AND " + strings.Join(conditions, " AND ")
	}
	return a.execInTenant(ctx, orgID, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, "DELETE FROM tenancy.casbin_rule WHERE "+where, args...)
		return err
	})
}

// AddPolicies implements persist.BatchAdapter.
func (a *pgAdapter) AddPolicies(sec, ptype string, rules [][]string) error {
	for _, rule := range rules {
		if err := a.AddPolicy(sec, ptype, rule); err != nil {
			return err
		}
	}
	return nil
}

// RemovePolicies implements persist.BatchAdapter.
func (a *pgAdapter) RemovePolicies(sec, ptype string, rules [][]string) error {
	for _, rule := range rules {
		if err := a.RemovePolicy(sec, ptype, rule); err != nil {
			return err
		}
	}
	return nil
}
