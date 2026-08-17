// Package usecases orchestrates the guest-facing storefront actions.
package usecases

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
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
const DefaultOfferTTL = 5 * time.Minute

// Service orchestrates guest-facing booking actions against the PMS, canonical
// reservations, and the soft-hold store.
type Service struct {
	props    ports.PropertyLookup
	pms      ports.PmsGateway
	res      ports.ReservationWriter
	promos   ports.PromoGateway
	holds    ports.HoldStore
	offers   ports.OfferStore
	idem     ports.IdempotencyStore
	audit    ports.AuditRecorder
	holdTTL  time.Duration
	offerTTL time.Duration
	log      *slog.Logger
}

// NewService creates a storefront service. A zero holdTTL selects DefaultHoldTTL.
// A nil audit recorder disables audit logging.
func NewService(
	props ports.PropertyLookup,
	pms ports.PmsGateway,
	res ports.ReservationWriter,
	promos ports.PromoGateway,
	holds ports.HoldStore,
	offers ports.OfferStore,
	idem ports.IdempotencyStore,
	audit ports.AuditRecorder,
	holdTTL time.Duration,
) *Service {
	if holdTTL <= 0 {
		holdTTL = DefaultHoldTTL
	}
	return &Service{
		props:    props,
		pms:      pms,
		res:      res,
		promos:   promos,
		holds:    holds,
		offers:   offers,
		idem:     idem,
		audit:    audit,
		holdTTL:  holdTTL,
		offerTTL: DefaultOfferTTL,
		log:      slog.Default().With("service", "storefront"),
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

// ListProperties returns the org's sellable properties for a direct booking
// engine, newest routing config included.
//
// This is the booking engine's bootstrap call: it has no property id until a
// guest picks one, so it cannot reach get_channel_config first. Each row
// therefore carries its own booking_route, letting the engine keep routing stay
// actions per property exactly as before while sourcing the list from one place.
//
// Properties whose direct channel is switched off are omitted rather than
// returned with a flag — the storefront already refuses to quote or book them
// (see requireBookingEngine), so offering them to a guest could only produce a
// dead end.
func (s *Service) ListProperties(ctx context.Context) (map[string]any, error) {
	listings, err := s.props.ListListings(ctx)
	if err != nil {
		return nil, fmt.Errorf("storefront: list properties: %w", err)
	}
	out := make([]map[string]any, 0, len(listings))
	for _, p := range listings {
		if !p.Channel.Enabled {
			continue
		}
		out = append(out, map[string]any{
			"property_id":           p.ID,
			"external_id":           p.ExternalID,
			"name":                  p.Name,
			"timezone":              p.Timezone,
			"currency":              p.DefaultCurrency,
			"is_default":            p.IsDefault,
			"booking_route":         p.Channel.Route,
			"booking_route_percent": p.Channel.Percent,
		})
	}
	return map[string]any{"properties": out}, nil
}

type property struct {
	ID, OrgID, Name, DefaultCurrency, ExternalID, Timezone string
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
		OrgID:           prop.OrgID,
		Name:            prop.Name,
		DefaultCurrency: prop.DefaultCurrency,
		ExternalID:      prop.ExternalID,
		Timezone:        prop.Timezone,
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
		if _, unified := body["stay"]; unified {
			return s.searchUnifiedAvailability(ctx, prop, body)
		}
		return s.searchAvailability(ctx, prop, body)
	case domain.ActionSearchFlexibleAvailability:
		return s.searchFlexibleAvailability(ctx, prop, body)
	case domain.ActionGetQuote:
		return s.getQuote(ctx, prop, body)
	case domain.ActionCreateBooking:
		return s.createBooking(ctx, prop, body)
	case domain.ActionGetBooking:
		return s.getBooking(ctx, prop, body)
	case domain.ActionUpdateBooking:
		return s.updateBooking(ctx, prop, body)
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

// searchUnifiedAvailability is the v1 migration boundary for the discriminated
// stay contract. Provider adapters remain exact/flexible internally while every
// caller receives concrete stay_options.
func (s *Service) searchUnifiedAvailability(ctx context.Context, prop property, body map[string]any) (map[string]any, error) {
	stay, ok := body["stay"].(map[string]any)
	if !ok {
		return nil, errors.New("stay must be an object")
	}
	guests, ok := body["guests"].(map[string]any)
	if !ok {
		return nil, errors.New("guests must be an object")
	}
	stayType := strings.ToLower(strings.TrimSpace(stringOr(stay["type"])))
	adults := intOr(guests["adults"], 0)
	children := intOr(guests["children"], 0)
	rooms := intOr(body["rooms"], 0)
	if adults < 1 {
		return nil, errors.New("guests.adults must be at least 1")
	}
	if children < 0 {
		return nil, errors.New("guests.children cannot be negative")
	}
	if rooms < 1 {
		return nil, errors.New("rooms must be at least 1")
	}
	searchID := "srch_" + uuid.NewString()
	candidateStays := 0
	truncated := false
	var rawStays []map[string]any

	switch stayType {
	case "exact":
		_, hasWindow := stay["check_in_window"]
		_, hasNights := stay["nights"]
		if hasWindow || hasNights {
			return nil, errors.New("exact stay must not contain check_in_window or nights")
		}
		checkin := strings.TrimSpace(stringOr(stay["check_in"]))
		checkout := strings.TrimSpace(stringOr(stay["check_out"]))
		result, err := s.searchAvailability(ctx, prop, map[string]any{
			"checkin": checkin, "checkout": checkout, "adults": adults,
			"children": children, "rooms": rooms, "room_type": stringOr(body["room_type"]),
		})
		if err != nil {
			return nil, err
		}
		candidateStays = 1
		offers, _ := result["available_rooms"].([]map[string]any)
		if len(offers) > 0 {
			rawStays = append(rawStays, map[string]any{
				"check_in": checkin, "check_out": checkout,
				"nights": calendarDays(checkin, checkout), "offers": offers,
			})
		}
	case "flexible":
		_, hasCheckIn := stay["check_in"]
		_, hasCheckOut := stay["check_out"]
		if hasCheckIn || hasCheckOut {
			return nil, errors.New("flexible stay must not contain check_in or check_out")
		}
		window, ok := stay["check_in_window"].(map[string]any)
		if !ok {
			return nil, errors.New("stay.check_in_window must be an object")
		}
		from, err := time.Parse("2006-01-02", strings.TrimSpace(stringOr(window["from"])))
		if err != nil {
			return nil, errors.New("stay.check_in_window.from must use YYYY-MM-DD")
		}
		to, err := time.Parse("2006-01-02", strings.TrimSpace(stringOr(window["to"])))
		if err != nil {
			return nil, errors.New("stay.check_in_window.to must use YYYY-MM-DD")
		}
		if to.Before(from) {
			return nil, errors.New("stay.check_in_window.to must be on or after from")
		}
		candidateStays = int(to.Sub(from).Hours()/24) + 1
		if candidateStays > 31 {
			return nil, errors.New("arrival window cannot exceed 31 candidate dates")
		}
		nights := intOr(stay["nights"], 0)
		if nights < 1 || nights > 30 {
			return nil, errors.New("stay.nights must be between 1 and 30")
		}
		// The legacy provider boundary expresses an inclusive checkout ceiling.
		legacyLatestCheckout := to.AddDate(0, 0, nights).Format("2006-01-02")
		limit := intOr(body["limit"], 0)
		result, err := s.searchFlexibleAvailability(ctx, prop, map[string]any{
			"nights": nights, "adults": adults, "children": children, "rooms": rooms,
			"room_type": stringOr(body["room_type"]), "earliest_checkin": from.Format("2006-01-02"),
			"latest_checkout": legacyLatestCheckout, "limit": limit,
			"sort_by": stringOr(body["sort_by"]),
		})
		if err != nil {
			return nil, err
		}
		rawStays, _ = result["stays"].([]map[string]any)
		truncated = limit > 0 && len(rawStays) == limit && candidateStays > limit
	default:
		return nil, errors.New("stay.type must be exact or flexible")
	}

	stayOptions := make([]map[string]any, 0, len(rawStays))
	for _, rawStay := range rawStays {
		checkin := firstString(rawStay, "check_in", "checkin")
		checkout := firstString(rawStay, "check_out", "checkout")
		nights := intOr(rawStay["nights"], calendarDays(checkin, checkout))
		stayID := "stay_" + uuid.NewString()
		rawOffers, _ := rawStay["offers"].([]map[string]any)
		if rawOffers == nil {
			rawOffers, _ = rawStay["available_rooms"].([]map[string]any)
		}
		offers := make([]map[string]any, 0, len(rawOffers))
		for _, rawOffer := range rawOffers {
			currency := currencyOr(stringOr(rawOffer["currency"]), prop.DefaultCurrency)
			total := floatOr(rawOffer["total_price"], 0)
			expiresAt := time.Now().UTC().Add(s.offerTTL)
			offer := domain.Offer{
				ID: "off_" + uuid.NewString(), SearchID: searchID, StayID: stayID,
				PropertyID: prop.ID, CheckIn: checkin, CheckOut: checkout, Nights: nights,
				Adults: adults, Children: children, RequestedRooms: rooms,
				RoomIDs: anyStringSlice(rawOffer["room_ids"]), TotalAmount: total,
				Currency: currency, ExpiresAt: expiresAt,
			}
			if s.offers == nil {
				return nil, errors.New("storefront: offer store is not configured")
			}
			if err := s.offers.Put(ctx, offer); err != nil {
				return nil, err
			}
			out := cloneMap(rawOffer)
			delete(out, "total_price")
			delete(out, "currency")
			out["offer_id"] = offer.ID
			out["total"] = map[string]any{"amount": total, "currency": currency}
			out["expires_at"] = expiresAt.Format(time.RFC3339)
			offers = append(offers, out)
		}
		if len(offers) == 0 {
			continue
		}
		stayOptions = append(stayOptions, map[string]any{
			"stay_id": stayID, "check_in": checkin, "check_out": checkout,
			"nights": nights, "offers": offers,
		})
	}
	return map[string]any{
		"search_id": searchID, "property_id": publicPropertyID(prop), "stay_options": stayOptions,
		"meta": map[string]any{
			"candidate_stays": candidateStays, "available_stays": len(stayOptions),
			"returned_stays": len(stayOptions), "truncated": truncated,
		},
	}, nil
}

func publicPropertyID(prop property) string {
	if prop.ExternalID != "" {
		return prop.ExternalID
	}
	return prop.ID
}

func calendarDays(from, to string) int {
	start, err1 := time.Parse("2006-01-02", from)
	end, err2 := time.Parse("2006-01-02", to)
	if err1 != nil || err2 != nil {
		return 0
	}
	return int(end.Sub(start).Hours() / 24)
}

func firstString(values map[string]any, keys ...string) string {
	for _, key := range keys {
		if value := strings.TrimSpace(stringOr(values[key])); value != "" {
			return value
		}
	}
	return ""
}

func anyStringSlice(value any) []string {
	switch values := value.(type) {
	case []string:
		return append([]string(nil), values...)
	case []any:
		out := make([]string, 0, len(values))
		for _, value := range values {
			if text, ok := value.(string); ok {
				out = append(out, text)
			}
		}
		return out
	default:
		return nil
	}
}

func cloneMap(input map[string]any) map[string]any {
	out := make(map[string]any, len(input)+3)
	for key, value := range input {
		out[key] = value
	}
	return out
}

func (s *Service) searchAvailability(ctx context.Context, prop property, body map[string]any) (map[string]any, error) {
	if err := s.requireBookingEngine(ctx, prop.ID); err != nil {
		return nil, err
	}
	checkin, checkout, err := parseDateRange(body)
	if err != nil {
		return nil, err
	}
	adults := intOr(body["adults"], 1)
	children := intOr(body["children"], 0)
	requestedRooms := intOr(body["rooms"], 1)
	offers, err := s.pms.SearchAvailability(ctx, prop.ID, pmsdomain.AvailabilityQuery{
		Checkin:  checkin,
		Checkout: checkout,
		Adults:   adults,
		Children: children,
		Rooms:    requestedRooms,
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
		if !o.IsAvailable || offerTouchesHeldRoom(held, o.RoomIDs) {
			continue
		}
		rooms = append(rooms, map[string]any{
			"room_ids":        o.RoomIDs,
			"room_count":      o.RoomCount,
			"room_names":      o.RoomNames,
			"room_types":      o.RoomTypes,
			"room_type_id":    o.RoomTypeID,
			"room_type":       o.RoomTypeName,
			"available_units": o.AvailableUnits,
			"price_per_night": o.PricePerNight,
			"total_price":     o.TotalPrice,
			"currency":        currencyOr(o.Currency, prop.DefaultCurrency),
			"capacity":        o.Capacity,
			"max_adults":      o.MaxAdults,
			"max_children":    o.MaxChildren,
			"description":     o.Description,
			"amenities":       o.Amenities,
		})
	}
	publicPropertyID := prop.ExternalID
	if publicPropertyID == "" {
		publicPropertyID = prop.ID
	}
	// PMS returns one bookable offer per solution. For rooms>1 that is a combo
	// ("id1,id2"), so presence of any offer means the party can be accommodated.
	return map[string]any{
		"source":                      "CHANNEL_MANAGER",
		"property_id":                 publicPropertyID,
		"channel_manager_property_id": prop.ID,
		"property_name":               prop.Name,
		"checkin":                     checkin.Format("2006-01-02"),
		"checkout":                    checkout.Format("2006-01-02"),
		"adults":                      adults,
		"children":                    children,
		"requested_rooms":             requestedRooms,
		"can_accommodate":             len(rooms) > 0,
		"available_rooms":             rooms,
		"total_available":             len(rooms),
	}, nil
}

func (s *Service) searchFlexibleAvailability(ctx context.Context, prop property, body map[string]any) (map[string]any, error) {
	if err := s.requireBookingEngine(ctx, prop.ID); err != nil {
		return nil, err
	}
	nights := intOr(body["nights"], 0)
	if nights < 1 {
		return nil, errors.New("nights is required and must be at least 1")
	}
	adults := intOr(body["adults"], 1)
	children := intOr(body["children"], 0)
	requestedRooms := intOr(body["rooms"], 1)
	result, err := s.pms.SearchFlexibleAvailability(ctx, prop.ID, pmsdomain.FlexibleAvailabilityQuery{
		Nights:          nights,
		Adults:          adults,
		Children:        children,
		Rooms:           requestedRooms,
		RoomTypeName:    strings.TrimSpace(stringOr(body["room_type"])),
		EarliestCheckin: strings.TrimSpace(stringOr(body["earliest_checkin"])),
		LatestCheckout:  strings.TrimSpace(stringOr(body["latest_checkout"])),
		Limit:           intOr(body["limit"], 0),
		SortBy:          strings.TrimSpace(stringOr(body["sort_by"])),
	})
	if err != nil {
		return nil, fmt.Errorf("storefront: search flexible availability: %w", err)
	}

	stays := make([]map[string]any, 0, len(result.Stays))
	for _, stay := range result.Stays {
		checkin, err := time.Parse("2006-01-02", stay.Checkin)
		if err != nil {
			return nil, fmt.Errorf("storefront: invalid flexible stay checkin: %w", err)
		}
		checkout, err := time.Parse("2006-01-02", stay.Checkout)
		if err != nil {
			return nil, fmt.Errorf("storefront: invalid flexible stay checkout: %w", err)
		}
		held, err := s.heldRooms(ctx, prop.ID, checkin, checkout)
		if err != nil {
			return nil, err
		}
		rooms := make([]map[string]any, 0, len(stay.Offers))
		for _, o := range stay.Offers {
			if !o.IsAvailable || offerTouchesHeldRoom(held, o.RoomIDs) {
				continue
			}
			rooms = append(rooms, map[string]any{
				"room_ids":        o.RoomIDs,
				"room_count":      o.RoomCount,
				"room_names":      o.RoomNames,
				"room_types":      o.RoomTypes,
				"room_type_id":    o.RoomTypeID,
				"room_type":       o.RoomTypeName,
				"available_units": o.AvailableUnits,
				"price_per_night": o.PricePerNight,
				"total_price":     o.TotalPrice,
				"currency":        currencyOr(o.Currency, prop.DefaultCurrency),
				"capacity":        o.Capacity,
				"max_adults":      o.MaxAdults,
				"max_children":    o.MaxChildren,
				"description":     o.Description,
				"amenities":       o.Amenities,
			})
		}
		if len(rooms) == 0 {
			continue
		}
		var startingRate *pmsdomain.FlexibleStayRate
		for _, room := range rooms {
			total, _ := room["total_price"].(float64)
			if startingRate == nil || total < startingRate.Total {
				startingRate = &pmsdomain.FlexibleStayRate{
					PerNight: room["price_per_night"].(float64),
					Total:    total,
					Currency: room["currency"].(string),
				}
			}
		}
		roomTypes := stay.RoomTypes
		if len(roomTypes) == 0 {
			seen := map[string]struct{}{}
			for _, room := range rooms {
				name, _ := room["room_type"].(string)
				if name == "" {
					continue
				}
				if _, ok := seen[name]; ok {
					continue
				}
				seen[name] = struct{}{}
				roomTypes = append(roomTypes, name)
			}
		}
		entry := map[string]any{
			"checkin":             stay.Checkin,
			"checkout":            stay.Checkout,
			"nights":              stay.Nights,
			"can_accommodate":     true,
			"matching_room_types": roomTypes,
			"available_rooms":     rooms,
			"total_available":     len(rooms),
		}
		if startingRate != nil {
			entry["starting_rate"] = map[string]any{
				"per_night": startingRate.PerNight,
				"total":     startingRate.Total,
				"currency":  currencyOr(startingRate.Currency, prop.DefaultCurrency),
			}
		}
		stays = append(stays, entry)
	}

	publicPropertyID := prop.ExternalID
	if publicPropertyID == "" {
		publicPropertyID = prop.ID
	}
	earliest := result.EarliestCheckin
	latest := result.LatestCheckout
	return map[string]any{
		"source":                      "CHANNEL_MANAGER",
		"property_id":                 publicPropertyID,
		"channel_manager_property_id": prop.ID,
		"property_name":               prop.Name,
		"nights":                      result.Nights,
		"adults":                      adults,
		"children":                    children,
		"requested_rooms":             requestedRooms,
		"sort_by":                     result.SortBy,
		"search_window": map[string]any{
			"earliest_checkin": earliest,
			"latest_checkout":  latest,
		},
		"stays":          stays,
		"total_matching": result.TotalMatching,
		"returned":       len(stays),
	}, nil
}

// offerTouchesHeldRoom reports whether a soft hold covers any room in an offer.
func offerTouchesHeldRoom(held map[string]bool, roomIDs []string) bool {
	for _, roomID := range roomIDs {
		if held[roomID] {
			return true
		}
	}
	return false
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
			for _, id := range strings.Split(h.RoomID, ",") {
				id = strings.TrimSpace(id)
				if id != "" {
					held[id] = true
				}
			}
		}
	}
	return held, nil
}

// getQuote prices a stay and places a soft hold the guest can commit at checkout.
func (s *Service) getQuote(ctx context.Context, prop property, body map[string]any) (map[string]any, error) {
	if err := s.requireBookingEngine(ctx, prop.ID); err != nil {
		return nil, err
	}
	var selectedOffer *domain.Offer
	if offerID := strings.TrimSpace(stringOr(body["offer_id"])); offerID != "" {
		if s.offers == nil {
			return nil, errors.New("storefront: offer store is not configured")
		}
		offer, err := s.offers.Get(ctx, offerID)
		if err != nil {
			return nil, err
		}
		if offer.PropertyID != prop.ID {
			return nil, errors.New("offer_id does not belong to this property")
		}
		for field, expected := range map[string]string{
			"checkin": offer.CheckIn, "checkout": offer.CheckOut,
		} {
			if supplied := strings.TrimSpace(stringOr(body[field])); supplied != "" && supplied != expected {
				return nil, fmt.Errorf("%s conflicts with offer_id", field)
			}
			body[field] = expected
		}
		if supplied := intOr(body["adults"], 0); supplied != 0 && supplied != offer.Adults {
			return nil, errors.New("adults conflicts with offer_id")
		}
		if supplied, exists := body["children"]; exists && intOr(supplied, -1) != offer.Children {
			return nil, errors.New("children conflicts with offer_id")
		}
		if supplied, exists := body["rooms"]; exists && intOr(supplied, 0) != offer.RequestedRooms {
			return nil, errors.New("rooms conflicts with offer_id")
		}
		body["adults"] = offer.Adults
		roomValues := make([]any, len(offer.RoomIDs))
		for index, roomID := range offer.RoomIDs {
			roomValues[index] = roomID
		}
		if supplied, exists := body["room_ids"]; exists {
			ids, err := strictStringArray(supplied)
			if err != nil {
				return nil, err
			}
			if strings.Join(ids, "\x00") != strings.Join(offer.RoomIDs, "\x00") {
				return nil, errors.New("room_ids conflicts with offer_id")
			}
		}
		body["room_ids"] = roomValues
		selectedOffer = &offer
	}
	roomIDs, err := strictStringArray(body["room_ids"])
	if err != nil {
		return nil, err
	}
	checkin, checkout, err := parseDateRange(body)
	if err != nil {
		return nil, err
	}

	held, err := s.heldRooms(ctx, prop.ID, checkin, checkout)
	if err != nil {
		return nil, err
	}
	for _, roomID := range roomIDs {
		if held[roomID] {
			return nil, errors.New("room is currently held by another guest")
		}
	}

	quote, err := s.pms.GetQuote(ctx, prop.ID, pmsdomain.QuoteQuery{
		RoomIDs:  roomIDs,
		Checkin:  checkin,
		Checkout: checkout,
		Adults:   intOr(body["adults"], 1),
	})
	if err != nil {
		return nil, fmt.Errorf("storefront: get quote: %w", err)
	}
	if selectedOffer != nil && quote.IsAvailable {
		quotedCurrency := currencyOr(quote.Currency, prop.DefaultCurrency)
		if quote.TotalPrice != selectedOffer.TotalAmount || quotedCurrency != selectedOffer.Currency {
			return nil, errors.New("offer price changed; search again")
		}
	}

	quoteIDs := quote.RoomIDs
	if len(quoteIDs) == 0 {
		quoteIDs = roomIDs
	}

	out := map[string]any{
		"room_ids":        quoteIDs,
		"room_count":      len(quoteIDs),
		"room_name":       quote.RoomName,
		"room_type":       quote.RoomType,
		"nights":          quote.Nights,
		"adults":          quote.Adults,
		"price_per_night": quote.PricePerNight,
		"total_price":     quote.TotalPrice,
		"currency":        currencyOr(quote.Currency, prop.DefaultCurrency),
		"is_available":    quote.IsAvailable,
	}
	if selectedOffer != nil {
		out["offer_id"] = selectedOffer.ID
	}
	if !quote.IsAvailable {
		return out, nil
	}

	hold := domain.Hold{
		Token:      uuid.NewString(),
		PropertyID: prop.ID,
		RoomID:     strings.Join(quoteIDs, ","),
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
		"room_ids":    quoteIDs,
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
	idemKey = strings.TrimSpace(idemKey)
	if idemKey == "" || len(idemKey) > 200 {
		return nil, errors.New("idempotency_key is required and must contain at most 200 characters")
	}
	body["idempotency_key"] = idemKey
	requestHash, err := bookingRequestHash(body)
	if err != nil {
		return nil, err
	}
	storeKey := bookingIdempotencyStoreKey(prop.OrgID, prop.ID, idemKey)
	record, seen, err := s.idem.Get(ctx, storeKey)
	if err != nil {
		return nil, err
	}
	if seen {
		if record.RequestHash == "" || record.RequestHash != requestHash || len(record.Response) == 0 {
			return nil, domain.ErrDuplicateRequest
		}
		var replay map[string]any
		if err := json.Unmarshal(record.Response, &replay); err != nil {
			return nil, fmt.Errorf("storefront: decode booking replay: %w", err)
		}
		return replay, nil
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
	roomIDs, err := strictStringArray(body["room_ids"])
	if err != nil {
		return nil, err
	}

	guestName, _ := body["guest_name"].(string)
	if guestName == "" {
		guestName, _ = body["name"].(string)
	}
	email, _ := body["email"].(string)
	phone, _ := body["phone"].(string)
	notes, _ := body["notes"].(string)

	pmsBooking, err := s.pms.CreateBooking(ctx, prop.ID, pmsdomain.CreateBookingInput{
		RoomIDs:        roomIDs,
		Checkin:        checkin,
		Checkout:       checkout,
		GuestName:      guestName,
		Email:          email,
		Phone:          phone,
		Adults:         intOr(body["adults"], 1),
		Children:       intOr(body["children"], 0),
		Notes:          notes,
		TotalAmount:    totalAmount,
		Currency:       currencyOr(stringOr(body["currency"]), prop.DefaultCurrency),
		IdempotencyKey: idemKey,
	})
	if err != nil {
		s.recordAudit(ctx, "storefront.booking.rejected", "property", prop.ID, map[string]any{
			"room_ids": roomIDs,
			"checkin":  checkin.Format("2006-01-02"),
			"checkout": checkout.Format("2006-01-02"),
			"reason":   err.Error(),
		})
		return nil, fmt.Errorf("storefront: create booking: %w", err)
	}

	if len(pmsBooking.BookingIDs) != 1 || len(pmsBooking.RoomIDs) == 0 {
		return nil, fmt.Errorf("storefront: create booking must return one confirmation number")
	}

	reservationIDs := make([]string, 0, len(pmsBooking.RoomIDs))
	reconciliationPending := false
	confirmationID := pmsBooking.BookingIDs[0]
	for index, roomID := range pmsBooking.RoomIDs {
		individual := *pmsBooking
		individual.BookingID = confirmationID
		individual.RoomID = roomID
		if index < len(pmsBooking.RoomNames) {
			individual.RoomName = pmsBooking.RoomNames[index]
		}
		if index < len(pmsBooking.RoomTypes) {
			individual.RoomType = pmsBooking.RoomTypes[index]
		}
		reservationKey := idemKey
		if len(pmsBooking.RoomIDs) > 1 && reservationKey != "" {
			reservationKey = fmt.Sprintf("%s:%d", reservationKey, index)
		}
		reservationID, pending := s.persistReservation(ctx, prop, &individual, domain.Hold{}, body, checkin, checkout, reservationKey)
		reservationIDs = append(reservationIDs, reservationID)
		reconciliationPending = reconciliationPending || pending
	}

	s.recordAudit(ctx, "storefront.booking.create", "property", prop.ID, map[string]any{
		"property_id":            prop.ID,
		"reservation_ids":        reservationIDs,
		"pms_booking_ids":        pmsBooking.BookingIDs,
		"room_ids":               pmsBooking.RoomIDs,
		"checkin":                checkin.Format("2006-01-02"),
		"checkout":               checkout.Format("2006-01-02"),
		"total_amount":           floatOr(body["total_amount"], 0),
		"payment_status":         pmsBooking.PaymentStatus,
		"reconciliation_pending": reconciliationPending,
	})

	out := map[string]any{
		"booking_id":      pmsBooking.BookingIDs[0],
		"booking_ids":     pmsBooking.BookingIDs,
		"room_ids":        pmsBooking.RoomIDs,
		"reservation_ids": reservationIDs,
		"group_status":    pmsBooking.GroupStatus,
		"room_names":      pmsBooking.RoomNames,
		"room_types":      pmsBooking.RoomTypes,
		"checkin":         pmsBooking.Checkin,
		"checkout":        pmsBooking.Checkout,
		"adults":          pmsBooking.Adults,
		"children":        pmsBooking.Children,
		"total_amount":    totalAmount,
		"currency":        currencyOr(stringOr(body["currency"]), prop.DefaultCurrency),
		"payment_status":  pmsBooking.PaymentStatus,
	}
	if pmsBooking.Message != "" {
		out["message"] = pmsBooking.Message
	}
	if reconciliationPending {
		out["reconciliation_pending"] = true
	}
	response, err := json.Marshal(out)
	if err != nil {
		return nil, fmt.Errorf("storefront: encode booking replay: %w", err)
	}
	cacheContext, cancelCache := context.WithTimeout(context.WithoutCancel(ctx), 2*time.Second)
	defer cancelCache()
	if err := s.idem.Put(cacheContext, storeKey, ports.IdempotencyRecord{
		RequestHash: requestHash,
		Response:    response,
	}); err != nil {
		s.log.Warn("cache idempotent booking result failed", "err", err, "key", storeKey)
	}
	return out, nil
}

func bookingRequestHash(body map[string]any) (string, error) {
	payload, err := json.Marshal(body)
	if err != nil {
		return "", fmt.Errorf("storefront: encode booking request fingerprint: %w", err)
	}
	return fmt.Sprintf("%x", sha256.Sum256(payload)), nil
}

func bookingIdempotencyStoreKey(organizationID, propertyID, idempotencyKey string) string {
	scope := organizationID + "\x00" + propertyID + "\x00" + idempotencyKey
	return fmt.Sprintf("%x", sha256.Sum256([]byte(scope)))
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
	input := pmsdomain.GetBookingInput{
		BookingID:        stringOr(body["booking_id"]),
		GuestSurname:     stringOr(body["guest_surname"]),
		GuestFirstName:   stringOr(body["guest_first_name"]),
		GuestName:        stringOr(body["guest_name"]),
		Phone:            stringOr(body["phone"]),
		Email:            stringOr(body["email"]),
		Checkin:          stringOr(body["checkin"]),
		PhoneMatchLast10: boolOr(body["phone_match_last10"]),
	}
	b, err := s.pms.GetBooking(ctx, prop.ID, input)
	if err != nil {
		return nil, fmt.Errorf("storefront: get booking: %w", err)
	}
	return map[string]any{
		"booking_id":     b.BookingID,
		"status":         b.Status,
		"guest_name":     b.GuestName,
		"email":          b.Email,
		"phone":          b.Phone,
		"room_ids":       b.RoomIDs,
		"room_name":      b.RoomName,
		"room_type":      b.RoomType,
		"property_name":  b.PropertyName,
		"checkin":        b.Checkin,
		"checkout":       b.Checkout,
		"adults":         b.Adults,
		"children":       b.Children,
		"notes":          b.Notes,
		"payment_status": b.PaymentStatus,
		"source":         b.Source,
	}, nil
}

func (s *Service) updateBooking(ctx context.Context, prop property, body map[string]any) (map[string]any, error) {
	bookingID := stringOr(body["booking_id"])
	guestSurname := stringOr(body["guest_surname"])
	if bookingID == "" {
		return nil, errors.New("booking_id is required")
	}
	if guestSurname == "" {
		return nil, errors.New("guest_surname is required")
	}
	input := pmsdomain.UpdateBookingInput{
		BookingID:    bookingID,
		GuestSurname: guestSurname,
		GuestName:    stringOr(body["guest_name"]),
		Email:        stringOr(body["email"]),
		Phone:        stringOr(body["phone"]),
		Notes:        stringOr(body["notes"]),
	}
	if _, ok := body["room_ids"]; ok {
		roomIDs, err := strictStringArray(body["room_ids"])
		if err != nil {
			return nil, err
		}
		input.RoomIDs = roomIDs
	}
	if checkin := stringOr(body["checkin"]); checkin != "" {
		t, err := time.Parse("2006-01-02", checkin)
		if err != nil {
			return nil, fmt.Errorf("invalid checkin: %w", err)
		}
		input.Checkin = &t
	}
	if checkout := stringOr(body["checkout"]); checkout != "" {
		t, err := time.Parse("2006-01-02", checkout)
		if err != nil {
			return nil, fmt.Errorf("invalid checkout: %w", err)
		}
		input.Checkout = &t
	}
	if _, ok := body["adults"]; ok {
		adults := intOr(body["adults"], 0)
		input.Adults = &adults
	}
	if _, ok := body["children"]; ok {
		children := intOr(body["children"], 0)
		input.Children = &children
	}

	b, err := s.pms.UpdateBooking(ctx, prop.ID, input)
	if err != nil {
		return nil, fmt.Errorf("storefront: update booking: %w", err)
	}
	s.recordAudit(ctx, "storefront.booking.update", "reservation", bookingID, map[string]any{
		"property_id":    prop.ID,
		"pms_booking_id": bookingID,
		"status":         b.Status,
	})
	return map[string]any{
		"booking_id":    b.BookingID,
		"status":        b.Status,
		"guest_name":    b.GuestName,
		"email":         b.Email,
		"phone":         b.Phone,
		"room_ids":      b.RoomIDs,
		"room_name":     b.RoomName,
		"room_type":     b.RoomType,
		"property_name": b.PropertyName,
		"checkin":       b.Checkin,
		"checkout":      b.Checkout,
		"adults":        b.Adults,
		"children":      b.Children,
		"notes":         b.Notes,
		"message":       b.Message,
	}, nil
}

func (s *Service) cancelBooking(ctx context.Context, prop property, body map[string]any) (map[string]any, error) {
	bookingID := stringOr(body["booking_id"])
	guestSurname := stringOr(body["guest_surname"])
	if bookingID == "" {
		return nil, errors.New("booking_id is required")
	}
	if guestSurname == "" {
		return nil, errors.New("guest_surname is required")
	}
	reason := stringOr(body["reason"])

	result, err := s.pms.CancelBooking(ctx, prop.ID, pmsdomain.CancelBookingInput{
		BookingID:    bookingID,
		GuestSurname: guestSurname,
		Reason:       reason,
	})
	if err != nil {
		return nil, fmt.Errorf("storefront: cancel booking: %w", err)
	}
	// Mirror the cancellation into canonical reservations so inventory and OTA
	// pushes see the released room. Failure here is a reconciliation concern,
	// not a guest-visible one.
	reservationID := stringOr(body["reservation_id"])
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

var strictRoomIDPattern = regexp.MustCompile(`^[A-Za-z0-9._~-]{1,128}$`)

func strictStringArray(value any) ([]string, error) {
	values, ok := value.([]any)
	if !ok || len(values) == 0 {
		return nil, errors.New("room_ids must be a non-empty array")
	}
	roomIDs := make([]string, len(values))
	seen := make(map[string]struct{}, len(values))
	for index, value := range values {
		roomID, ok := value.(string)
		roomID = strings.TrimSpace(roomID)
		if !ok || roomID == "" || !strictRoomIDPattern.MatchString(roomID) {
			return nil, errors.New("room_ids contains a blank or malformed identifier")
		}
		if _, duplicate := seen[roomID]; duplicate {
			return nil, errors.New("room_ids must not contain duplicates")
		}
		seen[roomID] = struct{}{}
		roomIDs[index] = roomID
	}
	return roomIDs, nil
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

func boolOr(v any) bool {
	b, ok := v.(bool)
	return ok && b
}

func currencyOr(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return "USD"
}
