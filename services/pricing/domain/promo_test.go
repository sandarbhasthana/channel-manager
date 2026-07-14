package domain

import (
	"errors"
	"testing"
	"time"
)

func ts(s string) time.Time {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		panic(err)
	}
	return t
}

func ptrInt(i int) *int              { return &i }
func ptrTime(t time.Time) *time.Time { return &t }

const propA = "11111111-1111-1111-1111-111111111111"
const propB = "22222222-2222-2222-2222-222222222222"

func basePromo() PromoCode {
	return PromoCode{Code: "SUMMER25", DiscountPct: 25, IsActive: true}
}

func TestAppliesToProperty(t *testing.T) {
	orgWide := basePromo()
	if !orgWide.AppliesToProperty(propA) || !orgWide.AppliesToProperty(propB) {
		t.Error("an org-wide code (empty PropertyID) must apply to every property")
	}

	scoped := basePromo()
	scoped.PropertyID = propA
	if !scoped.AppliesToProperty(propA) {
		t.Error("scoped code should apply to its own property")
	}
	if scoped.AppliesToProperty(propB) {
		t.Error("scoped code must not apply to another property")
	}
}

func TestExhausted(t *testing.T) {
	unlimited := basePromo()
	if unlimited.Exhausted() {
		t.Error("nil MaxUses means unlimited")
	}

	p := basePromo()
	p.MaxUses = ptrInt(2)

	p.Uses = 1
	if p.Exhausted() {
		t.Error("1 of 2 uses is not exhausted")
	}
	p.Uses = 2
	if !p.Exhausted() {
		t.Error("2 of 2 uses is exhausted")
	}
	// Defensive: a counter that somehow overshot is still exhausted.
	p.Uses = 3
	if !p.Exhausted() {
		t.Error("overshot counter must read as exhausted")
	}
}

func TestValidate(t *testing.T) {
	now := ts("2026-07-10T12:00:00Z")

	cases := []struct {
		name    string
		mutate  func(*PromoCode)
		prop    string
		wantErr error
	}{
		{"valid, org-wide", func(*PromoCode) {}, propA, nil},
		{"inactive", func(p *PromoCode) { p.IsActive = false }, propA, ErrPromoInactive},
		{
			"wrong property",
			func(p *PromoCode) { p.PropertyID = propA },
			propB,
			ErrPromoWrongScope,
		},
		{
			"right property",
			func(p *PromoCode) { p.PropertyID = propA },
			propA,
			nil,
		},
		{
			"not yet valid",
			func(p *PromoCode) { p.ValidFrom = ptrTime(ts("2026-08-01T00:00:00Z")) },
			propA,
			ErrPromoNotYetValid,
		},
		{
			"valid_from in the past is fine",
			func(p *PromoCode) { p.ValidFrom = ptrTime(ts("2026-01-01T00:00:00Z")) },
			propA,
			nil,
		},
		{
			"expired",
			func(p *PromoCode) { p.ValidUntil = ptrTime(ts("2026-07-01T00:00:00Z")) },
			propA,
			ErrPromoExpired,
		},
		{
			// valid_until is exclusive: a code expiring at exactly now is expired.
			"expires exactly now",
			func(p *PromoCode) { p.ValidUntil = ptrTime(now) },
			propA,
			ErrPromoExpired,
		},
		{
			"exhausted",
			func(p *PromoCode) { p.MaxUses = ptrInt(1); p.Uses = 1 },
			propA,
			ErrPromoExhausted,
		},
		{
			"last redemption still available",
			func(p *PromoCode) { p.MaxUses = ptrInt(2); p.Uses = 1 },
			propA,
			nil,
		},
		{
			// Inactive outranks expired: the operator's switch is checked first.
			"inactive and expired reports inactive",
			func(p *PromoCode) {
				p.IsActive = false
				p.ValidUntil = ptrTime(ts("2026-07-01T00:00:00Z"))
			},
			propA,
			ErrPromoInactive,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := basePromo()
			tc.mutate(&p)
			err := p.Validate(now, tc.prop)
			if !errors.Is(err, tc.wantErr) {
				t.Errorf("Validate() = %v, want %v", err, tc.wantErr)
			}
		})
	}
}

func TestDiscountedAmount(t *testing.T) {
	cases := []struct {
		pct    float64
		amount float64
		want   float64
	}{
		{25, 400, 300},
		{10, 99.99, 89.99},   // rounds to cents
		{100, 250, 0},        // full discount
		{33.33, 100, 66.67},  // fractional percent
		{15, 0, 0},
	}
	for _, tc := range cases {
		p := basePromo()
		p.DiscountPct = tc.pct
		if got := p.DiscountedAmount(tc.amount); got != tc.want {
			t.Errorf("%.2f%% off %.2f = %.2f, want %.2f", tc.pct, tc.amount, got, tc.want)
		}
	}
}

// The discounted amount must never carry sub-cent precision, or the figure
// stored will drift from the figure charged.
func TestDiscountedAmount_AlwaysWholeCents(t *testing.T) {
	p := basePromo()
	p.DiscountPct = 33.333
	got := p.DiscountedAmount(10)
	cents := got * 100
	if cents != float64(int64(cents)) {
		t.Errorf("DiscountedAmount produced sub-cent precision: %v", got)
	}
}
