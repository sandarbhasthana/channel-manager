package events

import (
	"context"

	"github.com/channel-manager/channel-manager/services/reservations/domain"
)

// NoopPublisher is a dev publisher that discards reservation events.
type NoopPublisher struct{}

func (NoopPublisher) PublishReservationCreated(_ context.Context, _ *domain.Reservation) error {
	return nil
}

func (NoopPublisher) PublishReservationUpdated(_ context.Context, _ *domain.Reservation) error {
	return nil
}
