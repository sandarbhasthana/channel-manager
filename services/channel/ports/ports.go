package ports

import (
	"context"
	"time"

	"github.com/channel-manager/channel-manager/services/channel/domain"
)

// ChannelAdapter is the primary interface every OTA/channel adapter must implement.
type ChannelAdapter interface {
	// ChannelID returns the unique identifier for this channel.
	ChannelID() string

	// Capabilities returns the set of features this adapter supports.
	Capabilities() []domain.ChannelCapability
}

// AvailabilityPusher pushes availability updates to a channel.
type AvailabilityPusher interface {
	PushAvailability(ctx context.Context, updates []domain.AvailabilityUpdate) error
}

// RatePusher pushes rate updates to a channel.
type RatePusher interface {
	PushRates(ctx context.Context, updates []domain.RateUpdate) error
}

// ReservationFetcher fetches reservations from a channel.
type ReservationFetcher interface {
	FetchReservations(ctx context.Context, propertyID string, since time.Time) ([]domain.FetchedReservation, error)
}
