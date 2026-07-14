package usecases

import (
	"errors"
	"testing"
	"time"

	pmsdomain "github.com/channel-manager/channel-manager/services/pms/domain"
	"github.com/channel-manager/channel-manager/services/storefront/domain"
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
	hold := h.liveHold("tok-reject")

	_, err := dispatch(t, h, domain.ActionCreateBooking, createBody(map[string]any{
		"hold_token": hold.Token,
	}))

	if err == nil {
		t.Fatal("expected error when PMS rejects the booking")
	}
	if h.res.ingestCalls != 0 {
		t.Errorf("canonical reservation written despite PMS rejection: %d calls", h.res.ingestCalls)
	}
	if h.holds.releaseCalls != 1 {
		t.Errorf("expected hold released exactly once, got %d", h.holds.releaseCalls)
	}
	if _, err := h.holds.Get(tenantCtx(), hold.Token); err != domain.ErrHoldNotFound {
		t.Error("hold should be gone after PMS rejection")
	}
	if h.idem.markCalls != 0 {
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
	hold := h.liveHold("tok-orphan")

	out, err := dispatch(t, h, domain.ActionCreateBooking, createBody(map[string]any{
		"hold_token":      hold.Token,
		"idempotency_key": "idem-orphan",
	}))

	if err != nil {
		t.Fatalf("guest must not see an error when the PMS confirmed: %v", err)
	}
	if out["booking_id"] != "pms-booking-1" {
		t.Errorf("expected PMS booking id in response, got %v", out["booking_id"])
	}
	if out["reconciliation_pending"] != true {
		t.Error("expected reconciliation_pending=true when canonical write fails")
	}
	if out["reservation_id"] != "" {
		t.Errorf("expected empty reservation_id, got %v", out["reservation_id"])
	}
	if h.idem.markCalls != 1 {
		t.Error("idempotency key must still be marked: the PMS booking is real")
	}
	assertAudited(t, h, "storefront.booking.create")
}

// A successful booking carries no reconciliation flag and yields a reservation id.
func TestCreateBooking_Success(t *testing.T) {
	h := newHarness()
	hold := h.liveHold("tok-ok")

	out, err := dispatch(t, h, domain.ActionCreateBooking, createBody(map[string]any{
		"hold_token":      hold.Token,
		"idempotency_key": "idem-ok",
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, present := out["reconciliation_pending"]; present {
		t.Error("reconciliation_pending must be absent on success")
	}
	if out["reservation_id"] != "reservation-1" {
		t.Errorf("expected reservation id, got %v", out["reservation_id"])
	}
	if h.holds.releaseCalls != 1 {
		t.Errorf("hold should be consumed exactly once, got %d", h.holds.releaseCalls)
	}
	if h.res.ingested.Status != "confirmed" {
		t.Errorf("expected confirmed reservation, got %q", h.res.ingested.Status)
	}
	if h.res.ingested.ChannelConfirmationID != "pms-booking-1" {
		t.Error("reservation should carry the PMS booking id")
	}
	// The hold, not the request body, is authoritative for the room.
	if h.pms.createdInput.RoomID != testRoomID {
		t.Errorf("expected room from hold, got %q", h.pms.createdInput.RoomID)
	}
}

// ── create_booking: idempotency replay ──────────────────────────────────────

// A replayed idempotency key is rejected before any side effect occurs.
func TestCreateBooking_DuplicateIdempotencyKey_NoSideEffects(t *testing.T) {
	h := newHarness()
	h.idem.seen["idem-replay"] = true
	hold := h.liveHold("tok-replay")

	_, err := dispatch(t, h, domain.ActionCreateBooking, createBody(map[string]any{
		"hold_token":      hold.Token,
		"idempotency_key": "idem-replay",
	}))

	if !errors.Is(err, domain.ErrDuplicateRequest) {
		t.Fatalf("expected ErrDuplicateRequest, got %v", err)
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

// ── create_booking: expired hold ────────────────────────────────────────────

// An unknown or expired hold token surfaces ErrHoldNotFound, which the HTTP
// layer maps to 410 Gone so the guest re-quotes.
func TestCreateBooking_ExpiredHold_ReturnsHoldNotFound(t *testing.T) {
	h := newHarness()

	_, err := dispatch(t, h, domain.ActionCreateBooking, createBody(map[string]any{
		"hold_token": "tok-does-not-exist",
	}))

	if !errors.Is(err, domain.ErrHoldNotFound) {
		t.Fatalf("expected ErrHoldNotFound, got %v", err)
	}
	if h.pms.createCalls != 0 {
		t.Error("PMS must not be called with an expired hold")
	}
	if h.res.ingestCalls != 0 {
		t.Error("no reservation may be written with an expired hold")
	}
}

// A hold belonging to another property must not be usable here.
func TestCreateBooking_HoldFromAnotherProperty_Rejected(t *testing.T) {
	h := newHarness()
	h.holds.seed(domain.Hold{
		Token:      "tok-foreign",
		PropertyID: "99999999-9999-9999-9999-999999999999",
		RoomID:     testRoomID,
		Checkin:    mustDay("2026-08-01"),
		Checkout:   mustDay("2026-08-03"),
		ExpiresAt:  time.Now().Add(5 * time.Minute),
	})

	_, err := dispatch(t, h, domain.ActionCreateBooking, createBody(map[string]any{
		"hold_token": "tok-foreign",
	}))
	if err == nil {
		t.Fatal("expected rejection of a hold from another property")
	}
	if h.pms.createCalls != 0 {
		t.Error("PMS must not be called with a foreign hold")
	}
}

// ── search_availability: holds subtracted ───────────────────────────────────

// A room held by an in-flight checkout must not be offered to another guest.
func TestSearchAvailability_ExcludesHeldRooms(t *testing.T) {
	h := newHarness()
	h.pms.offers = []pmsdomain.AvailabilityOffer{
		{RoomID: testRoomID, RoomTypeName: "Deluxe", IsAvailable: true, TotalPrice: 450, Currency: "USD"},
		{RoomID: "room-102", RoomTypeName: "Deluxe", IsAvailable: true, TotalPrice: 450, Currency: "USD"},
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
	if rooms[0]["room_id"] != "room-102" {
		t.Errorf("held room should be excluded, got %v", rooms[0]["room_id"])
	}
}

// A hold on non-overlapping dates must not suppress the room.
func TestSearchAvailability_NonOverlappingHoldDoesNotExclude(t *testing.T) {
	h := newHarness()
	h.pms.offers = []pmsdomain.AvailabilityOffer{
		{RoomID: testRoomID, RoomTypeName: "Deluxe", IsAvailable: true, Currency: "USD"},
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
		{RoomID: "room-103", IsAvailable: false},
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
		"booking_id": "pms-booking-1",
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
				{RoomID: testRoomID, RoomTypeName: "Deluxe", IsAvailable: true, TotalPrice: 450, Currency: "USD"},
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
	delete(body, "room_id")

	if _, err := dispatch(t, h, domain.ActionCreateBooking, body); err == nil {
		t.Fatal("expected error when neither room_id nor hold_token is supplied")
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
	hold := h.liveHold("tok-nil-audit")

	if _, err := dispatch(t, h, domain.ActionCreateBooking, createBody(map[string]any{
		"hold_token": hold.Token,
	})); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
