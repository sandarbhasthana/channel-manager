//go:build integration

// Integration tests for the booking-engine repository against a real Postgres.
// Requires migrations for tenancy, pms, and reservations applied, and a
// NON-SUPERUSER role (RLS scoping is part of what is under test).
//
//	TEST_DATABASE_URL='postgres://app:app_dev@127.0.0.1:5433/promo_test?sslmode=disable' \
//	  go test -race -tags=integration ./services/bookingengine/adapters/postgres/...
package postgres

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	platformauth "github.com/channel-manager/channel-manager/platform/auth"
	platformdb "github.com/channel-manager/channel-manager/platform/db"
	"github.com/channel-manager/channel-manager/services/bookingengine/domain"
)

var sharedPool *platformdb.Pool

func TestMain(m *testing.M) {
	raw := os.Getenv("TEST_DATABASE_URL")
	if raw == "" {
		fmt.Fprintln(os.Stderr, "TEST_DATABASE_URL not set; skipping")
		os.Exit(0)
	}
	u, err := url.Parse(raw)
	must(err)
	pw, _ := u.User.Password()
	port, _ := strconv.Atoi(u.Port())
	if port == 0 {
		port = 5432
	}
	sharedPool, err = platformdb.NewPool(context.Background(), platformdb.Config{
		Host: u.Hostname(), Port: port, DBName: strings.TrimPrefix(u.Path, "/"),
		User: u.User.Username(), Password: pw, SSLMode: "disable",
	})
	must(err)
	code := m.Run()
	sharedPool.Close()
	os.Exit(code)
}

func must(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func ctxFor(org string) context.Context {
	return platformauth.WithTenantContext(context.Background(),
		platformauth.TenantContext{UserID: "test", OrgID: org, Role: "owner"})
}

func uuidv4(t *testing.T) string {
	t.Helper()
	var s string
	if err := sharedPool.Inner().QueryRow(context.Background(), "SELECT gen_random_uuid()::text").Scan(&s); err != nil {
		t.Fatalf("uuid: %v", err)
	}
	return s
}

// newOrg creates a fresh org row (organizations is the tenant root; GetSettings
// joins it) and purges its rows on cleanup. The property/reservation deletes go
// through WithTenant so RLS lets them see the rows; the org row is not
// RLS-scoped.
func newOrg(t *testing.T) string {
	t.Helper()
	org := uuidv4(t)
	if _, err := sharedPool.Inner().Exec(context.Background(),
		"INSERT INTO tenancy.organizations (id, name, slug) VALUES ($1::uuid, 'test', $2)",
		org, "slug-"+org); err != nil {
		t.Fatalf("seed org: %v", err)
	}
	t.Cleanup(func() {
		_ = sharedPool.WithTenant(context.Background(), org, func(ctx context.Context, tx pgx.Tx) error {
			tx.Exec(ctx, "DELETE FROM reservations.reservations WHERE org_id=$1::uuid", org)
			tx.Exec(ctx, "DELETE FROM reservations.guests WHERE org_id=$1::uuid", org)
			tx.Exec(ctx, "DELETE FROM pms.properties WHERE org_id=$1::uuid", org)
			return nil
		})
		_, _ = sharedPool.Inner().Exec(context.Background(),
			"DELETE FROM tenancy.organizations WHERE id=$1::uuid", org)
	})
	return org
}

func seedProperty(t *testing.T, org string) string {
	t.Helper()
	var id string
	err := sharedPool.WithTenant(ctxFor(org), org, func(ctx context.Context, tx pgx.Tx) error {
		return tx.QueryRow(ctx, `
			INSERT INTO pms.properties (org_id, name, currency)
			VALUES ($1::uuid, 'BE Test', 'USD') RETURNING id::text`, org).Scan(&id)
	})
	if err != nil {
		t.Fatalf("seed property: %v", err)
	}
	return id
}

// seedReservation inserts a reservation with the given source label and booked_at.
func seedReservation(t *testing.T, org, propID, source, guestFirst, guestLast string, bookedAt time.Time) string {
	t.Helper()
	var resID string
	err := sharedPool.WithTenant(ctxFor(org), org, func(ctx context.Context, tx pgx.Tx) error {
		var guestID string
		if err := tx.QueryRow(ctx, `
			INSERT INTO reservations.guests (org_id, first_name, last_name)
			VALUES ($1::uuid, $2, $3) RETURNING id::text`, org, guestFirst, guestLast).Scan(&guestID); err != nil {
			return err
		}
		return tx.QueryRow(ctx, `
			INSERT INTO reservations.reservations
			  (org_id, property_id, primary_guest_id, confirmation_code, status,
			   check_in, check_out, adults, currency, total_amount_minor, metadata, booked_at)
			VALUES ($1::uuid, $2::uuid, $3::uuid, 'CONF123', 'confirmed',
			        '2026-09-01', '2026-09-03', 2, 'USD', 24000,
			        jsonb_build_object('source', $4::text), $5)
			RETURNING id::text`, org, propID, guestID, source, bookedAt).Scan(&resID)
	})
	if err != nil {
		t.Fatalf("seed reservation: %v", err)
	}
	return resID
}

func TestListDirectReservations_OnlyDirectAndOrgScoped(t *testing.T) {
	repo := NewRepository(sharedPool)
	orgA, orgB := newOrg(t), newOrg(t)
	propA := seedProperty(t, orgA)
	propB := seedProperty(t, orgB)
	now := time.Now().UTC()

	// orgA: one direct, one OTA (should be excluded)
	direct := seedReservation(t, orgA, propA, "direct", "Ada", "Lovelace", now)
	seedReservation(t, orgA, propA, "booking.com", "Otto", "Ota", now)
	// orgB: a direct booking that must never appear for orgA
	seedReservation(t, orgB, propB, "direct", "Bob", "Other", now)

	got, err := repo.ListDirectReservations(ctxFor(orgA), propA, 50, 0)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d reservations, want 1 (direct only, org-scoped)", len(got))
	}
	r := got[0]
	if r.ID != direct {
		t.Errorf("id = %s, want %s", r.ID, direct)
	}
	if r.GuestName != "Ada Lovelace" {
		t.Errorf("guest = %q, want \"Ada Lovelace\"", r.GuestName)
	}
	if r.ConfirmationCode != "CONF123" || r.TotalMinor != 24000 || r.Currency != "USD" {
		t.Errorf("unexpected fields: %+v", r)
	}
}

func TestListDirectReservations_OrderedAndPaged(t *testing.T) {
	repo := NewRepository(sharedPool)
	org := newOrg(t)
	prop := seedProperty(t, org)
	base := time.Now().UTC()
	// three bookings, ascending time; expect descending (newest first)
	seedReservation(t, org, prop, "direct", "A", "One", base.Add(1*time.Hour))
	seedReservation(t, org, prop, "direct", "B", "Two", base.Add(2*time.Hour))
	newest := seedReservation(t, org, prop, "direct", "C", "Three", base.Add(3*time.Hour))

	page1, err := repo.ListDirectReservations(ctxFor(org), prop, 2, 0)
	if err != nil {
		t.Fatalf("page1: %v", err)
	}
	if len(page1) != 2 || page1[0].ID != newest {
		t.Fatalf("page1 = %d rows, first=%s; want 2 rows newest-first (%s)", len(page1), first(page1), newest)
	}
	page2, err := repo.ListDirectReservations(ctxFor(org), prop, 2, 2)
	if err != nil {
		t.Fatalf("page2: %v", err)
	}
	if len(page2) != 1 {
		t.Errorf("page2 = %d rows, want 1", len(page2))
	}
}

func first(rs []domain.DirectReservation) string {
	if len(rs) == 0 {
		return ""
	}
	return rs[0].ID
}

func TestSettings_DefaultsAndUpdate(t *testing.T) {
	repo := NewRepository(sharedPool)
	org := newOrg(t)
	prop := seedProperty(t, org)

	s, err := repo.GetSettings(ctxFor(org), prop)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if !s.DirectChannelEnabled || s.Route != "pms" || s.Percent != 0 {
		t.Errorf("defaults = %+v, want enabled/pms/0", s)
	}

	// Edit all three from the dashboard.
	s, err = repo.UpdateSettings(ctxFor(org), domain.Settings{
		PropertyID: prop, DirectChannelEnabled: false, Route: "cm", Percent: 25,
	})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if s.DirectChannelEnabled || s.Route != "cm" || s.Percent != 25 {
		t.Errorf("after update = %+v, want disabled/cm/25", s)
	}
	// persisted?
	s, _ = repo.GetSettings(ctxFor(org), prop)
	if s.DirectChannelEnabled || s.Route != "cm" || s.Percent != 25 {
		t.Errorf("update did not persist: %+v", s)
	}
}

func TestSettings_UnknownPropertyNotFound(t *testing.T) {
	repo := NewRepository(sharedPool)
	org := newOrg(t)
	if _, err := repo.GetSettings(ctxFor(org), uuidv4(t)); !errors.Is(err, domain.ErrPropertyNotFound) {
		t.Errorf("GetSettings unknown = %v, want ErrPropertyNotFound", err)
	}
	if _, err := repo.UpdateSettings(ctxFor(org), domain.Settings{
		PropertyID: uuidv4(t), Route: "pms",
	}); !errors.Is(err, domain.ErrPropertyNotFound) {
		t.Errorf("UpdateSettings unknown = %v, want ErrPropertyNotFound", err)
	}
}

// A property in another org must be invisible even by id (RLS + explicit org_id).
func TestSettings_OtherOrgPropertyNotFound(t *testing.T) {
	repo := NewRepository(sharedPool)
	orgA, orgB := newOrg(t), newOrg(t)
	propB := seedProperty(t, orgB)
	if _, err := repo.GetSettings(ctxFor(orgA), propB); !errors.Is(err, domain.ErrPropertyNotFound) {
		t.Errorf("cross-org GetSettings = %v, want ErrPropertyNotFound", err)
	}
}

