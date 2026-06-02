// Package mypms implements the channel-manager client for the MyPMS booking
// engine webhook API (docs/PMS_API_REFERENCE.md §1).
package mypms

import (
	"encoding/json"
	"fmt"
)

// Action names for POST /api/webhooks/bookings[/{propertyId}].
const (
	ActionSearchProperties   = "search_properties"
	ActionSearchAvailability = "search_availability"
	ActionGetRoomDetails     = "get_room_details"
	ActionGetQuote           = "get_quote"
	ActionCreateBooking        = "create_booking"
	ActionGetBooking           = "get_booking"
	ActionUpdateBooking        = "update_booking"
	ActionCancelBooking        = "cancel_booking"
	ActionListBookings         = "list_bookings"
)

// OrgHealthResponse is returned by GET /api/webhooks/bookings.
type OrgHealthResponse struct {
	Status            string   `json:"status"`
	Service           string   `json:"service"`
	AvailableActions  []string `json:"available_actions"`
	OrganizationID    string   `json:"organization_id"`
	BookingActions    []string `json:"booking_actions"`
}

// PropertyHealthResponse is returned by GET /api/webhooks/bookings/{propertyId}.
type PropertyHealthResponse struct {
	Status           string          `json:"status"`
	Service          string          `json:"service"`
	Property         PropertySummary `json:"property"`
	AvailableActions []string        `json:"available_actions"`
}

// PropertySummary is the property block in health / search responses.
type PropertySummary struct {
	PropertyID string `json:"property_id"`
	Name       string `json:"name"`
	City       string `json:"city"`
	Country    string `json:"country"`
	Currency   string `json:"currency"`
}

// SearchPropertiesRequest is the body for action search_properties.
type SearchPropertiesRequest struct {
	Action  string `json:"action"`
	City    string `json:"city,omitempty"`
	Country string `json:"country,omitempty"`
	Name    string `json:"name,omitempty"`
}

// SearchPropertiesResponse wraps the property list from the PMS.
type SearchPropertiesResponse struct {
	Properties []PropertySummary `json:"properties"`
	// Some deployments return a bare array; UnmarshalJSON handles both.
}

func (r *SearchPropertiesResponse) UnmarshalJSON(data []byte) error {
	var wrapped struct {
		Properties []PropertySummary `json:"properties"`
		Data       json.RawMessage   `json:"data"`
	}
	if err := json.Unmarshal(data, &wrapped); err == nil {
		if len(wrapped.Properties) > 0 {
			r.Properties = wrapped.Properties
			return nil
		}
		if len(wrapped.Data) > 0 {
			var arr []PropertySummary
			if err := json.Unmarshal(wrapped.Data, &arr); err == nil {
				r.Properties = arr
				return nil
			}
			var nested struct {
				Properties []PropertySummary `json:"properties"`
			}
			if err := json.Unmarshal(wrapped.Data, &nested); err == nil && len(nested.Properties) > 0 {
				r.Properties = nested.Properties
				return nil
			}
		}
	}
	var arr []PropertySummary
	if err := json.Unmarshal(data, &arr); err == nil {
		r.Properties = arr
		return nil
	}
	return json.Unmarshal(data, &wrapped)
}

// SearchAvailabilityRequest is the body for action search_availability.
type SearchAvailabilityRequest struct {
	Action    string `json:"action"`
	Checkin   string `json:"checkin"`
	Checkout  string `json:"checkout"`
	Adults    int    `json:"adults"`
	Children  int    `json:"children"`
	Rooms     int    `json:"rooms"`
	RoomType  string `json:"room_type,omitempty"`
}

// AvailabilityRoom is one bookable unit in a search_availability response.
type AvailabilityRoom struct {
	RoomID       string  `json:"room_id"`
	RoomName     string  `json:"room_name"`
	RoomTypeID   string  `json:"room_type_id"`
	RoomType     string  `json:"room_type"`
	RoomTypeName string  `json:"room_type_name"`
	Available    bool    `json:"is_available"`
	AvailableQty int     `json:"available"`
	Capacity     int     `json:"capacity"`
	PricePerNight float64 `json:"price_per_night"`
	TotalPrice   float64 `json:"total_price"`
	Currency     string  `json:"currency"`
}

// SearchAvailabilityResponse wraps availability results.
type SearchAvailabilityResponse struct {
	Rooms []AvailabilityRoom `json:"rooms"`
	Data  json.RawMessage    `json:"data"`
}

func (r *SearchAvailabilityResponse) RoomsList() []AvailabilityRoom {
	if len(r.Rooms) > 0 {
		return r.Rooms
	}
	if len(r.Data) > 0 {
		var arr []AvailabilityRoom
		if err := json.Unmarshal(r.Data, &arr); err == nil {
			return arr
		}
		var nested struct {
			Rooms []AvailabilityRoom `json:"rooms"`
		}
		if err := json.Unmarshal(r.Data, &nested); err == nil && len(nested.Rooms) > 0 {
			return nested.Rooms
		}
	}
	return nil
}

// GetRoomDetailsRequest is the body for action get_room_details.
type GetRoomDetailsRequest struct {
	Action     string `json:"action"`
	RoomID     string `json:"room_id,omitempty"`
	RoomTypeID string `json:"room_type_id,omitempty"`
}

// RoomTypeDetail describes a sellable room category from the PMS.
type RoomTypeDetail struct {
	ID           string `json:"id"`
	RoomTypeID   string `json:"room_type_id"`
	RoomType     string `json:"room_type"`
	Name         string `json:"name"`
	MaxOccupancy int    `json:"max_occupancy"`
	BaseOccupancy int   `json:"base_occupancy"`
	Capacity      int          `json:"capacity"`
	Description   string       `json:"description"`
	Rooms         []RoomDetail `json:"rooms"`
}

// RoomDetail describes a physical room from the PMS.
type RoomDetail struct {
	ID         string `json:"id"`
	RoomID     string `json:"room_id"`
	RoomTypeID string `json:"room_type_id"`
	Name       string `json:"name"`
}

func (r *RoomDetail) GetID() string {
	if r.RoomID != "" {
		return r.RoomID
	}
	return r.ID
}

// GetRoomDetailsResponse is returned by get_room_details.
type GetRoomDetailsResponse struct {
	RoomTypes []RoomTypeDetail `json:"room_types"`
	Rooms     []RoomDetail     `json:"rooms"`
	RoomType  *RoomTypeDetail  `json:"room_type"`
	Room      *RoomDetail      `json:"room"`
	Data      json.RawMessage  `json:"data"`
}

// RoomTypesList returns all room types from the response regardless of shape.
func (r *GetRoomDetailsResponse) RoomTypesList() []RoomTypeDetail {
	if len(r.RoomTypes) > 0 {
		return r.RoomTypes
	}
	if r.RoomType != nil {
		return []RoomTypeDetail{*r.RoomType}
	}
	if len(r.Data) > 0 {
		var nested struct {
			RoomTypes []RoomTypeDetail `json:"room_types"`
			RoomType  *RoomTypeDetail  `json:"room_type"`
		}
		if err := json.Unmarshal(r.Data, &nested); err == nil {
			if len(nested.RoomTypes) > 0 {
				return nested.RoomTypes
			}
			if nested.RoomType != nil {
				return []RoomTypeDetail{*nested.RoomType}
			}
		}
	}
	return nil
}

// RoomsList returns all rooms from the response regardless of shape.
func (r *GetRoomDetailsResponse) RoomsList() []RoomDetail {
	if len(r.Rooms) > 0 {
		return r.Rooms
	}
	if r.Room != nil {
		return []RoomDetail{*r.Room}
	}
	if len(r.Data) > 0 {
		var nested struct {
			Rooms []RoomDetail `json:"rooms"`
			Room  *RoomDetail  `json:"room"`
		}
		if err := json.Unmarshal(r.Data, &nested); err == nil {
			if len(nested.Rooms) > 0 {
				return nested.Rooms
			}
			if nested.Room != nil {
				return []RoomDetail{*nested.Room}
			}
		}
	}
	return nil
}

// GetQuoteRequest is the body for action get_quote.
type GetQuoteRequest struct {
	Action   string `json:"action"`
	RoomID   string `json:"room_id"`
	Checkin  string `json:"checkin"`
	Checkout string `json:"checkout"`
	Adults   int    `json:"adults"`
}

// Quote is returned by get_quote.
type Quote struct {
	RoomID        string  `json:"room_id"`
	RoomName      string  `json:"room_name"`
	RoomType      string  `json:"room_type"`
	Checkin       string  `json:"checkin"`
	Checkout      string  `json:"checkout"`
	Nights        int     `json:"nights"`
	Adults        int     `json:"adults"`
	Capacity      int     `json:"capacity"`
	PricePerNight float64 `json:"price_per_night"`
	TotalPrice    float64 `json:"total_price"`
	Currency      string  `json:"currency"`
	IsAvailable   bool    `json:"is_available"`
}

// CreateBookingRequest is the body for action create_booking.
type CreateBookingRequest struct {
	Action    string `json:"action"`
	RoomID    string `json:"room_id"`
	Checkin   string `json:"checkin"`
	Checkout  string `json:"checkout"`
	GuestName string `json:"guest_name"`
	Email     string `json:"email,omitempty"`
	Phone     string `json:"phone,omitempty"`
	Adults    int    `json:"adults"`
	Children  int    `json:"children"`
	Notes     string `json:"notes,omitempty"`
}

// Booking is the canonical booking shape from the PMS.
type Booking struct {
	BookingID     string `json:"booking_id"`
	Status        string `json:"status"`
	GuestName     string `json:"guest_name"`
	Email         string `json:"email"`
	Phone         string `json:"phone"`
	RoomID        string `json:"room_id"`
	RoomName      string `json:"room_name"`
	RoomType      string `json:"room_type"`
	PropertyName  string `json:"property_name"`
	Checkin       string `json:"checkin"`
	Checkout      string `json:"checkout"`
	Adults        int    `json:"adults"`
	Children      int    `json:"children"`
	Notes         string `json:"notes"`
	PaymentStatus string `json:"payment_status"`
	Source        string `json:"source"`
	Message       string `json:"message"`
}

// CreateBookingResponse wraps a created booking.
type CreateBookingResponse struct {
	Data Booking `json:"data"`
}

// GetBookingRequest is the body for action get_booking.
type GetBookingRequest struct {
	Action    string `json:"action"`
	BookingID string `json:"booking_id"`
}

// UpdateBookingRequest is the body for action update_booking.
type UpdateBookingRequest struct {
	Action    string `json:"action"`
	BookingID string `json:"booking_id"`
	Checkin   string `json:"checkin,omitempty"`
	Checkout  string `json:"checkout,omitempty"`
	GuestName string `json:"guest_name,omitempty"`
	Email     string `json:"email,omitempty"`
	Phone     string `json:"phone,omitempty"`
	Adults    *int   `json:"adults,omitempty"`
	Children  *int   `json:"children,omitempty"`
	Notes     string `json:"notes,omitempty"`
	RoomID    string `json:"room_id,omitempty"`
}

// CancelBookingRequest is the body for action cancel_booking.
type CancelBookingRequest struct {
	Action    string `json:"action"`
	BookingID string `json:"booking_id"`
	Reason    string `json:"reason,omitempty"`
}

// CancelBookingResponse is returned by cancel_booking.
type CancelBookingResponse struct {
	BookingID string `json:"booking_id"`
	Status    string `json:"status"`
	Message   string `json:"message"`
}

// ListBookingsRequest is the body for action list_bookings.
type ListBookingsRequest struct {
	Action    string `json:"action"`
	Status    string `json:"status,omitempty"`
	StartDate string `json:"start_date,omitempty"`
	EndDate   string `json:"end_date,omitempty"`
}

// ListBookingsResponse wraps a list of bookings.
type ListBookingsResponse struct {
	Data struct {
		Bookings []Booking `json:"bookings"`
		Count    int       `json:"count"`
	} `json:"data"`
}

// APIError is a structured error returned by the PMS.
type APIError struct {
	StatusCode int
	Message    string
	Body       string
}

func (e *APIError) Error() string {
	if e.StatusCode > 0 {
		return fmt.Sprintf("mypms: HTTP %d: %s", e.StatusCode, e.Message)
	}
	return "mypms: " + e.Message
}
