package mypms_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/channel-manager/channel-manager/services/pms/adapters/mypms"
	"github.com/channel-manager/channel-manager/services/pms/domain"
)

func TestClient_OrgHealth(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/api/webhooks/bookings" {
			t.Fatalf("unexpected %s %s", r.Method, r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-token" {
			t.Fatalf("auth header = %q", got)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"status":          "ok",
			"service":         "bookings",
			"organization_id": "org_1",
		})
	}))
	defer srv.Close()

	client := mypms.NewClient(mypms.Config{BaseURL: srv.URL, Token: "test-token"})
	health, err := client.OrgHealth(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if health.OrganizationID != "org_1" {
		t.Fatalf("org id = %q", health.OrganizationID)
	}
}

func TestClient_SearchProperties(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s", r.Method)
		}
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body["action"] != mypms.ActionSearchProperties {
			t.Fatalf("action = %v", body["action"])
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"properties": []map[string]any{
				{"property_id": "p1", "name": "Hotel One", "currency": "USD"},
			},
		})
	}))
	defer srv.Close()

	client := mypms.NewClient(mypms.Config{BaseURL: srv.URL, Token: "tok"})
	resp, err := client.SearchProperties(context.Background(), "", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Properties) != 1 || resp.Properties[0].PropertyID != "p1" {
		t.Fatalf("properties = %+v", resp.Properties)
	}
}

func TestClient_SearchAvailability(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/webhooks/bookings/prop-1" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"rooms": []map[string]any{
				{"room_type_id": "rt1", "is_available": true, "available": 3},
			},
		})
	}))
	defer srv.Close()

	client := mypms.NewClient(mypms.Config{BaseURL: srv.URL, Token: "tok"})
	resp, err := client.SearchAvailability(context.Background(), "prop-1", mypms.SearchAvailabilityRequest{
		Checkin:  "2026-06-01",
		Checkout: "2026-06-03",
		Adults:   2,
		Rooms:    1,
	})
	if err != nil {
		t.Fatal(err)
	}
	rooms := resp.RoomsList()
	if len(rooms) != 1 || rooms[0].RoomTypeID != "rt1" {
		t.Fatalf("rooms = %+v", rooms)
	}
}

// The PMS booking webhook returns availability under data.available_rooms with
// no per-room is_available flag. The adapter must surface those rooms as
// available offers; a regression here returns an empty list and the storefront
// shows "no rooms". Reproduces the Grand Palace search response.
func TestAdapter_SearchAvailability_PmsAvailableRoomsShape(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/webhooks/bookings/prop-1" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{
				"available_rooms": []map[string]any{
					{"room_ids": []string{"r1"}, "room_count": 1, "room_names": []string{"Room 101"}, "room_types": []string{"Standard Room"}, "room_type": "Standard Room", "capacity": 2, "price_per_night": 120, "total_price": 240, "currency": "USD"},
					{"room_ids": []string{"r2"}, "room_count": 1, "room_names": []string{"Room 201"}, "room_types": []string{"Deluxe Room"}, "room_type": "Deluxe Room", "capacity": 2, "price_per_night": 3800, "total_price": 7600, "currency": "INR"},
				},
			},
		})
	}))
	defer srv.Close()

	adapter := mypms.NewAdapterFromConfig(srv.URL, "tok")
	offers, err := adapter.SearchAvailability(context.Background(), "prop-1", domain.AvailabilityQuery{
		Checkin:  mustParse(t, "2026-08-01"),
		Checkout: mustParse(t, "2026-08-03"),
		Adults:   2,
		Rooms:    1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(offers) != 2 {
		t.Fatalf("expected 2 available offers, got %d", len(offers))
	}
	for _, o := range offers {
		if !o.IsAvailable {
			t.Errorf("offer %v should be available", o.RoomIDs)
		}
		if o.AvailableUnits < 1 {
			t.Errorf("offer %v should have >=1 unit, got %d", o.RoomIDs, o.AvailableUnits)
		}
	}
	if offers[0].RoomTypeName != "Standard Room" || offers[0].RoomTypeID != "Standard Room" {
		t.Errorf("room type should fall back to the name, got id=%q name=%q", offers[0].RoomTypeID, offers[0].RoomTypeName)
	}
}

// Production PMS historically returned scalar room_id / room_name without the
// room_ids array. That shape must still produce a bookable offer so existing
// properties keep working while room_ids is rolled out additively.
func TestAdapter_SearchAvailability_LegacyRoomIDShape(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{
				"available_rooms": []map[string]any{
					{
						"room_id": "cmpu45ktj00199kvsadren327", "room_name": "Room 103",
						"room_type": "Standard Room", "capacity": 2, "price_per_night": 113.33,
						"total_price": 340, "currency": "USD",
					},
					{
						"room_id": "room-1,room-2", "room_name": "2x Standard Room",
						"room_type": "2x Standard Room", "capacity": 4,
					},
				},
			},
		})
	}))
	defer srv.Close()

	adapter := mypms.NewAdapterFromConfig(srv.URL, "tok")
	offers, err := adapter.SearchAvailability(context.Background(), "prop-1", domain.AvailabilityQuery{
		Checkin:  mustParse(t, "2026-08-29"),
		Checkout: mustParse(t, "2026-09-01"),
		Adults:   2,
		Rooms:    1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(offers) != 2 {
		t.Fatalf("expected 2 offers, got %d", len(offers))
	}
	if got := offers[0].RoomIDs; len(got) != 1 || got[0] != "cmpu45ktj00199kvsadren327" {
		t.Fatalf("single room_id = %v", got)
	}
	if offers[0].RoomCount != 1 || offers[0].RoomNames[0] != "Room 103" {
		t.Fatalf("single room meta count=%d names=%v", offers[0].RoomCount, offers[0].RoomNames)
	}
	if got := offers[1].RoomIDs; len(got) != 2 || got[0] != "room-1" || got[1] != "room-2" {
		t.Fatalf("combo room_id = %v", got)
	}
	if offers[1].RoomCount != 2 {
		t.Fatalf("combo room_count = %d", offers[1].RoomCount)
	}
}

func TestAdapter_SearchFlexibleAvailability_PmsStayShape(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("Decode() error = %v", err)
		}
		if body["action"] != "search_flexible_availability" || body["nights"] != float64(2) {
			t.Fatalf("body = %#v", body)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{
				"nights": 2, "adults": 2, "children": 0, "requested_rooms": 1,
				"sort_by": "soonest",
				"search_window": map[string]any{"earliest_checkin": "2026-08-16", "latest_checkout": "2026-08-22"},
				"stays": []map[string]any{{
					"checkin": "2026-08-18", "checkout": "2026-08-20", "nights": 2,
					"can_accommodate": true,
					"starting_rate": map[string]any{"per_night": 150, "total": 300, "currency": "USD"},
					"matching_room_types": []string{"Deluxe King"},
					"available_rooms": []map[string]any{
						{"room_ids": []string{"r1"}, "room_count": 1, "room_type": "Deluxe King", "price_per_night": 150, "total_price": 300, "currency": "USD"},
					},
					"total_available": 1,
				}},
				"total_matching": 1, "returned": 1,
			},
		})
	}))
	defer srv.Close()

	adapter := mypms.NewAdapterFromConfig(srv.URL, "tok")
	result, err := adapter.SearchFlexibleAvailability(context.Background(), "prop-1", domain.FlexibleAvailabilityQuery{
		Nights: 2, Adults: 2, Rooms: 1, EarliestCheckin: "2026-08-16", LatestCheckout: "2026-08-22",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Nights != 2 || len(result.Stays) != 1 || result.Stays[0].Checkin != "2026-08-18" {
		t.Fatalf("result = %+v", result)
	}
	if len(result.Stays[0].Offers) != 1 || result.Stays[0].Offers[0].RoomIDs[0] != "r1" {
		t.Fatalf("offers = %+v", result.Stays[0].Offers)
	}
}

func mustParse(t *testing.T, s string) time.Time {
	t.Helper()
	d, err := time.Parse("2006-01-02", s)
	if err != nil {
		t.Fatal(err)
	}
	return d
}
