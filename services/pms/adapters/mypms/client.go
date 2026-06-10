package mypms

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const defaultTimeout = 30 * time.Second

// Config holds connection settings for the MyPMS booking engine API.
type Config struct {
	BaseURL string
	Token   string
}

// Client calls the MyPMS webhook booking engine endpoints (§1).
type Client struct {
	baseURL    string
	token      string
	httpClient *http.Client
}

// NewClient creates a client. BaseURL should not have a trailing slash.
func NewClient(cfg Config) *Client {
	base := strings.TrimRight(cfg.BaseURL, "/")
	return &Client{
		baseURL: base,
		token:   cfg.Token,
		httpClient: &http.Client{
			Timeout: defaultTimeout,
		},
	}
}

// OrgHealth calls GET /api/webhooks/bookings.
func (c *Client) OrgHealth(ctx context.Context) (*OrgHealthResponse, error) {
	var out OrgHealthResponse
	if err := c.do(ctx, http.MethodGet, "/api/webhooks/bookings", nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// SearchProperties calls POST /api/webhooks/bookings with action search_properties.
func (c *Client) SearchProperties(ctx context.Context, city, country, name string) (*SearchPropertiesResponse, error) {
	body := SearchPropertiesRequest{
		Action:  ActionSearchProperties,
		City:    city,
		Country: country,
		Name:    name,
	}
	var out SearchPropertiesResponse
	if err := c.do(ctx, http.MethodPost, "/api/webhooks/bookings", body, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// PropertyHealth calls GET /api/webhooks/bookings/{propertyId}.
func (c *Client) PropertyHealth(ctx context.Context, propertyID string) (*PropertyHealthResponse, error) {
	var out PropertyHealthResponse
	path := fmt.Sprintf("/api/webhooks/bookings/%s", propertyID)
	if err := c.do(ctx, http.MethodGet, path, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// SearchAvailability calls POST .../{propertyId} with action search_availability.
func (c *Client) SearchAvailability(ctx context.Context, propertyID string, req SearchAvailabilityRequest) (*SearchAvailabilityResponse, error) {
	req.Action = ActionSearchAvailability
	var out SearchAvailabilityResponse
	path := fmt.Sprintf("/api/webhooks/bookings/%s", propertyID)
	if err := c.do(ctx, http.MethodPost, path, req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetRoomDetails calls POST .../{propertyId} with action get_room_details.
func (c *Client) GetRoomDetails(ctx context.Context, propertyID, roomID, roomTypeID string) (*GetRoomDetailsResponse, error) {
	body := GetRoomDetailsRequest{
		Action:     ActionGetRoomDetails,
		RoomID:     roomID,
		RoomTypeID: roomTypeID,
	}
	var out GetRoomDetailsResponse
	path := fmt.Sprintf("/api/webhooks/bookings/%s", propertyID)
	if err := c.do(ctx, http.MethodPost, path, body, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetQuote calls POST .../{propertyId} with action get_quote.
func (c *Client) GetQuote(ctx context.Context, propertyID string, req GetQuoteRequest) (*Quote, error) {
	req.Action = ActionGetQuote
	var out Quote
	path := fmt.Sprintf("/api/webhooks/bookings/%s", propertyID)
	if err := c.do(ctx, http.MethodPost, path, req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// CreateBooking calls POST .../{propertyId} with action create_booking.
func (c *Client) CreateBooking(ctx context.Context, propertyID string, req CreateBookingRequest) (*Booking, error) {
	req.Action = ActionCreateBooking
	path := fmt.Sprintf("/api/webhooks/bookings/%s", propertyID)
	raw, err := c.doBytes(ctx, http.MethodPost, path, req)
	if err != nil {
		return nil, err
	}
	var wrapped CreateBookingResponse
	if err := json.Unmarshal(raw, &wrapped); err == nil && wrapped.Data.BookingID != "" {
		return &wrapped.Data, nil
	}
	var direct Booking
	if err := json.Unmarshal(raw, &direct); err != nil {
		return nil, fmt.Errorf("mypms: decode booking: %w", err)
	}
	return &direct, nil
}

// GetBooking calls POST .../{propertyId} with action get_booking.
func (c *Client) GetBooking(ctx context.Context, propertyID, bookingID string) (*Booking, error) {
	body := GetBookingRequest{
		Action:    ActionGetBooking,
		BookingID: bookingID,
	}
	var out Booking
	path := fmt.Sprintf("/api/webhooks/bookings/%s", propertyID)
	if err := c.do(ctx, http.MethodPost, path, body, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// UpdateBooking calls POST .../{propertyId} with action update_booking.
func (c *Client) UpdateBooking(ctx context.Context, propertyID string, req UpdateBookingRequest) (*Booking, error) {
	req.Action = ActionUpdateBooking
	var out Booking
	path := fmt.Sprintf("/api/webhooks/bookings/%s", propertyID)
	if err := c.do(ctx, http.MethodPost, path, req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// CancelBooking calls POST .../{propertyId} with action cancel_booking.
func (c *Client) CancelBooking(ctx context.Context, propertyID string, req CancelBookingRequest) (*CancelBookingResponse, error) {
	req.Action = ActionCancelBooking
	var out CancelBookingResponse
	path := fmt.Sprintf("/api/webhooks/bookings/%s", propertyID)
	if err := c.do(ctx, http.MethodPost, path, req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// DeleteBooking calls POST .../{propertyId} with action delete_booking.
func (c *Client) DeleteBooking(ctx context.Context, propertyID string, req DeleteBookingRequest) (*DeleteBookingResponse, error) {
	req.Action = ActionDeleteBooking
	var out DeleteBookingResponse
	path := fmt.Sprintf("/api/webhooks/bookings/%s", propertyID)
	if err := c.do(ctx, http.MethodPost, path, req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ListBookings calls POST .../{propertyId} with action list_bookings.
func (c *Client) ListBookings(ctx context.Context, propertyID string, req ListBookingsRequest) (*ListBookingsResponse, error) {
	req.Action = ActionListBookings
	var out ListBookingsResponse
	path := fmt.Sprintf("/api/webhooks/bookings/%s", propertyID)
	if err := c.do(ctx, http.MethodPost, path, req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) doBytes(ctx context.Context, method, path string, body any) ([]byte, error) {
	var r io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("mypms: marshal request: %w", err)
		}
		r = bytes.NewReader(b)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, r)
	if err != nil {
		return nil, fmt.Errorf("mypms: new request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("mypms: request failed: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("mypms: read body: %w", err)
	}

	fmt.Println("MYPMS RAW BODY:", string(raw))

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		msg := strings.TrimSpace(string(raw))
		if msg == "" {
			msg = resp.Status
		}
		return nil, &APIError{StatusCode: resp.StatusCode, Message: msg, Body: string(raw)}
	}
	return raw, nil
}

func (c *Client) do(ctx context.Context, method, path string, body any, out any) error {
	raw, err := c.doBytes(ctx, method, path, body)
	if err != nil {
		return err
	}
	if out == nil || len(raw) == 0 {
		return nil
	}
	if err := json.Unmarshal(raw, out); err != nil {
		return fmt.Errorf("mypms: decode response: %w", err)
	}
	return nil
}
