//go:build integration

// Integration tests for the rate repository. See promo_repo_integration_test.go
// for setup; run as a NON-SUPERUSER role or TestSaveBatch_RequiresTenant proves
// nothing.
package postgres

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/channel-manager/channel-manager/services/pricing/domain"
)

func rateDay(org, roomType string, date time.Time, rate float64) domain.RateDay {
	return domain.RateDay{
		OrgID: org, RoomTypeID: roomType, Date: date,
		BaseRate: rate, Currency: "USD",
	}
}

func newRoomType(t *testing.T) string { return uuidv4(t) }

func cleanupRates(t *testing.T, org string) {
	t.Helper()
	t.Cleanup(func() {
		_ = sharedPool.WithTenant(context.Background(), org, func(ctx context.Context, tx pgx.Tx) error {
			_, _ = tx.Exec(ctx, "DELETE FROM pricing.rate_points WHERE org_id = $1::uuid", org)
			_, err := tx.Exec(ctx, "DELETE FROM pricing.rate_plans WHERE org_id = $1::uuid", org)
			return err
		})
	})
}

// TestSaveBatch_WritesUnderRLS is the regression test for the original defect:
// SaveBatch began its transaction on the raw pool, so app.current_org_id was
// never set and every INSERT was rejected by the rate_plans WITH CHECK policy.
func TestSaveBatch_WritesUnderRLS(t *testing.T) {
	repo := NewRepository(sharedPool)
	org := newOrg(t, sharedPool)
	cleanupRates(t, org)
	day := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)

	if err := repo.SaveBatch(ctxFor(org), []domain.RateDay{
		rateDay(org, newRoomType(t), day, 199.00),
	}); err != nil {
		t.Fatalf("SaveBatch: %v", err)
	}

	var n int
	if err := sharedPool.WithTenant(context.Background(), org, func(ctx context.Context, tx pgx.Tx) error {
		return tx.QueryRow(ctx,
			"SELECT count(*) FROM pricing.rate_points WHERE org_id = $1::uuid", org).Scan(&n)
	}); err != nil {
		t.Fatalf("count: %v", err)
	}
	if n != 1 {
		t.Errorf("rate_points rows = %d, want 1", n)
	}
}

// TestSaveBatch_RequiresTenant: no tenant context, no write.
func TestSaveBatch_RequiresTenant(t *testing.T) {
	repo := NewRepository(sharedPool)
	err := repo.SaveBatch(context.Background(), []domain.RateDay{
		rateDay("", newRoomType(t), time.Now(), 100),
	})
	if err == nil {
		t.Fatal("SaveBatch with no tenant context succeeded, want error")
	}
}

// TestSaveBatch_RejectsCrossOrgBatch: a batch may never span organizations.
// The old code took org_id off each row, so a caller could write anywhere.
func TestSaveBatch_RejectsCrossOrgBatch(t *testing.T) {
	repo := NewRepository(sharedPool)
	orgA, orgB := newOrg(t, sharedPool), newOrg(t, sharedPool)
	cleanupRates(t, orgA)
	day := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)

	err := repo.SaveBatch(ctxFor(orgA), []domain.RateDay{
		rateDay(orgA, newRoomType(t), day, 100),
		rateDay(orgB, newRoomType(t), day, 100), // smuggled in
	})
	if !errors.Is(err, ErrCrossOrgBatch) {
		t.Fatalf("SaveBatch cross-org = %v, want ErrCrossOrgBatch", err)
	}

	// And nothing was written -- the check precedes the transaction.
	var n int
	_ = sharedPool.WithTenant(context.Background(), orgA, func(ctx context.Context, tx pgx.Tx) error {
		return tx.QueryRow(ctx,
			"SELECT count(*) FROM pricing.rate_points WHERE org_id = $1::uuid", orgA).Scan(&n)
	})
	if n != 0 {
		t.Errorf("rate_points rows = %d after rejected batch, want 0", n)
	}
}

// TestSaveBatch_RatePointReferencesRealPlan is the FK regression. On the second
// call the BAR plan already exists; the old code did ON CONFLICT DO NOTHING and
// then pointed rate_points at the caller's RatePlanID, which was never inserted.
func TestSaveBatch_RatePointReferencesRealPlan(t *testing.T) {
	repo := NewRepository(sharedPool)
	org := newOrg(t, sharedPool)
	cleanupRates(t, org)
	rt := newRoomType(t)
	d1 := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	d2 := time.Date(2026, 8, 2, 0, 0, 0, 0, time.UTC)

	if err := repo.SaveBatch(ctxFor(org), []domain.RateDay{rateDay(org, rt, d1, 100)}); err != nil {
		t.Fatalf("first SaveBatch: %v", err)
	}
	// Second call: the plan now exists. This is where the FK used to break.
	if err := repo.SaveBatch(ctxFor(org), []domain.RateDay{rateDay(org, rt, d2, 120)}); err != nil {
		t.Fatalf("second SaveBatch: %v", err)
	}

	// Every rate_point must join to a real plan, and only one BAR plan exists.
	var joined, plans int
	if err := sharedPool.WithTenant(context.Background(), org, func(ctx context.Context, tx pgx.Tx) error {
		if err := tx.QueryRow(ctx, `
			SELECT count(*) FROM pricing.rate_points p
			JOIN pricing.rate_plans rp ON rp.id = p.rate_plan_id
			WHERE p.org_id = $1::uuid`, org).Scan(&joined); err != nil {
			return err
		}
		return tx.QueryRow(ctx,
			"SELECT count(*) FROM pricing.rate_plans WHERE org_id = $1::uuid", org).Scan(&plans)
	}); err != nil {
		t.Fatalf("query: %v", err)
	}
	if joined != 2 {
		t.Errorf("rate_points joined to a plan = %d, want 2", joined)
	}
	if plans != 1 {
		t.Errorf("rate_plans = %d, want 1 (BAR is upserted, not duplicated)", plans)
	}
}

// TestSaveBatch_RoundsRatherThanTruncates. int64(0.29*100) == 28, because
// 0.29*100 is 28.999999999999996 in float64 and the conversion truncates.
func TestSaveBatch_RoundsRatherThanTruncates(t *testing.T) {
	repo := NewRepository(sharedPool)
	org := newOrg(t, sharedPool)
	cleanupRates(t, org)
	rt := newRoomType(t)

	cases := []struct {
		rate float64
		want int64
	}{
		{0.29, 29},      // truncation gives 28
		{8.87, 887},     // truncation gives 886
		{10.07, 1007},   // exact in float64; a control
		{199.99, 19999}, // exact; a control
	}
	base := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	days := make([]domain.RateDay, len(cases))
	for i, c := range cases {
		days[i] = rateDay(org, rt, base.AddDate(0, 0, i), c.rate)
	}
	if err := repo.SaveBatch(ctxFor(org), days); err != nil {
		t.Fatalf("SaveBatch: %v", err)
	}

	for i, c := range cases {
		var got int64
		if err := sharedPool.WithTenant(context.Background(), org, func(ctx context.Context, tx pgx.Tx) error {
			return tx.QueryRow(ctx,
				"SELECT amount_minor FROM pricing.rate_points WHERE org_id = $1::uuid AND stay_date = $2",
				org, base.AddDate(0, 0, i)).Scan(&got)
		}); err != nil {
			t.Fatalf("query %v: %v", c.rate, err)
		}
		if got != c.want {
			t.Errorf("BaseRate %v -> amount_minor %d, want %d", c.rate, got, c.want)
		}
	}
}
