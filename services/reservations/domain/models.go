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
	RawPayload            json.RawMessage `json:"raw_payload,omitempty"`
}
