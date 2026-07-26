// Package usecases orchestrates the guest-facing storefront actions.
package usecases

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"

	platformauth "github.com/channel-manager/channel-manager/platform/auth"
	pmsdomain "github.com/channel-manager/channel-manager/services/pms/domain"
	pricingdomain "github.com/channel-manager/channel-manager/services/pricing/domain"
	resdomain "github.com/channel-manager/channel-manager/services/reservations/domain"
	resusecases "github.com/channel-manager/channel-manager/services/reservations/usecases"
	"github.com/channel-manager/channel-manager/services/storefront/domain"
	"github.com/channel-manager/channel-manager/services/storefront/ports"
)

// DefaultHoldTTL is how long a soft hold survives without being committed.
const DefaultHoldTTL = 10 * time.Minute

// Service orchestrates guest-facing booking actions against the PMS, canonical
// reservations, and the soft-hold store.
type Service struct {
	props   ports.PropertyLookup
	pms     ports.PmsGateway
	res     ports.ReservationWriter
	promos  ports.PromoGateway
	holds   ports.HoldStore
	idem    ports.IdempotencyStore
	audit   ports.AuditRecorder
	holdTTL time.Duration
	log     *slog.Logger
}

// NewService creates a storefront service. A zero holdTTL selects DefaultHoldTTL.
// A nil audit recorder disables audit logging.
func NewService(
	props ports.PropertyLookup,
	pms ports.PmsGateway,
	res ports.ReservationWriter,
	promos ports.PromoGateway,
	holds ports.HoldStore,
	idem ports.IdempotencyStore,
	audit ports.AuditRecorder,
	holdTTL time.Duration,
) *Service {
	if holdTTL <= 0 {
		holdTTL = DefaultHoldTTL
	}
	return &Service{
		props:   props,
		pms:     pms,
		res:     res,
		promos:  promos,
		holds:   holds,
		idem:    idem,
		audit:   audit,
		holdTTL: holdTTL,
		log:     slog.Default().With("service", "storefront"),
	}
}

// recordAudit appends an audit entry for a storefront mutation.
//
// Only mutations are audited. search_availability and get_booking are pure
// reads and would flood an append-only table that exists to answer "who changed
// what". get_quote is audited because it places a hold, which is a state change.
func (s *Service) recordAudit(ctx context.Context, action, resourceType, resourceID string, metadata map[string]any) {
	if s.audit == nil {
		return
	}
	tc, err := platformauth.FromContext(ctx)
	if err != nil {
		s.log.Warn("skipping audit: no tenant context", "action", action)
		return
	}
	s.audit.Record(ctx, ports.AuditEvent{
		OrgID:        tc.OrgID,
		Action:       action,
		ResourceType: resourceType,
		ResourceID:   resourceID,
		Metadata:     metadata,
	})
}

// Health returns storefront liveness and the supported action list.
func (s *Service) Health(ctx context.Context, orgID string) map[string]any {
	return map[string]any{
		"status":            "ok",
		"service":           "channel-manager-storefront",
		"organization_id":   orgID,
		"available_actions": domain.AvailableActions,
	}
}

type property struct {
	ID, Name, DefaultCurrency, ExternalID string
}

// loadProperty resolves either an internal property UUID or the PMS external id.
func (s *Service) loadProperty(ctx context.Context, propertyID string) (property, error) {
	prop, err := s.props.GetByID(ctx, propertyID)
	if err != nil {
		prop, err = s.props.GetByExternalID(ctx, "", propertyID)
		if err != nil {
			return property{}, fmt.Errorf("property not found by ID or ExternalID: %w", err)
		}
	}
	return property{
		ID:              prop.ID,
		Name:            prop.Name,
		DefaultCurrency: prop.DefaultCurrency,
		ExternalID:      prop.ExternalID,
	}, nil
}

// Dispatch runs a property-scoped storefront action.
func (s *Service) Dispatch(ctx context.Context, propertyID, action string, body map[string]any) (any, error) {
	prop, err := s.loadProperty(ctx, propertyID)
	if err != nil {
		return nil, err
	}
	switch action {
	case domain.ActionSearchAvailability:
		return s.searchAvailability(ctx, prop, body)
	case domain.ActionGetQuote:
		return s.getQuote(ctx, prop, body)
	case domain.ActionCreateBooking:
		return s.createBooking(ctx, prop, body)
	case domain.ActionGetBooking:
		return s.getBooking(ctx, prop, body)
	case domain.ActionCancelBooking:
		return s.cancelBooking(ctx, prop, body)
	case domain.ActionGetPromo:
		return s.getPromo(ctx, prop, body)
	case domain.ActionRedeemPromo:
		return s.redeemPromo(ctx, prop, body)
	case domain.ActionReleasePromo:
		return s.releasePromo(ctx, prop, body)
	case domain.ActionGetChannelConfig:
		return s.getChannelConfig(ctx, prop)
	case domain.ActionRecordDirectReservation:
		return s.recordDirectReservation(ctx, prop, body)
	default:
		return nil, fmt.Errorf("unknown action %q", action)
	}
}

// getPromo resolves a code and evaluates its rules for this property without
// consuming a redemption.
//
// An ineligible code is not an error: the response carries valid=false and a
// machine-readable reason, so the booking engine can tell a guest "expired"
// rather than silently charging full price. Only an unknown code errors.
// getChannelConfig returns the property's booking-engine routing config for the
// booking engine to read. A pure config read — not gated by the direct-channel
// toggle, since the engine must be able to learn it is disabled.
func (s *Service) getChannelConfig(ctx context.Context, prop property) (map[string]any, error) {
	cfg, err := s.props.GetChannelConfig(ctx, prop.ID)
	if err != nil {
		return nil, fmt.Errorf("storefront: get channel config: %w", err)
	}
	return map[string]any{
		"property_id":            prop.ID,
		"booking_engine_enabled": cfg.Enabled,
		"booking_route":          cfg.Route,
		"booking_route_percent":  cfg.Percent,
	}, nil
}

func (s *Service) getPromo(ctx context.Context, prop property, body map[string]any) (map[string]any, error) {
	if s.promos == nil {
		return nil, errors.New("storefront: promo codes are not configured")
	}
	code, _ := body["code"].(string)
	if code == "" {
		return nil, errors.New("code is required")
	}

	result, err := s.promos.Lookup(ctx, code, prop.ID)
	if err != nil {
		return nil, err
	}

	out := map[string]any{
		"code":         result.Promo.Code,
		"discount_pct": result.Promo.DiscountPct,
		"valid":        result.Valid,
	}
	if result.Promo.Description != "" {
		out["description"] = result.Promo.Description
	}
	if !result.Valid {
		out["reason"] = promoReason(result.Reason)
		// A code that does not apply must not advertise a discount the guest
		// will not receive.
		out["discount_pct"] = 0.0
	}
	return out, nil
}

// redeemPromo atomically consumes one redemption.
//
// Channel Manager is the only writer of the usage counter. A booking engine
// that decremented its own copy could not enforce max_uses across replicas or
// channels, so this action exists rather than exposing the raw counter.
func (s *Service) redeemPromo(ctx context.Context, prop property, body map[string]any) (map[string]any, error) {
	if s.promos == nil {
		return nil, errors.New("storefront: promo codes are not configured")
	}
	code, _ := body["code"].(string)
	if code == "" {
		return nil, errors.New("code is required")
	}

	promo, err := s.promos.Redeem(ctx, code, prop.ID)
	if err != nil {
		// Rejections are expected, not exceptional: report them as a structured
		// refusal so the caller can distinguish "exhausted" from "server broke".
		if reason := promoReason(err); reason != "" {
			s.recordAudit(ctx, "storefront.promo.rejected", "promo", code, map[string]any{
				"property_id": prop.ID,
				"reason":      reason,
			})
			return nil, err
		}
		return nil, err
	}

	s.recordAudit(ctx, "storefront.promo.redeem", "promo", promo.Code, map[string]any{
		"property_id":    prop.ID,
		"discount_pct":   promo.DiscountPct,
		"uses":           promo.Uses,
		"reservation_id": stringOr(body["reservation_id"]),
	})

	out := map[string]any{
		"code":         promo.Code,
		"discount_pct": promo.DiscountPct,
		"uses":         promo.Uses,
		"redeemed":     true,
	}
	if promo.MaxUses != nil {
		out["max_uses"] = *promo.MaxUses
		out["remaining"] = *promo.MaxUses - promo.Uses
	}
	return out, nil
}

// releasePromo returns a redemption consumed by a booking that then failed.
func (s *Service) releasePromo(ctx context.Context, prop property, body map[string]any) (map[string]any, error) {
	if s.promos == nil {
		return nil, errors.New("storefront: promo codes are not configured")
	}
	code, _ := body["code"].(string)
	if code == "" {
		return nil, errors.New("code is required")
	}

	if err := s.promos.ReleaseRedemption(ctx, code); err != nil {
		return nil, err
	}
	s.recordAudit(ctx, "storefront.promo.release", "promo", code, map[string]any{
		"property_id": prop.ID,
		"reason":      stringOr(body["reason"]),
	})
	return map[string]any{"code": code, "released": true}, nil
}

// promoReason maps a pricing rejection onto a stable wire string. Returns ""
// for errors that are not promo rejections, so callers can tell a refusal from
// a genuine failure.
func promoReason(err error) string {
	switch {
	case err == nil:
		return ""
	case errors.Is(err, pricingdomain.ErrPromoNotFound):
		return "not_found"
	case errors.Is(err, pricingdomain.ErrPromoInactive):
		return "inactive"
	case errors.Is(err, pricingdomain.ErrPromoNotYetValid):
		return "not_yet_valid"
	case errors.Is(err, pricingdomain.ErrPromoExpired):
		return "expired"
	case errors.Is(err, pricingdomain.ErrPromoExhausted):
		return "exhausted"
	case errors.Is(err, pricingdomain.ErrPromoWrongScope):
		return "wrong_property"
	default:
		return ""
	}
}

// searchAvailability returns PMS offers net of in-flight direct holds.
//
// The PMS already reflects OTA bookings, so subtracting our own uncommitted
// holds yields true cross-channel availability for the storefront.
// requireBookingEngine refuses the action when the property's direct channel is
// switched off. Checked at the entry of the two guest-facing mutations —
// search and create — so a disabled property surfaces no offers and accepts no
// bookings, while reads of existing bookings still work.
func (s *Service) requireBookingEngine(ctx context.Context, propertyID string) error {
	enabled, err := s.props.BookingEngineEnabled(ctx, propertyID)
	if err != nil {
		return fmt.Errorf("storefront: booking engine status: %w", err)
	}
	if !enabled {
		return domain.ErrBookingEngineDisabled
	}
	return nil
}

func (s *Service) searchAvailability(ctx context.Context, prop property, body map[string]any) (map[string]any, error) {
	if err := s.requireBookingEngine(ctx, prop.ID); err != nil {
		return nil, err
	}
	checkin, checkout, err := parseDateRange(body)
	if err != nil {
		return nil, err
	}
	offers, err := s.pms.SearchAvailability(ctx, prop.ID, pmsdomain.AvailabilityQuery{
		Checkin:  checkin,
		Checkout: checkout,
		Adults:   intOr(body["adults"], 1),
		Children: intOr(body["children"], 0),
		Rooms:    intOr(body["rooms"], 1),
	})
	if err != nil {
		return nil, fmt.Errorf("storefront: search availability: %w", err)
	}

	held, err := s.heldRooms(ctx, prop.ID, checkin, checkout)
	if err != nil {
		return nil, err
	}

	rooms := make([]map[string]any, 0, len(offers))
	for _, o := range offers {
		if !o.IsAvailable || held[o.RoomID] {
			continue
		}
		rooms = append(rooms, map[string]any{
			"room_id":         o.RoomID,
			"room_type_id":    o.RoomTypeID,
			"room_type":       o.RoomTypeName,
			"available_units": o.AvailableUnits,
			"price_per_night": o.PricePerNight,
			"total_price":     o.TotalPrice,
			"currency":        currencyOr(o.Currency, prop.DefaultCurrency),
			"capacity":        o.Capacity,
		})
	}
	return map[string]any{"available_rooms": rooms}, nil
}

// heldRooms returns the set of room ids soft-held over the requested stay.
func (s *Service) heldRooms(ctx context.Context, propertyID string, checkin, checkout time.Time) (map[string]bool, error) {
	active, err := s.holds.ActiveForProperty(ctx, propertyID)
	if err != nil {
		return nil, fmt.Errorf("storefront: load holds: %w", err)
	}
	held := make(map[string]bool, len(active))
	for _, h := range active {
		if h.Overlaps(checkin, checkout) {
			held[h.RoomID] = true
		}
	}
	return held, nil
}

// getQuote prices a stay and places a soft hold the guest can commit at checkout.
func (s *Service) getQuote(ctx context.Context, prop property, body map[string]any) (map[string]any, error) {
	if err := s.requireBookingEngine(ctx, prop.ID); err != nil {
		return nil, err
	}
	roomID, _ := body["room_id"].(string)
	if roomID == "" {
		return nil, errors.New("room_id is required")
	}
	checkin, checkout, err := parseDateRange(body)
	if err != nil {
		return nil, err
	}

	held, err := s.heldRooms(ctx, prop.ID, checkin, checkout)
	if err != nil {
		return nil, err
	}
	if held[roomID] {
		return nil, errors.New("room is currently held by another guest")
	}

	quote, err := s.pms.GetQuote(ctx, prop.ID, pmsdomain.QuoteQuery{
		RoomID:   roomID,
		Checkin:  checkin,
		Checkout: checkout,
		Adults:   intOr(body["adults"], 1),
	})
	if err != nil {
		return nil, fmt.Errorf("storefront: get quote: %w", err)
	}

	out := map[string]any{
		"room_id":         quote.RoomID,
		"room_name":       quote.RoomName,
		"room_type":       quote.RoomType,
		"nights":          quote.Nights,
		"adults":          quote.Adults,
		"price_per_night": quote.PricePerNight,
		"total_price":     quote.TotalPrice,
		"currency":        currencyOr(quote.Currency, prop.DefaultCurrency),
		"is_available":    quote.IsAvailable,
	}
	if !quote.IsAvailable {
		return out, nil
	}

	hold := domain.Hold{
		Token:      uuid.NewString(),
		PropertyID: prop.ID,
		RoomID:     quote.RoomID,
		RoomTypeID: quote.RoomType,
		Checkin:    checkin,
		Checkout:   checkout,
		ExpiresAt:  time.Now().Add(s.holdTTL),
	}
	if err := s.holds.Place(ctx, hold); err != nil {
		return nil, err
	}
	s.recordAudit(ctx, "storefront.hold.place", "hold", hold.Token, map[string]any{
		"property_id": prop.ID,
		"room_id":     hold.RoomID,
		"checkin":     checkin.Format("2006-01-02"),
		"checkout":    checkout.Format("2006-01-02"),
		"expires_at":  hold.ExpiresAt.UTC().Format(time.RFC3339),
	})
	out["hold_token"] = hold.Token
	out["hold_expires_at"] = hold.ExpiresAt.UTC().Format(time.RFC3339)
	return out, nil
}

// createBooking commits a held room into a real booking.
//
// Ordering note: the PMS booking is created *before* the canonical reservation
// is persisted. The PMS is the external authority that can still reject a stay,
// and rejection is the common failure mode; creating the reservation first would
// publish reservation.created (fanning availability out to OTAs) for a booking
// the PMS may refuse. If the PMS confirms but the canonical write then fails —
// rare, and it means the database is down — we never tell the guest their paid
// booking failed. We return the PMS booking id with reconciliation_pending set
// so the nightly reconciliation job adopts the orphan.
func (s *Service) createBooking(ctx context.Context, prop property, body map[string]any) (map[string]any, error) {
	if err := s.requireBookingEngine(ctx, prop.ID); err != nil {
		return nil, err
	}
	idemKey, _ := body["idempotency_key"].(string)
	if idemKey != "" {
		seen, err := s.idem.Exists(ctx, idemKey)
		if err != nil {
			return nil, err
		}
		if seen {
			return nil, domain.ErrDuplicateRequest
		}
	}

	checkin, checkout, err := parseDateRange(body)
	if err != nil {
		return nil, err
	}
	// A booking must carry a total. The booking engine derives it server-side;
	// silently persisting a zero (the old behaviour) hides revenue and is G2.
	totalAmount := floatOr(body["total_amount"], 0)
	if totalAmount <= 0 {
		return nil, errors.New("total_amount is required and must be greater than zero")
	}

	roomID, _ := body["room_id"].(string)
	roomTypeID, _ := body["room_type_id"].(string)

	// A hold is optional but strongly preferred: it is what makes the
	// last-room race safe. When present it also authoritatively supplies the room.
	holdToken, _ := body["hold_token"].(string)
	var hold domain.Hold
	if holdToken != "" {
		hold, err = s.holds.Get(ctx, holdToken)
		if err != nil {
			return nil, err
		}
		if hold.PropertyID != prop.ID {
			return nil, errors.New("hold does not belong to this property")
		}
		roomID = hold.RoomID
		if hold.RoomTypeID != "" {
			roomTypeID = hold.RoomTypeID
		}
	}
	// A booking identifies its inventory by a specific room, a room type, or a
	// hold that supplies the room. CM never mints a room number for a preference
	// booking (D6): room assignment is the PMS's concern.
	if roomID == "" && roomTypeID == "" {
		return nil, errors.New("room_id, room_type_id, or hold_token is required")
	}

	guestName, _ := body["guest_name"].(string)
	if guestName == "" {
		guestName, _ = body["name"].(string)
	}
	email, _ := body["email"].(string)
	phone, _ := body["phone"].(string)
	notes, _ := body["notes"].(string)

	pmsBooking, err := s.pms.CreateBooking(ctx, prop.ID, pmsdomain.CreateBookingInput{
		RoomID:     roomID,
		RoomTypeID: roomTypeID,
		Checkin:    checkin,
		Checkout:   checkout,
		GuestName:  guestName,
		Email:     email,
		Phone:     phone,
		Adults:    intOr(body["adults"], 1),
		Children:  intOr(body["children"], 0),
		Notes:     notes,
	})
	if err != nil {
		// The PMS refused the stay: free the room for the next guest.
		if holdToken != "" {
			_ = s.holds.Release(ctx, holdToken)
		}
		s.recordAudit(ctx, "storefront.booking.rejected", "property", prop.ID, map[string]any{
			"room_id":  roomID,
			"checkin":  checkin.Format("2006-01-02"),
			"checkout": checkout.Format("2006-01-02"),
			"reason":   err.Error(),
		})
		return nil, fmt.Errorf("storefront: create booking: %w", err)
	}

	reservationID, reconciliationPending := s.persistReservation(ctx, prop, pmsBooking, hold, body, checkin, checkout, idemKey)

	s.recordAudit(ctx, "storefront.booking.create", "reservation", pmsBooking.BookingID, map[string]any{
		"property_id":            prop.ID,
		"reservation_id":         reservationID,
		"pms_booking_id":         pmsBooking.BookingID,
		"room_id":                roomID,
		"checkin":                checkin.Format("2006-01-02"),
		"checkout":               checkout.Format("2006-01-02"),
		"total_amount":           floatOr(body["total_amount"], 0),
		"payment_status":         pmsBooking.PaymentStatus,
		"reconciliation_pending": reconciliationPending,
	})

	if holdToken != "" {
		_ = s.holds.Release(ctx, holdToken)
	}
	if idemKey != "" {
		if err := s.idem.Mark(ctx, idemKey); err != nil {
			s.log.Warn("mark idempotency key failed", "err", err, "key", idemKey)
		}
	}

	out := map[string]any{
		"booking_id":     pmsBooking.BookingID,
		"reservation_id": reservationID,
		"status":         pmsBooking.Status,
		"room_id":        pmsBooking.RoomID,
		"room_type":      pmsBooking.RoomType,
		"checkin":        pmsBooking.Checkin,
		"checkout":       pmsBooking.Checkout,
		"payment_status": pmsBooking.PaymentStatus,
	}
	if reconciliationPending {
		out["reconciliation_pending"] = true
	}
	return out, nil
}

// persistReservation writes the canonical direct reservation. It never fails the
// caller: a write failure is reported as reconciliation_pending instead.
func (s *Service) persistReservation(
	ctx context.Context,
	prop property,
	pmsBooking *pmsdomain.PmsBooking,
	hold domain.Hold,
	body map[string]any,
	checkin, checkout time.Time,
	idemKey string,
) (reservationID string, reconciliationPending bool) {
	tc, err := platformauth.FromContext(ctx)
	if err != nil {
		s.log.Error("no tenant context persisting reservation", "booking_id", pmsBooking.BookingID)
		return "", true
	}

	meta, _ := json.Marshal(map[string]any{
		"source":         domain.DirectChannel,
		"pms_booking_id": pmsBooking.BookingID,
		"payment_status": pmsBooking.PaymentStatus,
	})

	// Prefer the hold's room type; otherwise the caller supplies it directly for
	// a preference booking that never placed a hold.
	roomTypeID := hold.RoomTypeID
	if roomTypeID == "" {
		roomTypeID, _ = body["room_type_id"].(string)
	}

	res := &resdomain.Reservation{
		OrgID:                 tc.OrgID,
		PropertyID:            prop.ID,
		ExternalPropertyID:    prop.ExternalID,
		RoomTypeID:            roomTypeID,
		GuestName:             pmsBooking.GuestName,
		CheckIn:               checkin,
		CheckOut:              checkout,
		Status:                "confirmed",
		TotalAmount:           floatOr(body["total_amount"], 0),
		Currency:              currencyOr(stringOr(body["currency"]), prop.DefaultCurrency),
		ChannelConfirmationID: pmsBooking.BookingID,
		RawPayload:            meta,
	}

	id, _, err := s.res.RecordReservation(ctx, res, idemKey)
	if err != nil {
		if errors.Is(err, resusecases.ErrDuplicateRequest) {
			// Another in-flight attempt already recorded this reservation.
			return "", false
		}
		s.log.Error("canonical reservation write failed; orphaned PMS booking",
			"err", err, "pms_booking_id", pmsBooking.BookingID, "property_id", prop.ID)
		return "", true
	}
	return id, false
}

// recordDirectReservation mirrors a booking the engine already created at the
// PMS (booking_route=pms) into the canonical reservations store, tagged
// source="direct", so it appears in the CM Booking Engine view. It creates NO
// PMS booking — the PMS already owns the stay — unlike createBooking, whose PMS
// call this deliberately omits.
//
// Deliberately NOT gated by requireBookingEngine: the booking has already
// happened, so it must be recorded for visibility whether or not the direct
// channel is currently switched on. Idempotent via the caller's key: a repeat
// is a no-op, never a duplicate row.
func (s *Service) recordDirectReservation(ctx context.Context, prop property, body map[string]any) (map[string]any, error) {
	tc, err := platformauth.FromContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("storefront: record direct reservation: %w", err)
	}

	checkin, checkout, err := parseDateRange(body)
	if err != nil {
		return nil, err
	}

	totalAmount := floatOr(body["total_amount"], 0)
	if totalAmount <= 0 {
		return nil, errors.New("total_amount is required and must be greater than zero")
	}

	guestName, _ := body["guest_name"].(string)
	if guestName == "" {
		guestName, _ = body["name"].(string)
	}

	pmsBookingID := stringOr(body["pms_booking_id"])
	idemKey, _ := body["idempotency_key"].(string)

	meta, _ := json.Marshal(map[string]any{
		"source":         domain.DirectChannel,
		"pms_booking_id": pmsBookingID,
		"payment_status": stringOr(body["payment_status"]),
	})

	res := &resdomain.Reservation{
		OrgID:                 tc.OrgID,
		PropertyID:            prop.ID,
		ExternalPropertyID:    prop.ExternalID,
		RoomTypeID:            stringOr(body["room_type_id"]),
		GuestName:             guestName,
		CheckIn:               checkin,
		CheckOut:              checkout,
		Status:                "confirmed",
		TotalAmount:           totalAmount,
		Currency:              currencyOr(stringOr(body["currency"]), prop.DefaultCurrency),
		ChannelConfirmationID: pmsBookingID,
		RawPayload:            meta,
	}

	id, _, err := s.res.RecordReservation(ctx, res, idemKey)
	if err != nil {
		// Mirroring a booking we already recorded is a no-op, not an error: the
		// booking engine may retry the mirror after a network blip.
		if errors.Is(err, resusecases.ErrDuplicateRequest) {
			return map[string]any{"recorded": false, "duplicate": true}, nil
		}
		return nil, fmt.Errorf("storefront: record direct reservation: %w", err)
	}

	s.recordAudit(ctx, "storefront.booking.record_direct", "reservation", id, map[string]any{
		"property_id":    prop.ID,
		"reservation_id": id,
		"pms_booking_id": pmsBookingID,
		"total_amount":   totalAmount,
	})

	return map[string]any{"reservation_id": id, "recorded": true}, nil
}

func (s *Service) getBooking(ctx context.Context, prop property, body map[string]any) (map[string]any, error) {
	bookingID, _ := body["booking_id"].(string)
	if bookingID == "" {
		return nil, errors.New("booking_id is required")
	}
	b, err := s.pms.GetBooking(ctx, prop.ID, bookingID)
	if err != nil {
		return nil, fmt.Errorf("storefront: get booking: %w", err)
	}
	return map[string]any{
		"booking_id":     b.BookingID,
		"status":         b.Status,
		"guest_name":     b.GuestName,
		"room_type":      b.RoomType,
		"checkin":        b.Checkin,
		"checkout":       b.Checkout,
		"payment_status": b.PaymentStatus,
	}, nil
}

func (s *Service) cancelBooking(ctx context.Context, prop property, body map[string]any) (map[string]any, error) {
	bookingID, _ := body["booking_id"].(string)
	if bookingID == "" {
		return nil, errors.New("booking_id is required")
	}
	reason, _ := body["reason"].(string)

	result, err := s.pms.CancelBooking(ctx, prop.ID, bookingID, reason)
	if err != nil {
		return nil, fmt.Errorf("storefront: cancel booking: %w", err)
	}
	// Mirror the cancellation into canonical reservations so inventory and OTA
	// pushes see the released room. Failure here is a reconciliation concern,
	// not a guest-visible one.
	reservationID, _ := body["reservation_id"].(string)
	if reservationID != "" {
		if _, err := s.res.CancelReservation(ctx, reservationID); err != nil {
			s.log.Error("canonical reservation cancel failed",
				"err", err, "reservation_id", reservationID, "pms_booking_id", bookingID)
		}
	}
	s.recordAudit(ctx, "storefront.booking.cancel", "reservation", bookingID, map[string]any{
		"property_id":    prop.ID,
		"reservation_id": reservationID,
		"pms_booking_id": bookingID,
		"reason":         reason,
		"status":         result.Status,
	})
	return map[string]any{
		"booking_id": result.BookingID,
		"status":     result.Status,
		"message":    result.Message,
	}, nil
}

// ── body parsing helpers ────────────────────────────────────────────────────

func parseDateRange(body map[string]any) (time.Time, time.Time, error) {
	checkinRaw, _ := body["checkin"].(string)
	checkoutRaw, _ := body["checkout"].(string)
	if checkinRaw == "" || checkoutRaw == "" {
		return time.Time{}, time.Time{}, errors.New("checkin and checkout are required (YYYY-MM-DD)")
	}
	checkin, err := time.Parse("2006-01-02", checkinRaw)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("invalid checkin: %w", err)
	}
	checkout, err := time.Parse("2006-01-02", checkoutRaw)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("invalid checkout: %w", err)
	}
	if !checkout.After(checkin) {
		return time.Time{}, time.Time{}, errors.New("checkout must be after checkin")
	}
	return checkin, checkout, nil
}

func intOr(v any, def int) int {
	switch n := v.(type) {
	case float64: // encoding/json decodes numbers into float64
		return int(n)
	case int:
		return n
	}
	return def
}

func floatOr(v any, def float64) float64 {
	if n, ok := v.(float64); ok {
		return n
	}
	return def
}

func stringOr(v any) string {
	s, _ := v.(string)
	return s
}

func currencyOr(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return "USD"
}
