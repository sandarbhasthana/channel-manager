package csv

import (
	"context"
	"time"

	"github.com/channel-manager/channel-manager/services/pms/domain"
)

// Adapter implements the PMS adapter for CSV file imports.
type Adapter struct{}

// NewAdapter creates a new CSV adapter.
func NewAdapter() *Adapter {
	return &Adapter{}
}

func (a *Adapter) PmsID() string { return "csv" }

func (a *Adapter) Capabilities() []domain.PmsCapability {
	return []domain.PmsCapability{
		domain.CapabilityListProperties,
		domain.CapabilityListRoomTypes,
		domain.CapabilityGetInventory,
		domain.CapabilityGetRates,
	}
}

func (a *Adapter) ListProperties(ctx context.Context) ([]domain.Property, error) {
	// TODO: implement
	return nil, nil
}

func (a *Adapter) ListRoomTypes(ctx context.Context, propertyID string) ([]domain.RoomType, error) {
	// TODO: implement
	return nil, nil
}

func (a *Adapter) GetInventory(ctx context.Context, propertyID, roomTypeID string, from, to time.Time) ([]domain.InventorySnapshot, error) {
	// TODO: implement
	return nil, nil
}

func (a *Adapter) GetRates(ctx context.Context, propertyID, roomTypeID string, from, to time.Time) ([]domain.RateSnapshot, error) {
	// TODO: implement
	return nil, nil
}

func (a *Adapter) GetReservations(ctx context.Context, propertyID string, since time.Time) ([]domain.PmsReservation, error) {
	// TODO: implement
	return nil, nil
}
