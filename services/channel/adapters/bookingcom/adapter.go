package bookingcom

import (
	"context"
	"time"

	"github.com/channel-manager/channel-manager/services/channel/domain"
)

// Adapter implements the channel adapter for Booking.com.
type Adapter struct{}

// NewAdapter creates a new Booking.com adapter.
func NewAdapter() *Adapter {
	return &Adapter{}
}

func (a *Adapter) ChannelID() string {
	return "bookingcom"
}

func (a *Adapter) Capabilities() []domain.ChannelCapability {
	return []domain.ChannelCapability{
		domain.CapabilityPushAvailability,
		domain.CapabilityPushRates,
		domain.CapabilityFetchReservations,
	}
}

func (a *Adapter) PushAvailability(ctx context.Context, updates []domain.AvailabilityUpdate) error {
	// TODO: implement Booking.com availability push
	return nil
}

func (a *Adapter) PushRates(ctx context.Context, updates []domain.RateUpdate) error {
	// TODO: implement Booking.com rate push
	return nil
}

func (a *Adapter) FetchReservations(ctx context.Context, propertyID string, since time.Time) ([]domain.FetchedReservation, error) {
	// TODO: implement Booking.com reservation fetch
	return nil, nil
}
