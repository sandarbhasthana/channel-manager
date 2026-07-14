// Package ports declares the interfaces the booking-engine use cases depend on.
package ports

import (
	"context"

	"github.com/channel-manager/channel-manager/services/bookingengine/domain"
)

// Repository reads direct-channel reservations and reads/writes the per-property
// booking-engine settings. Every method is tenant-scoped via the auth context.
type Repository interface {
	// ListDirectReservations returns direct bookings for a property, most
	// recent first, limited to limit rows starting at offset.
	ListDirectReservations(ctx context.Context, propertyID string, limit, offset int) ([]domain.DirectReservation, error)
	// GetSettings returns the property's booking-engine settings, or
	// ErrPropertyNotFound if the property is not in the caller's org.
	GetSettings(ctx context.Context, propertyID string) (domain.Settings, error)
	// UpdateSettings writes the booking-engine settings and returns them as
	// persisted, or ErrPropertyNotFound.
	UpdateSettings(ctx context.Context, in domain.Settings) (domain.Settings, error)
}
