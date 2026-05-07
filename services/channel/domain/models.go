package domain

import "time"

// ChannelCapability represents a feature supported by a channel adapter.
type ChannelCapability string

const (
	CapabilityPushAvailability  ChannelCapability = "push_availability"
	CapabilityPushRates         ChannelCapability = "push_rates"
	CapabilityFetchReservations ChannelCapability = "fetch_reservations"
	CapabilityPushReservations  ChannelCapability = "push_reservations"
)

// AvailabilityUpdate represents an availability push payload.
type AvailabilityUpdate struct {
	PropertyID string    `json:"property_id"`
	RoomTypeID string    `json:"room_type_id"`
	Date       time.Time `json:"date"`
	Available  int       `json:"available"`
	StopSell   bool      `json:"stop_sell"`
	MinStay    int       `json:"min_stay"`
	MaxStay    int       `json:"max_stay"`
}

// RateUpdate represents a rate push payload.
type RateUpdate struct {
	PropertyID string    `json:"property_id"`
	RoomTypeID string    `json:"room_type_id"`
	RatePlanID string    `json:"rate_plan_id"`
	Date       time.Time `json:"date"`
	BaseRate   float64   `json:"base_rate"`
	Currency   string    `json:"currency"`
}

// FetchedReservation represents a reservation fetched from a channel.
type FetchedReservation struct {
	ChannelConfirmationID string    `json:"channel_confirmation_id"`
	GuestName             string    `json:"guest_name"`
	RoomTypeExternalID    string    `json:"room_type_external_id"`
	CheckIn               time.Time `json:"check_in"`
	CheckOut              time.Time `json:"check_out"`
	Status                string    `json:"status"`
	TotalAmount           float64   `json:"total_amount"`
	Currency              string    `json:"currency"`
}
