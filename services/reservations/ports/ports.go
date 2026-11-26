package ports

import (
	"context"

	"github.com/channel-manager/channel-manager/services/reservations/domain"
)

// ReservationRepository provides persistence for reservations.
type ReservationRepository interface {
	GetByID(ctx context.Context, id string) (*domain.Reservation, error)
	ListByProperty(ctx context.Context, propertyID string) ([]domain.Reservation, error)
	// Save inserts a booking, or updates the one with the same channel
	// connection + external id, and says which it did.
	Save(ctx context.Context, reservation *domain.Reservation) (domain.SaveResult, error)
	UpdateStatus(ctx context.Context, id, status string) error
	// MarkDelivery records whether the PMS took an announced booking.
	MarkDelivery(ctx context.Context, id string, delivered bool, lastError string) error
	// ListUndelivered returns a property's bookings the PMS hasn't taken yet.
	ListUndelivered(ctx context.Context, propertyID string) ([]domain.Reservation, error)
}

// ReservationEventPublisher publishes reservation events.
type ReservationEventPublisher interface {
	PublishReservationCreated(ctx context.Context, reservation *domain.Reservation) error
	PublishReservationUpdated(ctx context.Context, reservation *domain.Reservation) error
}
