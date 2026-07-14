// Package domain holds the booking-engine read model and settings.
//
// The booking engine is the direct (own-website) sales channel. Channel
// Manager owns it; this package exposes a read view over direct-channel
// reservations plus the per-property on/off switch. Reservations are written
// by the storefront ingress, never here.
package domain

import (
	"errors"
	"time"
)

// Source is the reservations.metadata "source" value that marks a booking as
// direct rather than originating from an OTA. It matches the storefront's
// domain.DirectChannel constant; the two must agree or the dashboard shows an
// empty list. Duplicated rather than imported to avoid a dependency from the
// booking engine onto the storefront.
const Source = "direct"

// ErrPropertyNotFound is returned when a property does not exist in the caller's
// org (or is hidden from it by RLS).
var ErrPropertyNotFound = errors.New("bookingengine: property not found")

// ErrInvalidSettings is returned when an update carries an out-of-range value.
var ErrInvalidSettings = errors.New("bookingengine: invalid settings")

// DirectReservation is a booking made through the storefront, projected for the
// dashboard. It is not the canonical reservation record.
type DirectReservation struct {
	ID               string
	PropertyID       string
	ConfirmationCode string
	GuestName        string
	CheckIn          time.Time
	CheckOut         time.Time
	Status           string
	TotalMinor       int64
	Currency         string
	BookedAt         time.Time
}

// Settings is the per-property booking-engine configuration, managed from the
// CM dashboard.
type Settings struct {
	PropertyID           string
	DirectChannelEnabled bool
	// Route is "pms" or "cm"; Percent is the 0–100 canary ramp for "cm".
	Route   string
	Percent int
}
