package ports

import (
	"context"

	"github.com/channel-manager/channel-manager/services/reservations/domain"
)

// ReservationRepository provides persistence for reservations.
type ReservationRepository interface {
	GetByID(ctx context.Context, id string) (*domain.Reservation, error)
	ListByProperty(ctx context.Context, propertyID string) ([]domain.Reservation, error)
	Save(ctx context.Context, reservation *domain.Reservation) error
	UpdateStatus(ctx context.Context, id, status string) error
}

// ReservationEventPublisher publishes reservation events.
type ReservationEventPublisher interface {
	PublishReservationCreated(ctx context.Context, reservation *domain.Reservation) error
	PublishReservationUpdated(ctx context.Context, reservation *domain.Reservation) error
}
