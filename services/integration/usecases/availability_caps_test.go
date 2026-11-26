package usecases

import (
	"testing"
	"time"

	chdomain "github.com/channel-manager/channel-manager/services/channel/domain"
)

func TestParseChannelCaps(t *testing.T) {
	caps := parseChannelCaps(map[string]any{
		"channel_caps": []any{
			map[string]any{"channel_id": "bk", "room_type_id": "dk", "max_available": float64(18)},
			map[string]any{"channel_id": "bk", "room_type_id": "suite", "max_available": float64(0)},
			map[string]any{"channel_id": "", "room_type_id": "x", "max_available": float64(1)},
			map[string]any{"channel_id": "ex", "room_type_id": "dk", "max_available": float64(-3)},
			"junk",
		},
	})
	if caps["bk"]["dk"] != 18 || caps["bk"]["suite"] != 0 {
		t.Fatalf("bk caps = %v", caps["bk"])
	}
	if v, ok := caps["ex"]["dk"]; !ok || v != 0 {
		t.Fatalf("negative cap should clamp to 0, got %v", caps["ex"])
	}
	if len(caps) != 2 {
		t.Fatalf("want 2 channels, got %d", len(caps))
	}
}

func TestApplyCapsClampsWithoutMutatingInput(t *testing.T) {
	day := time.Date(2026, 11, 26, 0, 0, 0, 0, time.UTC)
	base := []chdomain.AvailabilityUpdate{
		{RoomTypeID: "dk", Date: day, Available: 20},
		{RoomTypeID: "suite", Date: day, Available: 3},
		{RoomTypeID: "twin", Date: day, Available: 5},
	}
	got := applyCaps(base, map[string]int{"dk": 18, "suite": 0})

	if got[0].Available != 18 || got[0].StopSell {
		t.Errorf("dk = %+v, want 18 open", got[0])
	}
	if got[1].Available != 0 || !got[1].StopSell {
		t.Errorf("suite = %+v, want 0 stop-sell", got[1])
	}
	if got[2].Available != 5 {
		t.Errorf("uncapped twin changed: %+v", got[2])
	}
	if base[0].Available != 20 || base[1].StopSell {
		t.Errorf("input mutated: %+v", base)
	}
}

func TestApplyCapsBelowCapKeepsHouseAvailability(t *testing.T) {
	base := []chdomain.AvailabilityUpdate{{RoomTypeID: "dk", Available: 4}}
	if got := applyCaps(base, map[string]int{"dk": 18}); got[0].Available != 4 {
		t.Fatalf("cap above availability must not raise it, got %d", got[0].Available)
	}
}

func TestParsePMSInventory(t *testing.T) {
	ups, err := parsePMSInventory("p1", []any{
		map[string]any{
			"room_type_id": "dk",
			"days": []any{
				map[string]any{"date": "2026-11-26", "available": float64(3), "min_stay": float64(1), "max_stay": float64(30), "stop_sell": false},
				map[string]any{"date": "2026-11-27", "available": float64(0), "min_stay": float64(1), "max_stay": float64(30), "stop_sell": false},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(ups) != 2 || ups[0].Available != 3 || ups[0].MaxStay != 30 || ups[0].PropertyID != "p1" {
		t.Fatalf("unexpected %+v", ups)
	}
	if !ups[1].StopSell {
		t.Errorf("zero availability should stop-sell")
	}
	if _, err := parsePMSInventory("p1", []any{map[string]any{"days": []any{}}}); err == nil {
		t.Errorf("missing room_type_id should error")
	}
	if _, err := parsePMSInventory("p1", []any{map[string]any{"room_type_id": "dk", "days": []any{map[string]any{"date": "26/11"}}}}); err == nil {
		t.Errorf("bad date should error")
	}
}
