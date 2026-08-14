package ports

import (
	"context"
	"time"

	"github.com/channel-manager/channel-manager/services/pms/domain"
)

// BookingEngineClient is the inbound API surface for the MyPMS webhook booking
// engine (PMS_API_REFERENCE.md §1).
type BookingEngineClient interface {
	PmsID() string
	Capabilities() []domain.PmsCapability

	OrgHealth(ctx context.Context) (*domain.OrgHealth, error)
	SearchProperties(ctx context.Context, filter domain.PropertySearchFilter) ([]domain.Property, error)
	PropertyHealth(ctx context.Context, externalPropertyID string) (*domain.PropertyHealth, error)
	ListRoomTypes(ctx context.Context, externalPropertyID string) ([]domain.RoomType, error)
	SearchAvailability(ctx context.Context, externalPropertyID string, q domain.AvailabilityQuery) ([]domain.AvailabilityOffer, error)
	SearchFlexibleAvailability(ctx context.Context, externalPropertyID string, q domain.FlexibleAvailabilityQuery) (*domain.FlexibleAvailabilityResult, error)
	GetInventory(ctx context.Context, externalPropertyID, roomTypeID string, from, to time.Time) ([]domain.InventorySnapshot, error)
	GetQuote(ctx context.Context, externalPropertyID string, q domain.QuoteQuery) (*domain.Quote, error)
	CreateBooking(ctx context.Context, externalPropertyID string, in domain.CreateBookingInput) (*domain.PmsBooking, error)
	GetBooking(ctx context.Context, externalPropertyID string, in domain.GetBookingInput) (*domain.PmsBooking, error)
	ListBookings(ctx context.Context, externalPropertyID string, in domain.ListBookingsInput) (*domain.ListBookingsResult, error)
	UpdateBooking(ctx context.Context, externalPropertyID string, in domain.UpdateBookingInput) (*domain.PmsBooking, error)
	CancelBooking(ctx context.Context, externalPropertyID string, in domain.CancelBookingInput) (*domain.CancelBookingResult, error)
	DeleteBooking(ctx context.Context, externalPropertyID, bookingID string) (*domain.DeleteBookingResult, error)
}

// PmsAdapter is the legacy adapter interface retained for other PMS providers.
type PmsAdapter interface {
	PmsID() string
	Capabilities() []domain.PmsCapability
	ListProperties(ctx context.Context) ([]domain.Property, error)
	ListRoomTypes(ctx context.Context, propertyID string) ([]domain.RoomType, error)
	GetInventory(ctx context.Context, propertyID, roomTypeID string, from, to time.Time) ([]domain.InventorySnapshot, error)
	GetRates(ctx context.Context, propertyID, roomTypeID string, from, to time.Time) ([]domain.RateSnapshot, error)
	GetReservations(ctx context.Context, propertyID string, since time.Time) ([]domain.PmsReservation, error)
}

// ConnectionRepository persists PMS connections.
type ConnectionRepository interface {
	Create(ctx context.Context, conn domain.Connection, credentials map[string]string) (domain.Connection, error)
	GetByID(ctx context.Context, id string) (domain.Connection, error)
	ListByOrg(ctx context.Context, orgID string) ([]domain.Connection, error)
	UpdateStatus(ctx context.Context, id, status, lastError string) error
	UpdateLastSync(ctx context.Context, id string, t time.Time) error
	Delete(ctx context.Context, id string) error
}

// PropertyRepository persists canonical properties.
type PropertyRepository interface {
	Upsert(ctx context.Context, p domain.Property) (domain.Property, error)
	GetByID(ctx context.Context, id string) (domain.Property, error)
	GetByExternalID(ctx context.Context, connectionID, externalID string) (domain.Property, error)
	ListByConnection(ctx context.Context, connectionID string) ([]domain.Property, error)
	ListByOrg(ctx context.Context, orgID string) ([]domain.Property, error)
}

// RoomTypeRepository persists canonical room types.
type RoomTypeRepository interface {
	Upsert(ctx context.Context, rt domain.RoomType) (domain.RoomType, error)
	ListByProperty(ctx context.Context, propertyID string) ([]domain.RoomType, error)
	GetByExternalID(ctx context.Context, propertyID, externalID string) (domain.RoomType, error)
}

// RoomRepository persists canonical physical rooms.
type RoomRepository interface {
	Upsert(ctx context.Context, r domain.Room) (domain.Room, error)
	ListByProperty(ctx context.Context, propertyID string) ([]domain.Room, error)
	ListByRoomType(ctx context.Context, roomTypeID string) ([]domain.Room, error)
}

// SecretResolver stores connection credentials.
type SecretResolver interface {
	Store(ctx context.Context, ref string, creds map[string]string) (string, error)
	Resolve(ctx context.Context, ref string) (map[string]string, error)
}

// InventoryWriter upserts canonical inventory from PMS ingestion.
type InventoryWriter interface {
	BulkUpsertFromPMS(ctx context.Context, orgID string, days []InventoryDayInput) (rowsAffected int32, eventID string, err error)
}

// InventoryDayInput is a simplified inventory row for ingestion.
type InventoryDayInput struct {
	RoomTypeID string
	StayDate   time.Time
	Available  int
	StopSell   bool
}
