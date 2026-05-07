package ports

import (
	"context"
	"time"

	"github.com/channel-manager/channel-manager/services/pms/domain"
)

// PmsAdapter is the primary interface every PMS adapter must implement.
type PmsAdapter interface {
	// PmsID returns the unique identifier for this PMS.
	PmsID() string

	// Capabilities returns the set of features this adapter supports.
	Capabilities() []domain.PmsCapability

	// ListProperties returns all properties accessible via this PMS.
	ListProperties(ctx context.Context) ([]domain.Property, error)

	// ListRoomTypes returns all room types for a property.
	ListRoomTypes(ctx context.Context, propertyID string) ([]domain.RoomType, error)

	// GetInventory returns inventory snapshots for a date range.
	GetInventory(ctx context.Context, propertyID, roomTypeID string, from, to time.Time) ([]domain.InventorySnapshot, error)

	// GetRates returns rate snapshots for a date range.
	GetRates(ctx context.Context, propertyID, roomTypeID string, from, to time.Time) ([]domain.RateSnapshot, error)

	// GetReservations returns reservations modified since the given time.
	GetReservations(ctx context.Context, propertyID string, since time.Time) ([]domain.PmsReservation, error)
}

// ReservationPusher pushes reservations into a PMS.
type ReservationPusher interface {
	PushReservation(ctx context.Context, propertyID string, reservation *domain.PmsReservation) error
}

// InventoryPusher pushes inventory updates into a PMS.
type InventoryPusher interface {
	PushInventory(ctx context.Context, propertyID string, snapshots []domain.InventorySnapshot) error
}

// ChangeFeed provides a stream of change events from a PMS.
type ChangeFeed interface {
	Subscribe(ctx context.Context, propertyID string) (<-chan domain.ChangeEvent, error)
}
