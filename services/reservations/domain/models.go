package domain

import (
	"encoding/json"
	"time"
)

// Reservation represents a booking from an OTA or direct channel.
type Reservation struct {
	ID                    string          `json:"id"`
	OrgID                 string          `json:"org_id"`
	PropertyID            string          `json:"property_id"`
	ExternalPropertyID    string          `json:"external_property_id"`
	RoomTypeID            string          `json:"room_type_id"`
	ChannelID             string          `json:"channel_id"`
	GuestName             string          `json:"guest_name"`
	CheckIn               time.Time       `json:"check_in"`
	CheckOut              time.Time       `json:"check_out"`
	Status                string          `json:"status"`
	TotalAmount           float64         `json:"total_amount"`
	Currency              string          `json:"currency"`
	ChannelConfirmationID string          `json:"channel_confirmation_id"`
	// Party size as the channel reported it. Zero means "not reported"; the
	// webhook publisher then omits it rather than inventing a number.
	Adults   int `json:"adults,omitempty"`
	Children int `json:"children,omitempty"`
	// The channel's own room-type id, kept alongside RoomTypeID so the PMS can
	// re-map when RoomTypeID is already an internal id (direct bookings) or an
	// external one (OTA pulls).
	ChannelRoomTypeID string          `json:"channel_room_type_id,omitempty"`
	RatePlanID        string          `json:"rate_plan_id,omitempty"`
	RawPayload        json.RawMessage `json:"raw_payload,omitempty"`
}
