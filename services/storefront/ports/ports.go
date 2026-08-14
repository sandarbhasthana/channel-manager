// Package ports declares the outbound interfaces the storefront depends on.
package ports

import (
	"context"
	"encoding/json"

	pmsdomain "github.com/channel-manager/channel-manager/services/pms/domain"
	pricingdomain "github.com/channel-manager/channel-manager/services/pricing/domain"
	resdomain "github.com/channel-manager/channel-manager/services/reservations/domain"
	"github.com/channel-manager/channel-manager/services/storefront/domain"
)

// PropertyLookup resolves a property by internal id or PMS external id.
// Narrower than pms/ports.PropertyRepository: the storefront only reads.
type PropertyLookup interface {
	GetByID(ctx context.Context, id string) (pmsdomain.Property, error)
	GetByExternalID(ctx context.Context, connectionID, externalID string) (pmsdomain.Property, error)
	// BookingEngineEnabled reports whether the direct sales channel is on for a
	// property. The storefront refuses to quote or create bookings when it is off.
	BookingEngineEnabled(ctx context.Context, id string) (bool, error)
	// GetChannelConfig returns the property's booking-engine configuration
	// (enabled + route + percent), which the booking engine reads to decide
	// where to route its own stay actions.
	GetChannelConfig(ctx context.Context, id string) (pmsdomain.ChannelConfig, error)
	// ListListings returns the caller org's active properties with their
	// booking-engine config. A booking engine calls this to discover which
	// properties it may sell and which one is the org default; it cannot use
	// GetChannelConfig for that, since that needs a property id it does not yet
	// have.
	ListListings(ctx context.Context) ([]pmsdomain.PropertyListing, error)
}

// PmsGateway is the subset of the PMS service the storefront calls.
// *pms/usecases.PmsService satisfies this.
type PmsGateway interface {
	SearchAvailability(ctx context.Context, propertyID string, q pmsdomain.AvailabilityQuery) ([]pmsdomain.AvailabilityOffer, error)
	SearchFlexibleAvailability(ctx context.Context, propertyID string, q pmsdomain.FlexibleAvailabilityQuery) (*pmsdomain.FlexibleAvailabilityResult, error)
	GetQuote(ctx context.Context, propertyID string, q pmsdomain.QuoteQuery) (*pmsdomain.Quote, error)
	CreateBooking(ctx context.Context, propertyID string, in pmsdomain.CreateBookingInput) (*pmsdomain.PmsBooking, error)
	GetBooking(ctx context.Context, propertyID string, in pmsdomain.GetBookingInput) (*pmsdomain.PmsBooking, error)
	UpdateBooking(ctx context.Context, propertyID string, in pmsdomain.UpdateBookingInput) (*pmsdomain.PmsBooking, error)
	CancelBooking(ctx context.Context, propertyID string, in pmsdomain.CancelBookingInput) (*pmsdomain.CancelBookingResult, error)
}

// ReservationWriter is the subset of the reservations service the storefront
// calls. *reservations/usecases.ReservationService satisfies this.
type ReservationWriter interface {
	// RecordReservation persists a canonical reservation for a booking that
	// already exists in the PMS (a direct booking made through the storefront).
	// It does NOT publish reservation.created back to the PMS — the PMS already
	// has the booking, so propagating it would create a duplicate reservation.
	RecordReservation(ctx context.Context, res *resdomain.Reservation, idempotencyKey string) (string, bool, error)
	CancelReservation(ctx context.Context, id string) (*resdomain.Reservation, error)
}

// PromoGateway is the subset of the pricing service the storefront calls.
// *pricing/usecases.PromoService satisfies this.
//
// Lookup does not consume a redemption; Redeem does, atomically. The booking
// engine evaluates discounts against Lookup and commits with Redeem.
type PromoGateway interface {
	Lookup(ctx context.Context, code, propertyID string) (pricingdomain.LookupResult, error)
	Redeem(ctx context.Context, code, propertyID string) (pricingdomain.PromoCode, error)
	ReleaseRedemption(ctx context.Context, code string) error
}

// HoldStore persists short-lived soft holds placed at get_quote time.
//
// Implementations must expire holds automatically once Hold.ExpiresAt passes,
// so an abandoned checkout never permanently withholds inventory.
type HoldStore interface {
	// Place stores h until it expires.
	Place(ctx context.Context, h domain.Hold) error
	// Get returns the hold for token, or domain.ErrHoldNotFound.
	Get(ctx context.Context, token string) (domain.Hold, error)
	// Release removes the hold for token. Releasing an unknown token is a no-op.
	Release(ctx context.Context, token string) error
	// ActiveForProperty returns all unexpired holds for a property. Used to
	// subtract in-flight direct checkouts from PMS-reported availability.
	ActiveForProperty(ctx context.Context, propertyID string) ([]domain.Hold, error)
}

// IdempotencyRecord stores the original result of a create_booking mutation.
type IdempotencyRecord struct {
	RequestHash string          `json:"request_hash"`
	Response    json.RawMessage `json:"response"`
}

// IdempotencyStore replays completed create_booking retries.
type IdempotencyStore interface {
	Get(ctx context.Context, key string) (IdempotencyRecord, bool, error)
	Put(ctx context.Context, key string, record IdempotencyRecord) error
}

// AuditEvent is one auditable storefront mutation.
//
// The storefront deliberately does not model an actor type: every caller on
// this ingress authenticates with an org-scoped integration API key, never a
// user session. The adapter fills that in.
type AuditEvent struct {
	OrgID        string
	Action       string
	ResourceType string
	ResourceID   string
	Metadata     map[string]any
}

// AuditRecorder appends to the audit trail.
//
// Recording must never fail a guest's booking, so this returns nothing:
// implementations log their own errors. A nil AuditRecorder disables auditing.
type AuditRecorder interface {
	Record(ctx context.Context, e AuditEvent)
}
