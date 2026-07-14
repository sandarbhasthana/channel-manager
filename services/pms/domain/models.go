package domain

import "time"

// PmsCapability represents a feature supported by a PMS adapter.
type PmsCapability string

const (
	CapabilityListProperties      PmsCapability = "list_properties"
	CapabilityListRoomTypes       PmsCapability = "list_room_types"
	CapabilityGetInventory        PmsCapability = "get_inventory"
	CapabilityGetRates            PmsCapability = "get_rates"
	CapabilityGetReservations     PmsCapability = "get_reservations"
	CapabilityPushReservations    PmsCapability = "push_reservations"
	CapabilityPushInventory       PmsCapability = "push_inventory"
	CapabilityChangeFeed          PmsCapability = "change_feed"
	CapabilitySearchAvailability  PmsCapability = "search_availability"
	CapabilityGetQuote            PmsCapability = "get_quote"
	CapabilityCreateBooking       PmsCapability = "create_booking"
	CapabilityGetBooking          PmsCapability = "get_booking"
	CapabilityUpdateBooking       PmsCapability = "update_booking"
	CapabilityCancelBooking       PmsCapability = "cancel_booking"
)

// Connection is a tenant's link to a PMS account.
type Connection struct {
	ID         string
	OrgID      string
	Provider   string
	Name       string
	Status     string
	SecretRef  string
	Config     map[string]string
	LastSyncAt *time.Time
	LastError  string
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// Property is the canonical hotel listing as known to Channel Manager.
type Property struct {
	ID                 string
	OrgID              string
	ConnectionID       string
	ExternalID         string
	Name               string
	Timezone           string
	DefaultCurrency    string
	City               string
	Country            string
	IsActive           bool
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

// ChannelConfig is a property's booking-engine configuration, which the booking
// engine reads to decide where to route its stay actions. Route is "pms" or
// "cm"; Percent is the 0–100 canary ramp for the "cm" route.
type ChannelConfig struct {
	Enabled bool
	Route   string
	Percent int
}

// PropertySearchFilter filters search_properties.
type PropertySearchFilter struct {
	City    string
	Country string
	Name    string
}

// OrgHealth is returned by the PMS org-level health check.
type OrgHealth struct {
	Status           string
	Service          string
	OrganizationID   string
	AvailableActions []string
}

// PropertyHealth is returned by the PMS property-level health check.
type PropertyHealth struct {
	Status           string
	Service          string
	Property         Property
	AvailableActions []string
}

// RoomType is a sellable unit category within a property.
type RoomType struct {
	ID                 string
	OrgID              string
	PropertyID         string
	ExternalPropertyID string
	ExternalID         string
	Code               string
	Name               string
	MaxOccupancy       int
	BaseOccupancy      int
	Rooms              []Room
	IsActive           bool
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

// Room is a physical room within a RoomType.
type Room struct {
	ID           string
	OrgID        string
	PropertyID   string
	RoomTypeID   string
	ExternalID   string
	Name         string
	IsActive     bool
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// AvailabilityQuery parameters for search_availability.
type AvailabilityQuery struct {
	Checkin      time.Time
	Checkout     time.Time
	Adults       int
	Children     int
	Rooms        int
	RoomTypeName string
}

// AvailabilityOffer is one bookable option from search_availability.
type AvailabilityOffer struct {
	RoomID         string
	RoomTypeID     string
	RoomTypeName   string
	AvailableUnits int
	IsAvailable    bool
	PricePerNight  float64
	TotalPrice     float64
	Currency       string
	Capacity       int
}

// QuoteQuery parameters for get_quote.
type QuoteQuery struct {
	RoomID   string
	Checkin  time.Time
	Checkout time.Time
	Adults   int
}

// Quote is a price quote for a stay.
type Quote struct {
	RoomID        string
	RoomName      string
	RoomType      string
	Nights        int
	Adults        int
	Capacity      int
	PricePerNight float64
	TotalPrice    float64
	Currency      string
	IsAvailable   bool
}

// CreateBookingInput parameters for create_booking.
type CreateBookingInput struct {
	RoomID    string
	Checkin   time.Time
	Checkout  time.Time
	GuestName string
	Email     string
	Phone     string
	Adults    int
	Children  int
	Notes     string
}

// UpdateBookingInput parameters for update_booking.
type UpdateBookingInput struct {
	BookingID string
	Checkin   *time.Time
	Checkout  *time.Time
	GuestName string
	Email     string
	Phone     string
	Adults    *int
	Children  *int
	Notes     string
	RoomID    string
}

// ListBookingsInput parameters for list_bookings.
type ListBookingsInput struct {
	Status    string
	StartDate *time.Time
	EndDate   *time.Time
}

// ListBookingsResult is returned by list_bookings.
type ListBookingsResult struct {
	Bookings []PmsBooking
	Count    int
}

// PmsBooking is a reservation as returned by the PMS booking engine.
type PmsBooking struct {
	BookingID     string
	Status        string
	GuestName     string
	Email         string
	Phone         string
	RoomID        string
	RoomName      string
	RoomType      string
	PropertyName  string
	Checkin       string
	Checkout      string
	Adults        int
	Children      int
	Notes         string
	PaymentStatus string
	Source        string
	Message       string
}

// CancelBookingResult is returned by cancel_booking.
type CancelBookingResult struct {
	BookingID string
	Status    string
	Message   string
}

// DeleteBookingResult is returned by delete_booking.
type DeleteBookingResult struct {
	BookingID string
	Status    string
	Message   string
}

// InventorySnapshot represents a point-in-time inventory record from a PMS.
type InventorySnapshot struct {
	ExternalRoomTypeID string
	Date               time.Time
	Available          int
}

// RateSnapshot represents a point-in-time rate record from a PMS.
type RateSnapshot struct {
	RoomTypeID string
	RatePlanID string
	Date       time.Time
	Amount     float64
	Currency   string
}

// PmsReservation represents a reservation as seen by a PMS (legacy adapter surface).
type PmsReservation struct {
	ID        string
	GuestName string
	CheckIn   time.Time
	CheckOut  time.Time
	Status    string
}

// ChangeEvent represents a change notification from a PMS change feed.
type ChangeEvent struct {
	Type      string
	Resource  string
	ID        string
	Timestamp time.Time
}

// SyncCatalogResult summarizes a catalog sync from the PMS.
type SyncCatalogResult struct {
	PropertiesSynced  int
	RoomTypesSynced   int
}

// IngestAvailabilityResult summarizes availability ingestion.
type IngestAvailabilityResult struct {
	InventoryRowsAffected int32
	EventID               string
}
