package domain

import (
	"errors"
	"time"
)

// ErrHoldNotFound is returned when a hold token is unknown or already expired.
var ErrHoldNotFound = errors.New("storefront: hold not found or expired")

// ErrOfferNotFound is returned when an offer reference is unknown or expired.
var ErrOfferNotFound = errors.New("storefront: offer not found or expired")

// ErrDuplicateRequest is returned when an idempotency key was already used.
var ErrDuplicateRequest = errors.New("storefront: duplicate request")

// ErrBookingEngineDisabled is returned when a property's direct sales channel
// is switched off. The storefront neither quotes nor creates bookings for it.
var ErrBookingEngineDisabled = errors.New("storefront: booking engine is disabled for this property")

// Hold is a short-lived soft reservation of one room, created at get_quote and
// consumed at create_booking. Holds prevent two guests from checking out the
// last available room concurrently, and expire on their own when a cart is
// abandoned.
type Hold struct {
	Token      string `json:"token"`
	PropertyID string `json:"property_id"`
	// RoomID stores comma-joined physical room ids for this hold. Public APIs emit room_ids.
	RoomID     string    `json:"room_id"`
	RoomTypeID string    `json:"room_type_id"`
	Checkin    time.Time `json:"checkin"`
	Checkout   time.Time `json:"checkout"`
	ExpiresAt  time.Time `json:"expires_at"`
}

// Offer is an expiring, server-side snapshot of one concrete bookable stay.
// It does not reserve inventory; get_quote revalidates availability before
// creating a hold.
type Offer struct {
	ID             string    `json:"id"`
	SearchID       string    `json:"search_id"`
	StayID         string    `json:"stay_id"`
	PropertyID     string    `json:"property_id"`
	CheckIn        string    `json:"check_in"`
	CheckOut       string    `json:"check_out"`
	Nights         int       `json:"nights"`
	Adults         int       `json:"adults"`
	Children       int       `json:"children"`
	RequestedRooms int       `json:"requested_rooms"`
	RoomIDs        []string  `json:"room_ids"`
	TotalAmount    float64   `json:"total_amount"`
	Currency       string    `json:"currency"`
	ExpiresAt      time.Time `json:"expires_at"`
}

// Overlaps reports whether h covers any night in [checkin, checkout).
// Stays are half-open: a checkout on the same day another stay checks in
// does not overlap.
func (h Hold) Overlaps(checkin, checkout time.Time) bool {
	return h.Checkin.Before(checkout) && checkin.Before(h.Checkout)
}
