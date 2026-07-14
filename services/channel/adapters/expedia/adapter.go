package expedia

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/channel-manager/channel-manager/services/channel/domain"
)

// Adapter implements the channel adapter for Expedia.
type Adapter struct {
	client *http.Client
}

// NewAdapter creates a new Expedia adapter.
func NewAdapter() *Adapter {
	return &Adapter{client: &http.Client{Timeout: 10 * time.Second}}
}

func (a *Adapter) ChannelID() string {
	return "expedia"
}

func (a *Adapter) Capabilities() []domain.ChannelCapability {
	return []domain.ChannelCapability{
		domain.CapabilityPushAvailability,
		domain.CapabilityPushRates,
		domain.CapabilityFetchReservations,
	}
}

func (a *Adapter) PushAvailability(ctx context.Context, updates []domain.AvailabilityUpdate) error {
	body, _ := json.Marshal(map[string]any{"updates": updates})
	req, _ := http.NewRequestWithContext(ctx, "POST", "http://localhost:3001/api/push-availability", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-ota-provider", "expedia")
	
	resp, err := a.client.Do(req)
	if err != nil {
		return fmt.Errorf("expedia mock ota error: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("expedia mock ota returned status %d", resp.StatusCode)
	}
	return nil
}

func (a *Adapter) PushRates(ctx context.Context, updates []domain.RateUpdate) error {
	body, _ := json.Marshal(map[string]any{"updates": updates})
	req, _ := http.NewRequestWithContext(ctx, "POST", "http://localhost:3001/api/push-rates", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-ota-provider", "expedia")
	
	resp, err := a.client.Do(req)
	if err != nil {
		return fmt.Errorf("expedia mock ota error: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("expedia mock ota returned status %d", resp.StatusCode)
	}
	return nil
}

func (a *Adapter) FetchReservations(ctx context.Context, propertyID string, since time.Time) ([]domain.FetchedReservation, error) {
	req, _ := http.NewRequestWithContext(ctx, "GET", "http://localhost:3001/api/reservations?provider=expedia", nil)
	resp, err := a.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("expedia mock ota fetch error: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("expedia mock ota returned status %d", resp.StatusCode)
	}

	var payload struct {
		Reservations []struct {
			ID          string  `json:"id"`
			Status      string  `json:"status"`
			CheckIn     string  `json:"check_in"`
			CheckOut    string  `json:"check_out"`
			GuestName   string  `json:"guest_name"`
			RoomTypeID  string  `json:"room_type_id"`
			TotalPrice  float64 `json:"total_price"`
			Currency    string  `json:"currency"`
		} `json:"reservations"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("expedia decode error: %w", err)
	}

	var res []domain.FetchedReservation
	for _, r := range payload.Reservations {
		// Only return reservations that would conceptually be "new" or "modified" since the `since` time.
		// For the mock, we'll just return all of them and let the SyncService deduplicate them by ID.
		checkIn, _ := time.Parse("2006-01-02", r.CheckIn)
		checkOut, _ := time.Parse("2006-01-02", r.CheckOut)
		
		status := "confirmed"
		if r.Status == "cancelled" {
			status = "cancelled"
		}
		
		res = append(res, domain.FetchedReservation{
			ChannelConfirmationID: r.ID,
			Status:                status,
			CheckIn:               checkIn,
			CheckOut:              checkOut,
			GuestName:             r.GuestName,
			RoomTypeExternalID:    r.RoomTypeID,
			TotalAmount:           r.TotalPrice,
			Currency:              r.Currency,
		})
	}
	return res, nil
}

