package domain

import "time"

// PmsCapability represents a feature supported by a PMS adapter.
type PmsCapability string

const (
	CapabilityListProperties    PmsCapability = "list_properties"
	CapabilityListRoomTypes     PmsCapability = "list_room_types"
	CapabilityGetInventory      PmsCapability = "get_inventory"
	CapabilityGetRates          PmsCapability = "get_rates"
	CapabilityGetReservations   PmsCapability = "get_reservations"
	CapabilityPushReservations  PmsCapability = "push_reservations"
	CapabilityPushInventory     PmsCapability = "push_inventory"
	CapabilityChangeFeed        PmsCapability = "change_feed"
)

// Property represents a hotel property in a PMS.
type Property struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// RoomType represents a room type in a PMS.
type RoomType struct {
	ID         string `json:"id"`
	PropertyID string `json:"property_id"`
	Name       string `json:"name"`
	MaxOccup   int    `json:"max_occupancy"`
}

// InventorySnapshot represents a point-in-time inventory record from a PMS.
type InventorySnapshot struct {
	RoomTypeID string    `json:"room_type_id"`
	Date       time.Time `json:"date"`
	Available  int       `json:"available"`
}

// RateSnapshot represents a point-in-time rate record from a PMS.
type RateSnapshot struct {
	RoomTypeID string    `json:"room_type_id"`
	RatePlanID string    `json:"rate_plan_id"`
	Date       time.Time `json:"date"`
	Amount     float64   `json:"amount"`
	Currency   string    `json:"currency"`
}

// PmsReservation represents a reservation as seen by a PMS.
type PmsReservation struct {
	ID        string    `json:"id"`
	GuestName string    `json:"guest_name"`
	CheckIn   time.Time `json:"check_in"`
	CheckOut  time.Time `json:"check_out"`
	Status    string    `json:"status"`
}

// ChangeEvent represents a change notification from a PMS change feed.
type ChangeEvent struct {
	Type      string    `json:"type"`
	Resource  string    `json:"resource"`
	ID        string    `json:"id"`
	Timestamp time.Time `json:"timestamp"`
}
