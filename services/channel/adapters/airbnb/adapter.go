package airbnb

import (
	"context"
	"time"

	"github.com/channel-manager/channel-manager/services/channel/domain"
)

// Adapter implements the channel adapter for Airbnb.
type Adapter struct{}

// NewAdapter creates a new Airbnb adapter.
func NewAdapter() *Adapter {
	return &Adapter{}
}

func (a *Adapter) ChannelID() string {
	return "airbnb"
}

func (a *Adapter) Capabilities() []domain.ChannelCapability {
	return []domain.ChannelCapability{
		domain.CapabilityPushAvailability,
		domain.CapabilityPushRates,
		domain.CapabilityFetchReservations,
	}
}

func (a *Adapter) PushAvailability(ctx context.Context, updates []domain.AvailabilityUpdate) error {
	// TODO: implement Airbnb availability push
	return nil
}

func (a *Adapter) PushRates(ctx context.Context, updates []domain.RateUpdate) error {
	// TODO: implement Airbnb rate push
	return nil
}

func (a *Adapter) FetchReservations(ctx context.Context, propertyID string, since time.Time) ([]domain.FetchedReservation, error) {
	// TODO: implement Airbnb reservation fetch
	return nil, nil
}
