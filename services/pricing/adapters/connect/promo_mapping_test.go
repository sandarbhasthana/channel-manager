package connect

import (
	"testing"
	"time"

	pricingv1 "github.com/channel-manager/channel-manager/gen/go/pricing/v1"
	"github.com/channel-manager/channel-manager/services/pricing/domain"
)

func intp(i int) *int { return &i }

// max_uses is the one field with a lossy representation: nil (unlimited) in the
// domain, 0 on the wire. These tests pin both directions.
func TestPromoFromProto_MaxUses(t *testing.T) {
	if got := promoFromProto(&pricingv1.PromoCode{MaxUses: 0}); got.MaxUses != nil {
		t.Errorf("max_uses 0 → %v, want nil (unlimited)", *got.MaxUses)
	}
	if got := promoFromProto(&pricingv1.PromoCode{MaxUses: 5}); got.MaxUses == nil || *got.MaxUses != 5 {
		t.Errorf("max_uses 5 → %v, want 5", got.MaxUses)
	}
}

func TestPromoToProto_MaxUses(t *testing.T) {
	if got := promoToProto(domain.PromoCode{MaxUses: nil}); got.MaxUses != 0 {
		t.Errorf("nil max_uses → %d, want 0", got.MaxUses)
	}
	if got := promoToProto(domain.PromoCode{MaxUses: intp(3)}); got.MaxUses != 3 {
		t.Errorf("max_uses 3 → %d, want 3", got.MaxUses)
	}
}

// A domain → proto → domain round trip preserves the fields the dashboard edits.
func TestPromoRoundTrip(t *testing.T) {
	from := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	until := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	in := domain.PromoCode{
		ID: "id-1", PropertyID: "prop-1", Code: "SUMMER25", Description: "summer",
		DiscountPct: 15, MaxUses: intp(100), IsActive: true,
		ValidFrom: &from, ValidUntil: &until,
	}
	out := promoFromProto(promoToProto(in))

	if out.Code != in.Code || out.PropertyID != in.PropertyID || out.DiscountPct != in.DiscountPct ||
		out.Description != in.Description || out.IsActive != in.IsActive {
		t.Errorf("round trip scalar mismatch: %+v", out)
	}
	if out.MaxUses == nil || *out.MaxUses != 100 {
		t.Errorf("round trip max_uses = %v, want 100", out.MaxUses)
	}
	if out.ValidFrom == nil || !out.ValidFrom.Equal(from) || out.ValidUntil == nil || !out.ValidUntil.Equal(until) {
		t.Errorf("round trip validity window mismatch: %v..%v", out.ValidFrom, out.ValidUntil)
	}
}

// An org-wide promo (empty property_id) survives the round trip as empty, not "".
func TestPromoRoundTrip_OrgWide(t *testing.T) {
	out := promoFromProto(promoToProto(domain.PromoCode{Code: "ORGWIDE", PropertyID: ""}))
	if out.PropertyID != "" {
		t.Errorf("org-wide property_id = %q, want empty", out.PropertyID)
	}
}
