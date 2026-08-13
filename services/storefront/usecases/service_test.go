package usecases

import (
	"encoding/json"
	"errors"
	"testing"
	"time"

	pmsdomain "github.com/channel-manager/channel-manager/services/pms/domain"
	"github.com/channel-manager/channel-manager/services/storefront/domain"
	"github.com/channel-manager/channel-manager/services/storefront/ports"
)

// dispatch is a helper that runs an action and returns the map payload.
func dispatch(t *testing.T, h *harness, action string, body map[string]any) (map[string]any, error) {
	t.Helper()
	body["action"] = action
	resp, err := h.svc.Dispatch(tenantCtx(), testPropID, action, body)
	if err != nil {
		return nil, err
	}
	out, ok := resp.(map[string]any)
	if !ok {
		t.Fatalf("expected map[string]any response, got %T", resp)
	}
	return out, nil
}

// ── create_booking: PMS rejection ───────────────────────────────────────────

// When the PMS refuses the stay, the hold must be released so the room returns
// to inventory, no canonical reservation is written, and nothing is published
// to OTAs. This is the common failure and the reason for PMS-first ordering.
func TestCreateBooking_PmsRejection_ReleasesHoldAndWritesNoReservation(t *testing.T) {
	h := newHarness()
	h.pms.createErr = errors.New("no availability")
	_, err := dispatch(t, h, domain.ActionCreateBooking, createBody(nil))

	if err == nil {
		t.Fatal("expected error when PMS rejects the booking")
	}
	if h.res.ingestCalls != 0 {
		t.Errorf("canonical reservation written despite PMS rejection: %d calls", h.res.ingestCalls)
	}
	if h.holds.releaseCalls != 0 {
		t.Errorf("strict array booking should not consume holds, got %d releases", h.holds.releaseCalls)
	}
	if h.idem.putCalls != 0 {
		t.Error("idempotency key must not be marked for a failed booking")
	}
	assertAudited(t, h, "storefront.booking.rejected")
	assertNotAudited(t, h, "storefront.booking.create")
}

// ── create_booking: canonical write failure ─────────────────────────────────

// If the PMS confirms but the canonical write fails, the guest holds a real,
// paid booking. We must return success with reconciliation_pending rather than
// telling them it failed.
func TestCreateBooking_CanonicalWriteFails_ReturnsReconciliationPending(t *testing.T) {
	h := newHarness()
	h.res.ingestErr = errors.New("database is down")
	out, err := dispatch(t, h, domain.ActionCreateBooking, createBody(map[string]any{
		"idempotency_key": "idem-orphan",
	}))

	if err != nil {
		t.Fatalf("guest must not see an error when the PMS confirmed: %v", err)
	}
	if got := out["booking_ids"].([]string); len(got) != 1 || got[0] != "pms-booking-1" {
		t.Errorf("expected PMS booking ids in response, got %v", got)
	}
	if out["reconciliation_pending"] != true {
		t.Error("expected reconciliation_pending=true when canonical write fails")
	}
	if got := out["reservation_ids"].([]string); len(got) != 1 || got[0] != "" {
		t.Errorf("expected one empty reservation id, got %v", got)
	}
	if h.idem.putCalls != 1 {
		t.Error("idempotency key must still be marked: the PMS booking is real")
	}
	assertAudited(t, h, "storefront.booking.create")
}

// A successful booking carries no reconciliation flag and yields a reservation id.
func TestCreateBooking_Success(t *testing.T) {
	h := newHarness()
	out, err := dispatch(t, h, domain.ActionCreateBooking, createBody(map[string]any{
		"idempotency_key": "idem-ok",
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, present := out["reconciliation_pending"]; present {
		t.Error("reconciliation_pending must be absent on success")
	}
	if got := out["reservation_ids"].([]string); len(got) != 1 || got[0] != "reservation-1" {
		t.Errorf("expected reservation id, got %v", got)
	}
	if h.holds.releaseCalls != 0 {
		t.Errorf("strict array booking should not consume holds, got %d releases", h.holds.releaseCalls)
	}
	if h.res.ingested.Status != "confirmed" {
		t.Errorf("expected confirmed reservation, got %q", h.res.ingested.Status)
	}
	if h.res.ingested.ChannelConfirmationID != "pms-booking-1" {
		t.Error("reservation should carry the PMS booking id")
	}
	if len(h.pms.createdInput.RoomIDs) != 1 || h.pms.createdInput.RoomIDs[0] != testRoomID {
		t.Errorf("expected exact room_ids array, got %v", h.pms.createdInput.RoomIDs)
	}
}

// ── create_booking: idempotency replay ──────────────────────────────────────

// A replayed idempotency key returns the original response before any side effect occurs.
func TestCreateBooking_DuplicateIdempotencyKey_ReplaysResponseWithNoSideEffects(t *testing.T) {
	h := newHarness()
	body := createBody(map[string]any{
		"idempotency_key": "idem-replay",
	})
	body["action"] = domain.ActionCreateBooking
	requestHash, err := bookingRequestHash(body)
	if err != nil {
		t.Fatalf("bookingRequestHash() error = %v", err)
	}
	storeKey := bookingIdempotencyStoreKey(testOrgID, testPropID, "idem-replay")
	h.idem.records[storeKey] = ports.IdempotencyRecord{
		RequestHash: requestHash,
		Response: json.RawMessage(
			`{"booking_ids":["pms-booking-1"],"room_ids":["room-101"],"group_status":"CONFIRMED"}`,
		),
	}
	out, err := dispatch(t, h, domain.ActionCreateBooking, body)

	if err != nil {
		t.Fatalf("replay returned error: %v", err)
	}
	if got := out["booking_ids"].([]any); len(got) != 1 || got[0] != "pms-booking-1" {
		t.Fatalf("replayed booking_ids = %#v", got)
	}
	if h.pms.createCalls != 0 {
		t.Error("PMS must not be called on a replayed request")
	}
	if h.res.ingestCalls != 0 {
		t.Error("no reservation may be written on a replayed request")
	}
	if h.holds.releaseCalls != 0 {
		t.Error("a replay must not consume the original hold")
	}
	if len(h.audit.events) != 0 {
		t.Errorf("a rejected replay is not an auditable mutation, got %v", h.audit.actions())
	}
}

func TestCreateBooking_DuplicateIdempotencyKeyRejectsChangedPayload(t *testing.T) {
	h := newHarness()
	original := createBody(map[string]any{"idempotency_key": "idem-conflict"})
	original["action"] = domain.ActionCreateBooking
	requestHash, err := bookingRequestHash(original)
	if err != nil {
		t.Fatalf("bookingRequestHash() error = %v", err)
	}
	storeKey := bookingIdempotencyStoreKey(testOrgID, testPropID, "idem-conflict")
	h.idem.records[storeKey] = ports.IdempotencyRecord{
		RequestHash: requestHash,
		Response:    json.RawMessage(`{"booking_ids":["pms-booking-1"],"room_ids":["room-101"]}`),
	}
	changed := createBody(map[string]any{
		"idempotency_key": "idem-conflict",
		"guest_name":      "Different Guest",
	})

	_, err = dispatch(t, h, domain.ActionCreateBooking, changed)

	if !errors.Is(err, domain.ErrDuplicateRequest) {
		t.Fatalf("expected ErrDuplicateRequest, got %v", err)
	}
	if h.pms.createCalls != 0 {
		t.Fatal("PMS was called for a conflicting idempotency replay")
	}
}

func TestCreateBookingRequiresIdempotencyKey(t *testing.T) {
	h := newHarness()
	body := createBody(map[string]any{"idempotency_key": ""})

	_, err := dispatch(t, h, domain.ActionCreateBooking, body)

	if err == nil || err.Error() != "idempotency_key is required and must contain at most 200 characters" {
		t.Fatalf("expected required idempotency key error, got %v", err)
	}
	if h.pms.createCalls != 0 {
		t.Fatal("PMS was called without an idempotency key")
	}
}

// ── create_booking: expired hold ────────────────────────────────────────────

// An unknown or expired hold token surfaces ErrHoldNotFound, which the HTTP
// layer maps to 410 Gone so the guest re-quotes.
func TestCreateBooking_HoldCannotReplaceRoomIDs(t *testing.T) {
	h := newHarness()
	body := createBody(map[string]any{
		"hold_token": "tok-does-not-exist",
	})
	delete(body, "room_ids")
	_, err := dispatch(t, h, domain.ActionCreateBooking, body)

	if err == nil {
		t.Fatal("expected room_ids validation error")
	}
	if h.pms.createCalls != 0 {
		t.Error("PMS must not be called with an expired hold")
	}
	if h.res.ingestCalls != 0 {
		t.Error("no reservation may be written with an expired hold")
	}
}

// A hold belonging to another property must not be usable here.
func TestCreateBooking_ScalarRoomIDRejected(t *testing.T) {
	h := newHarness()
	body := createBody(map[string]any{"room_id": testRoomID})
	delete(body, "room_ids")
	_, err := dispatch(t, h, domain.ActionCreateBooking, body)
	if err == nil {
		t.Fatal("expected rejection of scalar room_id")
	}
	if h.pms.createCalls != 0 {
		t.Error("PMS must not be called with scalar room_id")
	}
}

// ── search_availability: holds subtracted ───────────────────────────────────

// A room held by an in-flight checkout must not be offered to another guest.
func TestSearchAvailability_ExcludesHeldRooms(t *testing.T) {
	h := newHarness()
	h.pms.offers = []pmsdomain.AvailabilityOffer{
		{RoomIDs: []string{testRoomID}, RoomCount: 1, RoomTypeName: "Deluxe", IsAvailable: true, TotalPrice: 450, Currency: "USD"},
		{RoomIDs: []string{"room-102"}, RoomCount: 1, RoomTypeName: "Deluxe", IsAvailable: true, TotalPrice: 450, Currency: "USD"},
	}
	h.liveHold("tok-held") // holds room-101 over 2026-08-01..03

	out, err := dispatch(t, h, domain.ActionSearchAvailability, map[string]any{
		"checkin": "2026-08-01", "checkout": "2026-08-03",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	rooms := out["available_rooms"].([]map[string]any)
	if len(rooms) != 1 {
		t.Fatalf("expected 1 available room, got %d", len(rooms))
	}
	if got := rooms[0]["room_ids"].([]string); len(got) != 1 || got[0] != "room-102" {
		t.Errorf("held room should be excluded, got %v", got)
	}
	if out["source"] != "CHANNEL_MANAGER" || out["property_id"] != testExtPropID {
		t.Fatalf("scope metadata = %#v", out)
	}
	if out["can_accommodate"] != true || out["total_available"] != 1 {
		t.Fatalf("availability summary = %#v", out)
	}
}

// A hold on non-overlapping dates must not suppress the room.
func TestSearchAvailability_NonOverlappingHoldDoesNotExclude(t *testing.T) {
	h := newHarness()
	h.pms.offers = []pmsdomain.AvailabilityOffer{
		{RoomIDs: []string{testRoomID}, RoomCount: 1, RoomTypeName: "Deluxe", IsAvailable: true, Currency: "USD"},
	}
	h.liveHold("tok-held") // 2026-08-01..03

	out, err := dispatch(t, h, domain.ActionSearchAvailability, map[string]any{
		"checkin": "2026-08-03", "checkout": "2026-08-05", // starts as the hold ends
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	rooms := out["available_rooms"].([]map[string]any)
	if len(rooms) != 1 {
		t.Fatalf("half-open stay should not conflict; expected 1 room, got %d", len(rooms))
	}
}

// PMS offers flagged unavailable are never surfaced.
func TestSearchAvailability_ExcludesUnavailableOffers(t *testing.T) {
	h := newHarness()
	h.pms.offers = []pmsdomain.AvailabilityOffer{
		{RoomIDs: []string{"room-103"}, RoomCount: 1, IsAvailable: false},
	}
	out, err := dispatch(t, h, domain.ActionSearchAvailability, map[string]any{
		"checkin": "2026-08-01", "checkout": "2026-08-03",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rooms := out["available_rooms"].([]map[string]any); len(rooms) != 0 {
		t.Errorf("expected no rooms, got %d", len(rooms))
	}
}

// Multi-room combo offers already satisfy requested_rooms; can_accommodate
// must be true when any combo is present (not len(offers) >= rooms).
func TestSearchAvailability_ComboOfferCanAccommodate(t *testing.T) {
	h := newHarness()
	h.pms.offers = []pmsdomain.AvailabilityOffer{
		{
			RoomIDs: []string{"room-101", "room-102"}, RoomCount: 2, RoomTypeName: "2x Standard Room",
			IsAvailable: true, Capacity: 4, TotalPrice: 480, Currency: "USD",
		},
	}
	out, err := dispatch(t, h, domain.ActionSearchAvailability, map[string]any{
		"checkin": "2026-08-01", "checkout": "2026-08-03",
		"adults": 2, "children": 1, "rooms": 2,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	rooms := out["available_rooms"].([]map[string]any)
	if len(rooms) != 1 {
		t.Fatalf("expected 1 combo offer, got %d", len(rooms))
	}
	if out["can_accommodate"] != true {
		t.Fatalf("combo offer should accommodate; got %#v", out)
	}
	if out["requested_rooms"] != 2 || out["total_available"] != 1 {
		t.Fatalf("availability summary = %#v", out)
	}
}

// A hold on either physical room in a combo must suppress the whole offer.
func TestSearchAvailability_ExcludesComboWhenPartHeld(t *testing.T) {
	h := newHarness()
	h.pms.offers = []pmsdomain.AvailabilityOffer{
		{
			RoomIDs: []string{"room-101", "room-102"}, RoomCount: 2, RoomTypeName: "2x Standard Room",
			IsAvailable: true, Capacity: 4, Currency: "USD",
		},
		{RoomIDs: []string{"room-103"}, RoomCount: 1, RoomTypeName: "Deluxe", IsAvailable: true, Currency: "USD"},
	}
	h.liveHold("tok-held") // holds room-101

	out, err := dispatch(t, h, domain.ActionSearchAvailability, map[string]any{
		"checkin": "2026-08-01", "checkout": "2026-08-03", "rooms": 2,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	rooms := out["available_rooms"].([]map[string]any)
	if len(rooms) != 1 {
		t.Fatalf("expected only non-held offer, got %d (%#v)", len(rooms), rooms)
	}
	if got := rooms[0]["room_ids"].([]string); len(got) != 1 || got[0] != "room-103" {
		t.Fatalf("held combo should be excluded, got %#v", rooms[0])
	}
}

// ── get_quote: hold placement ───────────────────────────────────────────────

// An available quote places a hold and returns its token and expiry.
func TestGetQuote_PlacesHold(t *testing.T) {
	h := newHarness()
	h.pms.quote = &pmsdomain.Quote{
		RoomID: testRoomID, RoomType: "Deluxe", Nights: 2,
		TotalPrice: 450, Currency: "USD", IsAvailable: true,
	}

	out, err := dispatch(t, h, domain.ActionGetQuote, map[string]any{
		"room_id": testRoomID, "checkin": "2026-08-01", "checkout": "2026-08-03",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	token, ok := out["hold_token"].(string)
	if !ok || token == "" {
		t.Fatal("expected a hold_token on an available quote")
	}
	if out["hold_expires_at"] == nil {
		t.Error("expected hold_expires_at")
	}
	if _, err := h.holds.Get(tenantCtx(), token); err != nil {
		t.Errorf("hold should be retrievable: %v", err)
	}
	assertAudited(t, h, "storefront.hold.place")
}

// An unavailable quote places no hold.
func TestGetQuote_Unavailable_PlacesNoHold(t *testing.T) {
	h := newHarness()
	h.pms.quote = &pmsdomain.Quote{RoomID: testRoomID, IsAvailable: false}

	out, err := dispatch(t, h, domain.ActionGetQuote, map[string]any{
		"room_id": testRoomID, "checkin": "2026-08-01", "checkout": "2026-08-03",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, present := out["hold_token"]; present {
		t.Error("no hold token should be issued for an unavailable room")
	}
	assertNotAudited(t, h, "storefront.hold.place")
}

// Quoting a room already held by someone else is refused.
func TestGetQuote_AlreadyHeld_Refused(t *testing.T) {
	h := newHarness()
	h.pms.quote = &pmsdomain.Quote{RoomID: testRoomID, IsAvailable: true}
	h.liveHold("tok-other")

	_, err := dispatch(t, h, domain.ActionGetQuote, map[string]any{
		"room_id": testRoomID, "checkin": "2026-08-01", "checkout": "2026-08-03",
	})
	if err == nil {
		t.Fatal("expected refusal when the room is already held")
	}
}

// ── cancel_booking ──────────────────────────────────────────────────────────

// Cancelling mirrors into canonical reservations when a reservation id is given.
func TestCancelBooking_MirrorsCanonicalCancellation(t *testing.T) {
	h := newHarness()

	out, err := dispatch(t, h, domain.ActionCancelBooking, map[string]any{
		"booking_id":     "pms-booking-1",
		"guest_surname":  "Smith",
		"reservation_id": "reservation-1",
		"reason":         "guest request",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out["status"] != "CANCELLED" {
		t.Errorf("expected CANCELLED, got %v", out["status"])
	}
	if h.res.cancelCalls != 1 || h.res.cancelledIDs[0] != "reservation-1" {
		t.Error("canonical reservation should be cancelled")
	}
	assertAudited(t, h, "storefront.booking.cancel")
}

// A canonical cancel failure must not fail the guest's cancellation: the PMS
// already released the room, and drift is a reconciliation concern.
func TestCancelBooking_WithoutReservationID_StillCancelsInPms(t *testing.T) {
	h := newHarness()

	_, err := dispatch(t, h, domain.ActionCancelBooking, map[string]any{
		"booking_id":    "pms-booking-1",
		"guest_surname": "Smith",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if h.pms.cancelCalls != 1 {
		t.Error("PMS cancel should still be called")
	}
	if h.res.cancelCalls != 0 {
		t.Error("no canonical cancel without a reservation id")
	}
}

// ── validation ──────────────────────────────────────────────────────────────

func TestDispatch_UnknownAction(t *testing.T) {
	h := newHarness()
	if _, err := h.svc.Dispatch(tenantCtx(), testPropID, "drop_tables", map[string]any{}); err == nil {
		t.Fatal("expected error for unknown action")
	}
}

// When the booking engine is switched off for a property, the storefront must
// refuse to search, quote, or create — before any hold, PMS call, or write.
func TestBookingEngineDisabled_RefusesSearchQuoteCreate(t *testing.T) {
	for _, action := range []string{
		domain.ActionSearchAvailability,
		domain.ActionGetQuote,
		domain.ActionCreateBooking,
	} {
		t.Run(action, func(t *testing.T) {
			h := newHarness()
			h.props.beDisabled = true
			// Give each action an otherwise-valid body so the only reason to
			// fail is the disabled engine.
			h.pms.offers = []pmsdomain.AvailabilityOffer{
				{RoomIDs: []string{testRoomID}, RoomCount: 1, RoomTypeName: "Deluxe", IsAvailable: true, TotalPrice: 450, Currency: "USD"},
			}
			h.liveHold("tok-1")
			body := map[string]any{
				"checkin": "2026-08-01", "checkout": "2026-08-03",
				"room_id": testRoomID, "hold_token": "tok-1",
			}
			_, err := h.svc.Dispatch(tenantCtx(), testPropID, action, body)
			if !errors.Is(err, domain.ErrBookingEngineDisabled) {
				t.Fatalf("%s: err = %v, want ErrBookingEngineDisabled", action, err)
			}
			// Nothing was committed: the canonical reservation write is never reached.
			if h.res.ingestCalls != 0 {
				t.Errorf("%s: %d reservation writes, want 0", action, h.res.ingestCalls)
			}
		})
	}
}

// A disabled engine must not block reads of existing bookings.
func TestBookingEngineDisabled_StillAllowsGetBooking(t *testing.T) {
	h := newHarness()
	h.props.beDisabled = true
	// get_booking resolves via the PMS gateway; a disabled engine is irrelevant.
	if _, err := h.svc.Dispatch(tenantCtx(), testPropID, domain.ActionGetBooking,
		map[string]any{"booking_id": "bk-1"}); errors.Is(err, domain.ErrBookingEngineDisabled) {
		t.Fatal("get_booking was blocked by the disabled engine; reads must stay open")
	}
}

func TestParseDateRange_Invalid(t *testing.T) {
	cases := []struct {
		name string
		body map[string]any
	}{
		{"missing dates", map[string]any{}},
		{"checkout before checkin", map[string]any{"checkin": "2026-08-05", "checkout": "2026-08-01"}},
		{"checkout equals checkin", map[string]any{"checkin": "2026-08-01", "checkout": "2026-08-01"}},
		{"malformed", map[string]any{"checkin": "08/01/2026", "checkout": "2026-08-03"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, _, err := parseDateRange(tc.body); err == nil {
				t.Error("expected a validation error")
			}
		})
	}
}

func TestCreateBooking_NoRoomAndNoHold_Rejected(t *testing.T) {
	h := newHarness()
	body := createBody(nil)
	delete(body, "room_ids")

	if _, err := dispatch(t, h, domain.ActionCreateBooking, body); err == nil {
		t.Fatal("expected error when room_ids is missing")
	}
	if h.pms.createCalls != 0 {
		t.Error("PMS must not be called without a room")
	}
}

// ── audit assertions ────────────────────────────────────────────────────────

func assertAudited(t *testing.T, h *harness, action string) {
	t.Helper()
	for _, a := range h.audit.actions() {
		if a == action {
			return
		}
	}
	t.Errorf("expected audit action %q, got %v", action, h.audit.actions())
}

func assertNotAudited(t *testing.T, h *harness, action string) {
	t.Helper()
	for _, a := range h.audit.actions() {
		if a == action {
			t.Errorf("did not expect audit action %q, got %v", action, h.audit.actions())
		}
	}
}

// A nil audit recorder must not panic.
func TestNilAuditRecorder_DoesNotPanic(t *testing.T) {
	h := newHarness()
	h.svc = NewService(h.props, h.pms, h.res, h.promos, h.holds, h.idem, nil, time.Minute)
	if _, err := dispatch(t, h, domain.ActionCreateBooking, createBody(nil)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// A booking that identifies only a room type (a preference booking: no room
// number and no hold) is accepted and forwards the room type to the PMS. This
// is G1 -- the old code hard-rejected it and the guest silently lost the booking.
func TestCreateBooking_RoomTypeOnly_Rejected(t *testing.T) {
	h := newHarness()
	body := createBody(map[string]any{"room_type_id": "rt-deluxe"})
	delete(body, "room_ids")

	if _, err := dispatch(t, h, domain.ActionCreateBooking, body); err == nil {
		t.Fatal("room-type-only booking must be rejected by the strict room_ids contract")
	}
	if h.pms.createCalls != 0 {
		t.Fatalf("PMS must not be called, got %d calls", h.pms.createCalls)
	}
}

// A create with no total is refused rather than silently persisted as zero (G2).
func TestCreateBooking_NoTotal_Rejected(t *testing.T) {
	h := newHarness()
	body := createBody(nil)
	delete(body, "total_amount")

	if _, err := dispatch(t, h, domain.ActionCreateBooking, body); err == nil {
		t.Fatal("expected rejection when total_amount is absent")
	}
	if h.pms.createCalls != 0 {
		t.Error("PMS must not be called when the total is missing")
	}
	if h.res.ingestCalls != 0 {
		t.Error("no reservation may be written when the total is missing")
	}
}
