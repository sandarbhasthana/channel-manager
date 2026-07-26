// Package domain defines the storefront action vocabulary and models.
//
// The storefront is the guest-facing ingress used by direct booking engines.
// It deliberately speaks the same action-based dialect as the PMS booking
// webhook so an existing booking engine can be repointed at Channel Manager
// without changing its request shapes.
package domain

// Action names for POST /api/storefront/v1/{propertyId}.
const (
	ActionSearchAvailability = "search_availability"
	ActionGetQuote           = "get_quote"
	ActionCreateBooking      = "create_booking"
	ActionGetBooking         = "get_booking"
	ActionCancelBooking      = "cancel_booking"

	// ActionGetPromo reads a promo code and evaluates its rules for this
	// property, without consuming a redemption. The booking engine uses it to
	// price a stay and to tell a guest why a code did not apply.
	ActionGetPromo = "get_promo"

	// ActionRedeemPromo atomically consumes one redemption. Channel Manager is
	// the only writer of the usage counter; a booking engine that decremented
	// its own copy could not enforce max_uses.
	ActionRedeemPromo = "redeem_promo"

	// ActionReleasePromo returns a redemption consumed by a booking that
	// subsequently failed.
	ActionReleasePromo = "release_promo"

	// ActionGetChannelConfig returns the property's booking-engine routing
	// config (enabled, route, percent). The booking engine reads it to decide
	// where to send its own stay actions during the Phase 4 cutover.
	ActionGetChannelConfig = "get_channel_config"

	// ActionRecordDirectReservation records a direct-channel reservation for a
	// stay the booking engine sent straight to the PMS (booking_route=pms), so
	// the booking is still visible in the CM Booking Engine view — which lists
	// only reservations marked source="direct". Unlike create_booking it makes
	// NO PMS booking (the PMS already has the stay); it only mirrors the
	// canonical reservation record.
	ActionRecordDirectReservation = "record_direct_reservation"
)

// AvailableActions is returned by the storefront health endpoint.
var AvailableActions = []string{
	ActionSearchAvailability,
	ActionGetQuote,
	ActionCreateBooking,
	ActionGetBooking,
	ActionCancelBooking,
	ActionGetPromo,
	ActionRedeemPromo,
	ActionReleasePromo,
	ActionGetChannelConfig,
	ActionRecordDirectReservation,
}

// DirectChannel labels reservations that originate from the storefront rather
// than an OTA. Stored in the reservation raw payload, since ChannelID is
// reserved for an OTA connection UUID.
const DirectChannel = "direct"
