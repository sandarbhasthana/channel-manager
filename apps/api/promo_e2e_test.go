//go:build integration

// B7: the booking engine -> channel manager promo path, end to end.
//
// This is the first BE->CM call in the codebase and, before this test, it had
// only ever typechecked. It mounts the real storefront ingress -- real
// PromoRepository over a real PostgreSQL, real property lookup, real
// integration-key middleware -- and drives it with the booking engine's actual
// client, apps/api/src/cm.ts, compiled and run under node.
//
// Nothing here is a fake except the ports the promo actions never touch
// (PMS gateway, reservations, holds, idempotency, audit), which are nil.
//
//	TEST_DATABASE_URL='postgres://app:app_dev@127.0.0.1:5433/promo_test?sslmode=disable' \
//	BE_REPO=/path/to/aura-hospitality \
//	  go test -tags=integration -run TestB7 ./apps/api/
package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"

	platformauth "github.com/channel-manager/channel-manager/platform/auth"
	platformdb "github.com/channel-manager/channel-manager/platform/db"
	platformintegration "github.com/channel-manager/channel-manager/platform/integration"
	pmspostgres "github.com/channel-manager/channel-manager/services/pms/adapters/postgres"
	pricingdomain "github.com/channel-manager/channel-manager/services/pricing/domain"
	pricingpostgres "github.com/channel-manager/channel-manager/services/pricing/adapters/postgres"
	pricingusecases "github.com/channel-manager/channel-manager/services/pricing/usecases"
	storefronthttp "github.com/channel-manager/channel-manager/services/storefront/adapters/http"
	storefrontusecases "github.com/channel-manager/channel-manager/services/storefront/usecases"
)

const (
	b7OrgID = "3f1d9c2e-0000-4000-8000-000000000001"
	b7Token = "dev-pms-integration-token"
)

func b7Pool(t *testing.T) *platformdb.Pool {
	t.Helper()
	raw := os.Getenv("TEST_DATABASE_URL")
	if raw == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("parse TEST_DATABASE_URL: %v", err)
	}
	pw, _ := u.User.Password()
	port, _ := strconv.Atoi(u.Port())
	pool, err := platformdb.NewPool(context.Background(), platformdb.Config{
		Host: u.Hostname(), Port: port, DBName: strings.TrimPrefix(u.Path, "/"),
		User: u.User.Username(), Password: pw, SSLMode: "disable",
	})
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

// b7Server mounts the storefront ingress exactly as apps/api/main.go does for
// the promo actions: same handler, same intAuth middleware, same route.
func b7Server(t *testing.T, pool *platformdb.Pool) *httptest.Server {
	t.Helper()

	promoRepo := pricingpostgres.NewPromoRepository(pool)
	promoSvc := pricingusecases.NewPromoService(promoRepo, nil)
	propRepo := pmspostgres.NewPropertyRepository(pool)

	// The promo actions touch only props + promos. Everything else stays nil so
	// that a test failure cannot be an artefact of a stubbed collaborator.
	sfSvc := storefrontusecases.NewService(propRepo, nil, nil, promoSvc, nil, nil, nil, 0)
	sfHandler := storefronthttp.NewHandler(sfSvc)

	secrets, err := platformintegration.LoadEnvSecretsFromJSON(
		`{"` + b7OrgID + `":"` + b7Token + `"}`)
	if err != nil {
		t.Fatalf("load secrets: %v", err)
	}
	intAuth := platformintegration.NewAuthenticator(secrets, platformintegration.NewKeyStore(pool))

	mux := http.NewServeMux()
	mux.Handle("POST /api/storefront/v1/{propertyId}", intAuth.Middleware(http.HandlerFunc(sfHandler.Dispatch)))

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

// b7Seed creates the org's property and a max_uses=1 promo, returning the
// property id. Seeded through the repositories' own tenant context so RLS
// applies exactly as it does at runtime.
func b7Seed(t *testing.T, pool *platformdb.Pool, code string, maxUses int) string {
	t.Helper()
	ctx := platformauth.WithTenantContext(context.Background(),
		platformauth.TenantContext{UserID: "b7", OrgID: b7OrgID, Role: "owner"})
	b7Purge(pool)

	var propID string
	err := pool.WithTenant(ctx, b7OrgID, func(ctx context.Context, tx pgx.Tx) error {
		return tx.QueryRow(ctx, `
			INSERT INTO pms.properties (org_id, name, currency, external_id)
			VALUES ($1::uuid, 'B7 Test Property', 'USD', 'b7-ext')
			RETURNING id::text`, b7OrgID).Scan(&propID)
	})
	if err != nil {
		t.Fatalf("seed property: %v", err)
	}
	// Cleanup must go through WithTenant. A bare pool.Inner() DELETE runs with
	// no app.current_org_id, so RLS filters it to zero rows and it silently
	// cleans up nothing.
	t.Cleanup(func() { b7Purge(pool) })

	repo := pricingpostgres.NewPromoRepository(pool)
	if _, err := repo.Create(ctx, pricingdomain.PromoCode{
		Code: code, DiscountPct: 15, MaxUses: &maxUses, IsActive: true,
	}); err != nil {
		t.Fatalf("seed promo: %v", err)
	}
	return propID
}

// b7Purge removes this test's rows. Also run before seeding, so a previous
// crashed run cannot fail the next one on the UNIQUE (org_id, code) index.
func b7Purge(pool *platformdb.Pool) {
	ctx := context.Background()
	_ = pool.WithTenant(ctx, b7OrgID, func(ctx context.Context, tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, "DELETE FROM pricing.promo_codes WHERE org_id = $1::uuid", b7OrgID); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, "DELETE FROM pms.properties WHERE org_id = $1::uuid", b7OrgID)
		return err
	})
}

// TestB7_BookingEngineToChannelManagerPromoPath drives CM through the booking
// engine's own cm.ts client: getPromo -> redeemPromo -> redeemPromo again.
//
// The second redeem must surface as CmError{status: 409}, because that is the
// contract checkout.ts depends on to decide "honour the booking, reconcile the
// promo" rather than failing a guest who has already paid.
func TestB7_BookingEngineToChannelManagerPromoPath(t *testing.T) {
	beRepo := os.Getenv("BE_REPO")
	if beRepo == "" {
		t.Skip("BE_REPO not set (path to aura-hospitality)")
	}
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node not on PATH")
	}

	pool := b7Pool(t)
	srv := b7Server(t, pool)
	propID := b7Seed(t, pool, "SUMMER25", 1)

	// Compile the booking engine's real client. cm.ts has no imports, so tsc
	// can emit it standalone.
	work := t.TempDir()
	tsc := filepath.Join(beRepo, "node_modules", ".bin", "tsc")
	cmSrc := filepath.Join(beRepo, "apps", "api", "src", "cm.ts")
	out, err := exec.Command(tsc, cmSrc,
		"--outDir", work, "--module", "esnext", "--target", "es2022",
		"--moduleResolution", "bundler", "--skipLibCheck",
		// cm.ts reads process.env; @types/node lives at the BE repo root.
		"--typeRoots", filepath.Join(beRepo, "node_modules", "@types"),
		"--types", "node").CombinedOutput()
	if err != nil {
		t.Fatalf("compile cm.ts: %v\n%s", err, out)
	}
	if err := os.Rename(filepath.Join(work, "cm.js"), filepath.Join(work, "cm.mjs")); err != nil {
		t.Fatalf("rename: %v", err)
	}

	driver := `
import { getPromo, redeemPromo, CmError } from './cm.mjs';
const prop = process.env.B7_PROPERTY_ID;
const steps = {};
try {
  steps.lookup = await getPromo(prop, 'SUMMER25');
  steps.lookup_lowercase = await getPromo(prop, 'summer25');
  steps.redeem = await redeemPromo(prop, 'SUMMER25', 'res-b7-1');
  try {
    await redeemPromo(prop, 'SUMMER25', 'res-b7-2');
    steps.second_redeem = { threw: false };
  } catch (e) {
    steps.second_redeem = {
      threw: true, name: e.name, status: e.status,
      isRefusal: e instanceof CmError ? e.isRefusal : null,
    };
  }
  steps.lookup_after = await getPromo(prop, 'SUMMER25');
  try {
    await getPromo(prop, 'NO_SUCH_CODE');
    steps.unknown = { threw: false };
  } catch (e) {
    steps.unknown = { threw: true, status: e.status, isNotFound: e.isNotFound };
  }
} catch (e) {
  console.error('DRIVER ERROR', e);
  process.exit(1);
}
console.log(JSON.stringify(steps));
`
	if err := os.WriteFile(filepath.Join(work, "driver.mjs"), []byte(driver), 0o644); err != nil {
		t.Fatalf("write driver: %v", err)
	}

	cmd := exec.Command(node, filepath.Join(work, "driver.mjs"))
	cmd.Env = append(os.Environ(),
		"CM_API_URL="+srv.URL,
		"CM_API_KEY="+b7Token,
		"B7_PROPERTY_ID="+propID,
	)
	stdout, err := cmd.Output()
	if err != nil {
		t.Fatalf("run booking-engine client: %v\nstderr: %s", err, exitStderr(err))
	}

	var steps struct {
		Lookup struct {
			Code        string  `json:"code"`
			DiscountPct float64 `json:"discount_pct"`
			Valid       bool    `json:"valid"`
		} `json:"lookup"`
		LookupLowercase struct {
			Code  string `json:"code"`
			Valid bool   `json:"valid"`
		} `json:"lookup_lowercase"`
		Redeem struct {
			Code        string  `json:"code"`
			DiscountPct float64 `json:"discount_pct"`
			Uses        int     `json:"uses"`
			Redeemed    bool    `json:"redeemed"`
			MaxUses     int     `json:"max_uses"`
			Remaining   int     `json:"remaining"`
		} `json:"redeem"`
		SecondRedeem struct {
			Threw     bool   `json:"threw"`
			Name      string `json:"name"`
			Status    int    `json:"status"`
			IsRefusal bool   `json:"isRefusal"`
		} `json:"second_redeem"`
		LookupAfter struct {
			Valid       bool    `json:"valid"`
			Reason      string  `json:"reason"`
			DiscountPct float64 `json:"discount_pct"`
		} `json:"lookup_after"`
		Unknown struct {
			Threw      bool `json:"threw"`
			Status     int  `json:"status"`
			IsNotFound bool `json:"isNotFound"`
		} `json:"unknown"`
	}
	if err := json.Unmarshal(stdout, &steps); err != nil {
		t.Fatalf("parse driver output: %v\nraw: %s", err, stdout)
	}

	// get_promo reads without consuming.
	if !steps.Lookup.Valid || steps.Lookup.DiscountPct != 15 {
		t.Errorf("lookup = %+v, want valid 15%%", steps.Lookup)
	}
	// normalizeCode: the guest may type it in any case.
	if !steps.LookupLowercase.Valid || steps.LookupLowercase.Code != "SUMMER25" {
		t.Errorf("lowercase lookup = %+v, want valid SUMMER25", steps.LookupLowercase)
	}
	// redeem_promo consumes exactly one.
	if !steps.Redeem.Redeemed || steps.Redeem.Uses != 1 || steps.Redeem.Remaining != 0 {
		t.Errorf("redeem = %+v, want redeemed uses=1 remaining=0", steps.Redeem)
	}
	// The contract checkout.ts relies on: exhausted is a 409 refusal, not a 500.
	if !steps.SecondRedeem.Threw || steps.SecondRedeem.Status != 409 || !steps.SecondRedeem.IsRefusal {
		t.Errorf("second redeem = %+v, want CmError{status:409, isRefusal:true}", steps.SecondRedeem)
	}
	if steps.SecondRedeem.Name != "CmError" {
		t.Errorf("second redeem error name = %q, want CmError", steps.SecondRedeem.Name)
	}
	// An exhausted code must price at zero, not advertise a discount.
	if steps.LookupAfter.Valid || steps.LookupAfter.Reason != "exhausted" || steps.LookupAfter.DiscountPct != 0 {
		t.Errorf("lookup after exhaustion = %+v, want invalid/exhausted/0%%", steps.LookupAfter)
	}
	// Unknown code is the only 404.
	if !steps.Unknown.Threw || steps.Unknown.Status != 404 || !steps.Unknown.IsNotFound {
		t.Errorf("unknown code = %+v, want CmError{status:404}", steps.Unknown)
	}
}

func exitStderr(err error) []byte {
	if ee, ok := err.(*exec.ExitError); ok {
		return ee.Stderr
	}
	return nil
}
