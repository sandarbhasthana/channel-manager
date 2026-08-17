package usecases

import (
	"errors"
	"testing"

	pricingdomain "github.com/channel-manager/channel-manager/services/pricing/domain"
	"github.com/channel-manager/channel-manager/services/storefront/domain"
)

// ── get_promo ───────────────────────────────────────────────────────────────

func TestGetPromo_Valid(t *testing.T) {
	h := newHarness()

	out, err := dispatch(t, h, domain.ActionGetPromo, map[string]any{"code": "SUMMER25"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out["valid"] != true {
		t.Error("expected valid=true")
	}
	if out["discount_pct"] != 25.0 {
		t.Errorf("expected 25%% discount, got %v", out["discount_pct"])
	}
	// A read must not consume a redemption.
	if h.promos.redeemCalls != 0 {
		t.Error("get_promo must not redeem")
	}
}

// An ineligible code is a structured refusal, not an error — and it must not
// advertise a discount the guest will never receive.
func TestGetPromo_Ineligible_ReportsReasonAndZeroDiscount(t *testing.T) {
	cases := []struct {
		name       string
		reason     error
		wantReason string
	}{
		{"expired", pricingdomain.ErrPromoExpired, "expired"},
		{"exhausted", pricingdomain.ErrPromoExhausted, "exhausted"},
		{"inactive", pricingdomain.ErrPromoInactive, "inactive"},
		{"not yet valid", pricingdomain.ErrPromoNotYetValid, "not_yet_valid"},
		{"wrong property", pricingdomain.ErrPromoWrongScope, "wrong_property"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness()
			h.promos.lookupReason = tc.reason

			out, err := dispatch(t, h, domain.ActionGetPromo, map[string]any{"code": "SUMMER25"})
			if err != nil {
				t.Fatalf("an ineligible code is not an error: %v", err)
			}
			if out["valid"] != false {
				t.Error("expected valid=false")
			}
			if out["reason"] != tc.wantReason {
				t.Errorf("reason = %v, want %q", out["reason"], tc.wantReason)
			}
			if out["discount_pct"] != 0.0 {
				t.Errorf("an invalid code must offer no discount, got %v", out["discount_pct"])
			}
		})
	}
}

// An unknown code, by contrast, IS an error — the handler maps it to 404.
func TestGetPromo_NotFound_IsError(t *testing.T) {
	h := newHarness()
	h.promos.lookupErr = pricingdomain.ErrPromoNotFound

	_, err := dispatch(t, h, domain.ActionGetPromo, map[string]any{"code": "NOPE"})
	if !errors.Is(err, pricingdomain.ErrPromoNotFound) {
		t.Fatalf("expected ErrPromoNotFound, got %v", err)
	}
}

func TestGetPromo_MissingCode(t *testing.T) {
	h := newHarness()
	if _, err := dispatch(t, h, domain.ActionGetPromo, map[string]any{}); err == nil {
		t.Fatal("expected error when code is absent")
	}
}

// ── redeem_promo ────────────────────────────────────────────────────────────

func TestRedeemPromo_ConsumesOneRedemption(t *testing.T) {
	h := newHarness()
	max := 10
	h.promos.promo.MaxUses = &max
	h.promos.promo.Uses = 3

	out, err := dispatch(t, h, domain.ActionRedeemPromo, map[string]any{
		"code":           "SUMMER25",
		"reservation_id": "reservation-1",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out["redeemed"] != true {
		t.Error("expected redeemed=true")
	}
	if out["uses"] != 4 {
		t.Errorf("uses = %v, want 4", out["uses"])
	}
	if out["remaining"] != 6 {
		t.Errorf("remaining = %v, want 6", out["remaining"])
	}
	if h.promos.redeemCalls != 1 {
		t.Errorf("expected exactly 1 redeem call, got %d", h.promos.redeemCalls)
	}
	assertAudited(t, h, "storefront.promo.redeem")
}

// An unlimited code reports no remaining count rather than a misleading one.
func TestRedeemPromo_UnlimitedOmitsRemaining(t *testing.T) {
	h := newHarness()
	h.promos.promo.MaxUses = nil

	out, err := dispatch(t, h, domain.ActionRedeemPromo, map[string]any{"code": "SUMMER25"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, present := out["remaining"]; present {
		t.Error("an unlimited code must not report `remaining`")
	}
	if _, present := out["max_uses"]; present {
		t.Error("an unlimited code must not report `max_uses`")
	}
}

// Exhaustion surfaces as a typed error the handler maps to 409, and is audited
// so an operator can see codes failing at checkout.
func TestRedeemPromo_Exhausted(t *testing.T) {
	h := newHarness()
	h.promos.redeemErr = pricingdomain.ErrPromoExhausted

	_, err := dispatch(t, h, domain.ActionRedeemPromo, map[string]any{"code": "SUMMER25"})
	if !errors.Is(err, pricingdomain.ErrPromoExhausted) {
		t.Fatalf("expected ErrPromoExhausted, got %v", err)
	}
	assertAudited(t, h, "storefront.promo.rejected")
	assertNotAudited(t, h, "storefront.promo.redeem")
}

func TestRedeemPromo_MissingCode(t *testing.T) {
	h := newHarness()
	if _, err := dispatch(t, h, domain.ActionRedeemPromo, map[string]any{}); err == nil {
		t.Fatal("expected error when code is absent")
	}
	if h.promos.redeemCalls != 0 {
		t.Error("must not redeem without a code")
	}
}

// ── release_promo ───────────────────────────────────────────────────────────

func TestReleasePromo(t *testing.T) {
	h := newHarness()

	out, err := dispatch(t, h, domain.ActionReleasePromo, map[string]any{
		"code":   "SUMMER25",
		"reason": "booking failed",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out["released"] != true {
		t.Error("expected released=true")
	}
	if h.promos.releaseCalls != 1 || h.promos.releasedCode[0] != "SUMMER25" {
		t.Error("expected exactly one release of SUMMER25")
	}
	assertAudited(t, h, "storefront.promo.release")
}

// ── promo actions when the gateway is absent ────────────────────────────────

// A nil promo gateway must refuse cleanly rather than panic.
func TestPromoActions_NilGateway(t *testing.T) {
	for _, action := range []string{
		domain.ActionGetPromo,
		domain.ActionRedeemPromo,
		domain.ActionReleasePromo,
	} {
		t.Run(action, func(t *testing.T) {
			h := newHarness()
			h.svc = NewService(h.props, h.pms, h.res, nil, h.holds, h.offers, h.idem, h.audit, 0)

			if _, err := dispatch(t, h, action, map[string]any{"code": "X"}); err == nil {
				t.Error("expected an error when promos are not configured")
			}
		})
	}
}

// ── reason mapping ──────────────────────────────────────────────────────────

// promoReason must return "" for non-promo errors so callers can distinguish a
// refusal from a genuine failure.
func TestPromoReason(t *testing.T) {
	cases := []struct {
		err  error
		want string
	}{
		{nil, ""},
		{pricingdomain.ErrPromoNotFound, "not_found"},
		{pricingdomain.ErrPromoInactive, "inactive"},
		{pricingdomain.ErrPromoNotYetValid, "not_yet_valid"},
		{pricingdomain.ErrPromoExpired, "expired"},
		{pricingdomain.ErrPromoExhausted, "exhausted"},
		{pricingdomain.ErrPromoWrongScope, "wrong_property"},
		{errors.New("database is down"), ""},
	}
	for _, tc := range cases {
		if got := promoReason(tc.err); got != tc.want {
			t.Errorf("promoReason(%v) = %q, want %q", tc.err, got, tc.want)
		}
	}
}
