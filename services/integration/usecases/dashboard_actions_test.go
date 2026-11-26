package usecases

import (
	"testing"

	chdomain "github.com/channel-manager/channel-manager/services/channel/domain"
)

func TestApplyRateRules(t *testing.T) {
	base := []chdomain.RateUpdate{
		{RoomTypeID: "dk", BaseRate: 200},
		{RoomTypeID: "st", BaseRate: 99.99},
	}
	rules := []RateRule{
		{RoomTypeID: "dk", ChannelID: "conn-bk", AdjustPct: 12},
		{RoomTypeID: "st", ChannelID: "conn-bk", AdjustPct: -10},
		{RoomTypeID: "dk", ChannelID: "conn-ex", AdjustPct: 15},
	}
	bk := applyRateRules(base, "conn-bk", rules)
	if bk[0].BaseRate != 224 || bk[1].BaseRate != 89.99 {
		t.Fatalf("bk = %+v", bk)
	}
	if ab := applyRateRules(base, "conn-ab", rules); ab[0].BaseRate != 200 {
		t.Fatalf("channel without rules must get the base price, got %+v", ab)
	}
	if base[0].BaseRate != 200 {
		t.Fatalf("input mutated: %+v", base)
	}
}

func TestParseRateRules(t *testing.T) {
	ok, err := parseRateRules([]any{map[string]any{"room_type_id": "dk", "channel_id": "c", "adjust_pct": float64(12)}})
	if err != nil || len(ok) != 1 || ok[0].AdjustPct != 12 {
		t.Fatalf("got %+v %v", ok, err)
	}
	for _, bad := range []any{
		"nope",
		[]any{map[string]any{"room_type_id": "dk", "adjust_pct": float64(1)}},
		[]any{map[string]any{"room_type_id": "dk", "channel_id": "c", "adjust_pct": float64(-100)}},
		[]any{map[string]any{"room_type_id": "dk", "channel_id": "c", "adjust_pct": float64(301)}},
	} {
		if _, err := parseRateRules(bad); err == nil {
			t.Errorf("expected error for %#v", bad)
		}
	}
}

func TestParsePromo(t *testing.T) {
	p, err := parsePromo(map[string]any{"code": " WINTER10 ", "discount_pct": float64(10), "max_uses": float64(50), "valid_until": "2026-12-31"}, "prop-1")
	if err != nil {
		t.Fatal(err)
	}
	if p.Code != "WINTER10" || p.PropertyID != "prop-1" || p.MaxUses == nil || *p.MaxUses != 50 || p.ValidUntil == nil || !p.IsActive {
		t.Fatalf("got %+v", p)
	}
	org, _ := parsePromo(map[string]any{"code": "ALL", "discount_pct": float64(5), "org_wide": true}, "prop-1")
	if org.PropertyID != "" {
		t.Fatalf("org-wide promo must have no property, got %q", org.PropertyID)
	}
	if _, err := parsePromo(map[string]any{"discount_pct": float64(5)}, "p"); err == nil {
		t.Error("missing code should fail")
	}
	if _, err := parsePromo(map[string]any{"code": "X", "discount_pct": float64(5), "valid_from": "next week"}, "p"); err == nil {
		t.Error("bad date should fail")
	}
}
