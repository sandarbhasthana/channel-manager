package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"

	platformauth "github.com/channel-manager/channel-manager/platform/auth"
	platformdb "github.com/channel-manager/channel-manager/platform/db"
)

// ChannelRateRule is a per-channel price adjustment for one room type.
//
// It is the Channel Manager-owned layer on top of the PMS base rate: the final
// rate a channel receives is base * (1 + AdjustPct/100). Stored per
// (property, room_type, channel).
type ChannelRateRule struct {
	RoomTypeID string  `json:"roomTypeId"`
	ChannelID  string  `json:"channelId"`
	AdjustPct  float64 `json:"adjustPct"`
}

// ChannelRateRepository reads and writes per-channel rate rules.
type ChannelRateRepository struct {
	pool *platformdb.Pool
}

// NewChannelRateRepository creates a repository backed by pool.
func NewChannelRateRepository(pool *platformdb.Pool) *ChannelRateRepository {
	return &ChannelRateRepository{pool: pool}
}

// List returns every per-channel rule for a property, scoped to the caller's org.
func (r *ChannelRateRepository) List(ctx context.Context, propertyID string) ([]ChannelRateRule, error) {
	tc, err := platformauth.FromContext(ctx)
	if err != nil {
		return nil, err
	}
	org := tc.OrgID

	out := make([]ChannelRateRule, 0)
	err = r.pool.WithTenant(ctx, org, func(ctx context.Context, tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			SELECT room_type_id::text, channel_id::text, adjust_pct
			  FROM pricing.channel_rate_rules
			 WHERE org_id = $1::uuid AND property_id = $2::uuid
			 ORDER BY room_type_id, channel_id`, org, propertyID)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var rule ChannelRateRule
			if err := rows.Scan(&rule.RoomTypeID, &rule.ChannelID, &rule.AdjustPct); err != nil {
				return err
			}
			out = append(out, rule)
		}
		return rows.Err()
	})
	if err != nil {
		return nil, fmt.Errorf("pricing: list channel rate rules: %w", err)
	}
	return out, nil
}

// Upsert writes the given rules for a property. Each rule is keyed by
// (org, property, room_type, channel); an existing rule's adjustment is updated.
func (r *ChannelRateRepository) Upsert(ctx context.Context, propertyID string, rules []ChannelRateRule) error {
	if len(rules) == 0 {
		return nil
	}
	tc, err := platformauth.FromContext(ctx)
	if err != nil {
		return err
	}
	org := tc.OrgID

	return r.pool.WithTenant(ctx, org, func(ctx context.Context, tx pgx.Tx) error {
		for _, rule := range rules {
			if _, err := tx.Exec(ctx, `
				INSERT INTO pricing.channel_rate_rules
					(org_id, property_id, room_type_id, channel_id, adjust_pct)
				VALUES ($1::uuid, $2::uuid, $3::uuid, $4::uuid, $5)
				ON CONFLICT (org_id, property_id, room_type_id, channel_id)
				DO UPDATE SET adjust_pct = EXCLUDED.adjust_pct, updated_at = now()`,
				org, propertyID, rule.RoomTypeID, rule.ChannelID, rule.AdjustPct); err != nil {
				return fmt.Errorf("pricing: upsert channel rate rule: %w", err)
			}
		}
		return nil
	})
}
