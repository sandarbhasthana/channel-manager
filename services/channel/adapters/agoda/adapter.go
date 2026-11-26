package agoda

import (
	"context"
	"time"

	"github.com/channel-manager/channel-manager/services/channel/domain"
)

// Adapter implements the channel adapter for Agoda.
type Adapter struct{}

// NewAdapter creates a new Agoda adapter.
func NewAdapter() *Adapter {
	return &Adapter{}
}

func (a *Adapter) ChannelID() string {
	return "agoda"
}

// Capabilities is empty: this adapter is a placeholder. With no
// capabilities the channel service answers every push with
// domain.ErrNotImplemented instead of recording a sync that never reached
// Agoda.
func (a *Adapter) Capabilities() []domain.ChannelCapability {
	return nil
}

func (a *Adapter) PushAvailability(ctx context.Context, updates []domain.AvailabilityUpdate) error {
	// TODO: implement Agoda availability push
	return domain.ErrNotImplemented
}

func (a *Adapter) PushRates(ctx context.Context, updates []domain.RateUpdate) error {
	// TODO: implement Agoda rate push
	return domain.ErrNotImplemented
}

func (a *Adapter) FetchReservations(ctx context.Context, propertyID string, since time.Time) ([]domain.FetchedReservation, error) {
	// TODO: implement Agoda reservation fetch
	return nil, domain.ErrNotImplemented
}
