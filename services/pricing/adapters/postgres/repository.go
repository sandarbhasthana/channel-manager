package postgres

import (
	"context"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/jackc/pgx/v5"

	platformdb "github.com/channel-manager/channel-manager/platform/db"
	"github.com/channel-manager/channel-manager/services/pricing/domain"
	"github.com/channel-manager/channel-manager/services/pricing/ports"
)

// ErrCrossOrgBatch is returned when a batch carries rate days for an
// organization other than the caller's. A batch may never span orgs.
var ErrCrossOrgBatch = errors.New("pricing: batch spans organizations")

type repository struct {
	pool *platformdb.Pool
}

// NewRepository creates a new PostgreSQL backed RateRepository.
func NewRepository(pool *platformdb.Pool) ports.RateRepository {
	return &repository{pool: pool}
}


func (r *repository) Get(ctx context.Context, propertyID, roomTypeID, ratePlanID string, date time.Time) (*domain.RateDay, error) {
	// For MVP, we'll just implement ListByRange and SaveBatch
	return nil, nil
}

func (r *repository) ListByRange(ctx context.Context, propertyID, roomTypeID, ratePlanID string, from, to time.Time) ([]domain.RateDay, error) {
	// Not strictly required for the outbound sync, but we could return rates here
	// if we wanted to show them in the Channel Manager calendar.
	return nil, nil
}

func (r *repository) Save(ctx context.Context, day *domain.RateDay) error {
	return r.SaveBatch(ctx, []domain.RateDay{*day})
}

// SaveBatch upserts rate points for the caller's organization.
//
// The tenant comes from the context, never from the rows: rate_plans and
// rate_points are FORCE ROW LEVEL SECURITY, so the write must run inside
// WithTenant or the policy's WITH CHECK rejects it. A caller-supplied org_id
// would also let one tenant write another's rates.
func (r *repository) SaveBatch(ctx context.Context, days []domain.RateDay) error {
	if len(days) == 0 {
		return nil
	}

	org, err := orgID(ctx)
	if err != nil {
		return err
	}
	for _, d := range days {
		if d.OrgID != "" && d.OrgID != org {
			return fmt.Errorf("%w: %q != %q", ErrCrossOrgBatch, d.OrgID, org)
		}
	}

	err = r.pool.WithTenant(ctx, org, func(ctx context.Context, tx pgx.Tx) error {
		for _, d := range days {
			// Resolve the rate plan and take back its id. The previous code
			// inserted ON CONFLICT DO NOTHING and then pointed rate_points at
			// d.RatePlanID -- so whenever the plan already existed under a
			// different id, the rate_point's FK referenced a row that was never
			// written. Upserting and RETURNING id makes the reference real.
			var planID string
			if err := tx.QueryRow(ctx, `
				INSERT INTO pricing.rate_plans (org_id, room_type_id, code, name, currency, is_active)
				VALUES ($1::uuid, $2::uuid, $3, $4, $5, true)
				ON CONFLICT (org_id, code)
				DO UPDATE SET code = pricing.rate_plans.code
				RETURNING id::text
			`, org, d.RoomTypeID, "BAR", "Best Available Rate", d.Currency).Scan(&planID); err != nil {
				return fmt.Errorf("pricing: upsert rate plan: %w", err)
			}

			// Round rather than truncate. int64(0.29*100) is 28: float64 cannot
			// hold 0.29, and the product lands at 28.999999999999996. A
			// conversion to int64 truncates, quietly losing a cent.
			amountMinor := int64(math.Round(d.BaseRate * 100))

			if _, err := tx.Exec(ctx, `
				INSERT INTO pricing.rate_points (org_id, rate_plan_id, stay_date, amount_minor, currency)
				VALUES ($1::uuid, $2::uuid, $3, $4, $5)
				ON CONFLICT (org_id, rate_plan_id, stay_date)
				DO UPDATE SET amount_minor = EXCLUDED.amount_minor, currency = EXCLUDED.currency
			`, org, planID, d.Date, amountMinor, d.Currency); err != nil {
				return fmt.Errorf("pricing: upsert rate point: %w", err)
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	return nil
}
