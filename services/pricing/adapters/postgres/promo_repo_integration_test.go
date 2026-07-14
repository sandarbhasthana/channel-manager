//go:build integration

// Integration tests for PromoRepository against a real PostgreSQL.
//
//	createdb promo_test
//	TEST_DATABASE_URL='postgres://app:app@localhost:5432/promo_test?sslmode=disable' \
//	  go test -race -tags=integration ./services/pricing/adapters/postgres/...
//
// The migrations in migrations/tenancy and migrations/pricing must already be
// applied; tenancy/0001 and tenancy/0004 provision the `app` role these tests
// bind as.
//
// IMPORTANT: run these as `app`, or any other NON-SUPERUSER role -- which is
// what apps/api binds at runtime (cfg.DB.RuntimeUser). The repository omits
// org_id from its WHERE clauses and relies entirely on RLS for tenant scoping;
// PostgreSQL exempts superusers from RLS even under FORCE ROW LEVEL SECURITY,
// so a superuser connection makes TestRLS_* pass vacuously and lets Redeem
// mutate other orgs' rows. See TestRedeem_DoesNotTouchOtherOrgs.
package postgres

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	platformauth "github.com/channel-manager/channel-manager/platform/auth"
	platformdb "github.com/channel-manager/channel-manager/platform/db"
	"github.com/channel-manager/channel-manager/services/pricing/domain"
	pricingusecases "github.com/channel-manager/channel-manager/services/pricing/usecases"
)

// sharedPool is opened once in TestMain and shared by every test. A pool per
// test would serialise the concurrency tests behind connection setup.
var sharedPool *platformdb.Pool

func TestMain(m *testing.M) {
	raw := os.Getenv("TEST_DATABASE_URL")
	if raw == "" {
		fmt.Fprintln(os.Stderr, "TEST_DATABASE_URL not set; skipping integration tests")
		os.Exit(0)
	}
	u, err := url.Parse(raw)
	if err != nil {
		fmt.Fprintf(os.Stderr, "parse TEST_DATABASE_URL: %v\n", err)
		os.Exit(1)
	}
	pw, _ := u.User.Password()
	port, _ := strconv.Atoi(u.Port())
	if port == 0 {
		port = 5432
	}
	ssl := u.Query().Get("sslmode")
	if ssl == "" {
		ssl = "disable"
	}
	sharedPool, err = platformdb.NewPool(context.Background(), platformdb.Config{
		Host:     u.Hostname(),
		Port:     port,
		DBName:   strings.TrimPrefix(u.Path, "/"),
		User:     u.User.Username(),
		Password: pw,
		SSLMode:  ssl,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "connect: %v\n", err)
		os.Exit(1)
	}
	code := m.Run()
	sharedPool.Close()
	os.Exit(code)
}

// ctxFor returns a context carrying org as the authenticated tenant.
func ctxFor(org string) context.Context {
	return platformauth.WithTenantContext(context.Background(),
		platformauth.TenantContext{UserID: "test-user", OrgID: org, Role: "owner"})
}

// newOrg returns a fresh org id, and cleans up its rows afterwards.
//
// The DELETE goes through WithTenant: a bare pool.Inner() exec carries no
// app.current_org_id, so RLS filters it to zero rows and cleans up nothing.
// Harmless here because each test gets a fresh org, but it would silently rot.
func newOrg(t *testing.T, pool *platformdb.Pool) string {
	t.Helper()
	org := uuidv4(t)
	t.Cleanup(func() {
		_ = pool.WithTenant(context.Background(), org, func(ctx context.Context, tx pgx.Tx) error {
			_, err := tx.Exec(ctx, "DELETE FROM pricing.promo_codes WHERE org_id = $1::uuid", org)
			return err
		})
	})
	return org
}

func uuidv4(t *testing.T) string {
	t.Helper()
	var s string
	if err := sharedPool.Inner().QueryRow(context.Background(),
		"SELECT gen_random_uuid()::text").Scan(&s); err != nil {
		t.Fatalf("uuid: %v", err)
	}
	return s
}

func mustCreate(t *testing.T, r *PromoRepository, org string, p domain.PromoCode) domain.PromoCode {
	t.Helper()
	out, err := r.Create(ctxFor(org), p)
	if err != nil {
		t.Fatalf("create %q: %v", p.Code, err)
	}
	return out
}

func intp(i int) *int { return &i }

// --- B6: the guarantee ------------------------------------------------------

// TestRedeem_ConcurrentMaxUsesOne is the test that matters: N goroutines redeem
// a max_uses = 1 code at once. Exactly one must succeed; the rest must get
// ErrPromoExhausted, and uses must land on exactly 1.
func TestRedeem_ConcurrentMaxUsesOne(t *testing.T) {
	pool := sharedPool
	repo := NewPromoRepository(pool)
	org := newOrg(t, pool)
	mustCreate(t, repo, org, domain.PromoCode{
		Code: "RACE1", DiscountPct: 10, MaxUses: intp(1), IsActive: true,
	})

	const n = 20
	var (
		start sync.WaitGroup
		done  sync.WaitGroup
		mu    sync.Mutex
		wins  int
		errs  []error
	)
	start.Add(1)
	for i := 0; i < n; i++ {
		done.Add(1)
		go func() {
			defer done.Done()
			start.Wait() // release all goroutines at once
			_, err := repo.Redeem(ctxFor(org), "RACE1", "", time.Now())
			mu.Lock()
			defer mu.Unlock()
			if err == nil {
				wins++
			} else {
				errs = append(errs, err)
			}
		}()
	}
	start.Done()
	done.Wait()

	if wins != 1 {
		t.Errorf("winners = %d, want exactly 1", wins)
	}
	for _, err := range errs {
		if !errors.Is(err, domain.ErrPromoExhausted) {
			t.Errorf("loser error = %v, want ErrPromoExhausted", err)
		}
	}
	if got := usesOf(t, repo, org, "RACE1"); got != 1 {
		t.Errorf("uses = %d, want 1", got)
	}
}

// TestRedeem_ConcurrentMaxUsesN generalises: exactly max_uses redemptions win.
func TestRedeem_ConcurrentMaxUsesN(t *testing.T) {
	pool := sharedPool
	repo := NewPromoRepository(pool)
	org := newOrg(t, pool)
	const limit, n = 5, 20
	mustCreate(t, repo, org, domain.PromoCode{
		Code: "RACE5", DiscountPct: 10, MaxUses: intp(limit), IsActive: true,
	})

	var start, done sync.WaitGroup
	var mu sync.Mutex
	wins := 0
	start.Add(1)
	for i := 0; i < n; i++ {
		done.Add(1)
		go func() {
			defer done.Done()
			start.Wait()
			if _, err := repo.Redeem(ctxFor(org), "RACE5", "", time.Now()); err == nil {
				mu.Lock()
				wins++
				mu.Unlock()
			}
		}()
	}
	start.Done()
	done.Wait()

	if wins != limit {
		t.Errorf("winners = %d, want %d", wins, limit)
	}
	if got := usesOf(t, repo, org, "RACE5"); got != limit {
		t.Errorf("uses = %d, want %d", got, limit)
	}
}

func usesOf(t *testing.T, r *PromoRepository, org, code string) int {
	t.Helper()
	p, err := r.GetByCode(ctxFor(org), code)
	if err != nil {
		t.Fatalf("get %q: %v", code, err)
	}
	return p.Uses
}

// --- B6: rejection reasons --------------------------------------------------

func TestRedeem_RejectionReasons(t *testing.T) {
	pool := sharedPool
	repo := NewPromoRepository(pool)
	org := newOrg(t, pool)
	now := time.Now()
	otherProp := uuidv4(t)

	mustCreate(t, repo, org, domain.PromoCode{Code: "INACTIVE", DiscountPct: 10, IsActive: false})
	mustCreate(t, repo, org, domain.PromoCode{Code: "EXPIRED", DiscountPct: 10, IsActive: true,
		ValidUntil: ptime(now.Add(-time.Hour))})
	mustCreate(t, repo, org, domain.PromoCode{Code: "FUTURE", DiscountPct: 10, IsActive: true,
		ValidFrom: ptime(now.Add(time.Hour))})
	mustCreate(t, repo, org, domain.PromoCode{Code: "SCOPED", DiscountPct: 10, IsActive: true,
		PropertyID: otherProp})

	tests := []struct {
		code, prop string
		want       error
	}{
		{"MISSING", "", domain.ErrPromoNotFound},
		{"INACTIVE", "", domain.ErrPromoInactive},
		{"EXPIRED", "", domain.ErrPromoExpired},
		{"FUTURE", "", domain.ErrPromoNotYetValid},
		{"SCOPED", uuidv4(t), domain.ErrPromoWrongScope},
	}
	for _, tc := range tests {
		if _, err := repo.Redeem(ctxFor(org), tc.code, tc.prop, now); !errors.Is(err, tc.want) {
			t.Errorf("Redeem(%q) = %v, want %v", tc.code, err, tc.want)
		}
	}
}

func ptime(t time.Time) *time.Time { return &t }

// --- coupons: PromoService CRUD (dashboard path) ----------------------------

// TestPromoService_CRUD exercises the create/list/update/delete flow the
// dashboard coupons manager drives, through the real service + repo, org-scoped.
func TestPromoService_CRUD(t *testing.T) {
	svc := pricingusecases.NewPromoService(NewPromoRepository(sharedPool), nil)
	org := newOrg(t, sharedPool)
	ctx := ctxFor(org)

	created, err := svc.CreatePromo(ctx, domain.PromoCode{
		Code: "spring25", DiscountPct: 20, MaxUses: intp(50), IsActive: true,
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	// normalizeCode upper-cases on write.
	if created.Code != "SPRING25" {
		t.Errorf("created code = %q, want SPRING25", created.Code)
	}

	list, err := svc.ListPromos(ctx)
	if err != nil || len(list) != 1 || list[0].ID != created.ID {
		t.Fatalf("list = %v (err %v), want the one created code", list, err)
	}

	created.DiscountPct = 25
	created.IsActive = false
	updated, err := svc.UpdatePromo(ctx, created)
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if updated.DiscountPct != 25 || updated.IsActive {
		t.Errorf("update = %+v, want 25%% inactive", updated)
	}

	if err := svc.DeletePromo(ctx, created.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if list, _ := svc.ListPromos(ctx); len(list) != 0 {
		t.Errorf("after delete, list has %d, want 0", len(list))
	}
}

// Coupons are org-scoped: one org never sees another's.
func TestPromoService_ListOrgScoped(t *testing.T) {
	svc := pricingusecases.NewPromoService(NewPromoRepository(sharedPool), nil)
	orgA, orgB := newOrg(t, sharedPool), newOrg(t, sharedPool)
	if _, err := svc.CreatePromo(ctxFor(orgA), domain.PromoCode{Code: "AONLY", DiscountPct: 10, IsActive: true}); err != nil {
		t.Fatalf("create: %v", err)
	}
	if list, _ := svc.ListPromos(ctxFor(orgB)); len(list) != 0 {
		t.Errorf("org B sees %d of org A's coupons, want 0", len(list))
	}
}

// --- B6: NUMERIC(5,2) -> float64 -------------------------------------------

// discount_pct is NUMERIC(5,2) in Postgres and float64 in Go. Nothing else
// exercises that scan.
func TestPromo_DiscountPctScan(t *testing.T) {
	pool := sharedPool
	repo := NewPromoRepository(pool)
	org := newOrg(t, pool)

	for i, want := range []float64{0.01, 33.33, 12.5, 99.99, 100} {
		code := "PCT" + strconv.Itoa(i)
		created := mustCreate(t, repo, org, domain.PromoCode{
			Code: code, DiscountPct: want, IsActive: true,
		})
		if created.DiscountPct != want {
			t.Errorf("Create(%v).DiscountPct = %v", want, created.DiscountPct)
		}
		got, err := repo.GetByCode(ctxFor(org), code)
		if err != nil {
			t.Fatalf("get: %v", err)
		}
		if got.DiscountPct != want {
			t.Errorf("GetByCode(%s).DiscountPct = %v, want %v", code, got.DiscountPct, want)
		}
	}
}

// --- B6: constraints --------------------------------------------------------

func TestPromo_ConstraintsRejectBadRows(t *testing.T) {
	pool := sharedPool
	repo := NewPromoRepository(pool)
	org := newOrg(t, pool)
	now := time.Now()

	bad := []struct {
		name string
		p    domain.PromoCode
	}{
		{"discount_pct zero", domain.PromoCode{Code: "B1", DiscountPct: 0}},
		{"discount_pct over 100", domain.PromoCode{Code: "B2", DiscountPct: 100.01}},
		{"max_uses zero", domain.PromoCode{Code: "B3", DiscountPct: 10, MaxUses: intp(0)}},
		{"valid_until before valid_from", domain.PromoCode{Code: "B4", DiscountPct: 10,
			ValidFrom: ptime(now), ValidUntil: ptime(now.Add(-time.Hour))}},
	}
	for _, tc := range bad {
		if _, err := repo.Create(ctxFor(org), tc.p); err == nil {
			t.Errorf("Create(%s) succeeded, want constraint violation", tc.name)
		}
	}

	// UNIQUE (org_id, code): same code twice in one org fails...
	mustCreate(t, repo, org, domain.PromoCode{Code: "DUP", DiscountPct: 10, IsActive: true})
	if _, err := repo.Create(ctxFor(org), domain.PromoCode{Code: "DUP", DiscountPct: 10}); err == nil {
		t.Error("duplicate (org_id, code) succeeded, want unique violation")
	}
	// ...but the same code in another org is fine.
	other := newOrg(t, pool)
	mustCreate(t, repo, other, domain.PromoCode{Code: "DUP", DiscountPct: 10, IsActive: true})
}

// --- B6: RLS ----------------------------------------------------------------

// TestRLS_ReadsAreOrgScoped fails vacuously on a superuser connection.
func TestRLS_ReadsAreOrgScoped(t *testing.T) {
	pool := sharedPool
	repo := NewPromoRepository(pool)
	orgA, orgB := newOrg(t, pool), newOrg(t, pool)

	mustCreate(t, repo, orgA, domain.PromoCode{Code: "ONLY_A", DiscountPct: 10, IsActive: true})

	if _, err := repo.GetByCode(ctxFor(orgB), "ONLY_A"); !errors.Is(err, domain.ErrPromoNotFound) {
		t.Errorf("org B read org A's code: err = %v, want ErrPromoNotFound", err)
	}
	list, err := repo.ListByOrg(ctxFor(orgB))
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list) != 0 {
		t.Errorf("org B sees %d rows, want 0", len(list))
	}
}

// TestRedeem_DoesNotTouchOtherOrgs is the sharp end of the missing org_id
// predicate. Redeem's WHERE clause matches on `code` alone. Under RLS that is
// safe, and the runtime does bind as the unprivileged `app` role. But without
// RLS -- point TEST_DATABASE_URL at `postgres` and watch this fail -- one org's
// redeem burns every org's counter for the same code string. It is a regression
// test on the credential, not just on the query.
func TestRedeem_DoesNotTouchOtherOrgs(t *testing.T) {
	pool := sharedPool
	repo := NewPromoRepository(pool)
	orgA, orgB := newOrg(t, pool), newOrg(t, pool)

	for _, org := range []string{orgA, orgB} {
		mustCreate(t, repo, org, domain.PromoCode{
			Code: "SHARED", DiscountPct: 10, MaxUses: intp(1), IsActive: true,
		})
	}

	if _, err := repo.Redeem(ctxFor(orgA), "SHARED", "", time.Now()); err != nil {
		t.Fatalf("redeem for org A: %v", err)
	}
	if got := usesOf(t, repo, orgA, "SHARED"); got != 1 {
		t.Errorf("org A uses = %d, want 1", got)
	}
	if got := usesOf(t, repo, orgB, "SHARED"); got != 0 {
		t.Errorf("org B uses = %d, want 0 -- org A's redeem burned org B's counter "+
			"(are you connected as a superuser? RLS does not apply)", got)
	}
}

// --- B6: release ------------------------------------------------------------

func TestReleaseRedemption_ClampsAtZero(t *testing.T) {
	pool := sharedPool
	repo := NewPromoRepository(pool)
	org := newOrg(t, pool)
	mustCreate(t, repo, org, domain.PromoCode{
		Code: "REL", DiscountPct: 10, MaxUses: intp(1), IsActive: true,
	})

	if _, err := repo.Redeem(ctxFor(org), "REL", "", time.Now()); err != nil {
		t.Fatalf("redeem: %v", err)
	}
	for i := 0; i < 3; i++ {
		if err := repo.ReleaseRedemption(ctxFor(org), "REL"); err != nil {
			t.Fatalf("release %d: %v", i, err)
		}
	}
	if got := usesOf(t, repo, org, "REL"); got != 0 {
		t.Errorf("uses = %d after 1 redeem + 3 releases, want 0", got)
	}
	// Capacity was not manufactured: the code is redeemable exactly once again.
	if _, err := repo.Redeem(ctxFor(org), "REL", "", time.Now()); err != nil {
		t.Fatalf("redeem after release: %v", err)
	}
	if _, err := repo.Redeem(ctxFor(org), "REL", "", time.Now()); !errors.Is(err, domain.ErrPromoExhausted) {
		t.Errorf("second redeem = %v, want ErrPromoExhausted", err)
	}
}
