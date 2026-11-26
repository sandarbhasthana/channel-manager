package main

import (
	"context"

	"github.com/channel-manager/channel-manager/platform/db"
)

// orgDirectory lists the tenants the sync worker's jobs run for. The tenancy
// directory is not row-level secured (it is the tenant root), so it is read
// on the raw pool, outside any tenant transaction.
type orgDirectory struct{ pool *db.Pool }

func (o orgDirectory) ActiveOrgIDs(ctx context.Context) ([]string, error) {
	rows, err := o.pool.Inner().Query(ctx,
		`SELECT id::text FROM tenancy.organizations WHERE status = 'active' ORDER BY created_at`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}
