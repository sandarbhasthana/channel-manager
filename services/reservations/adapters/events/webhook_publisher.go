package events

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/channel-manager/channel-manager/services/reservations/domain"
)

// WebhookPublisher sends reservation events to the PMS webhook endpoint.
type WebhookPublisher struct {
	client  *http.Client
	baseURL string
}

// NewWebhookPublisher creates a new publisher.
func NewWebhookPublisher(baseURL string) *WebhookPublisher {
	return &WebhookPublisher{
		client:  &http.Client{Timeout: 10 * time.Second},
		baseURL: baseURL,
	}
}

func (p *WebhookPublisher) PublishReservationCreated(ctx context.Context, res *domain.Reservation) error {
	return p.sendWebhook(ctx, "booking_created", res)
}

func (p *WebhookPublisher) PublishReservationUpdated(ctx context.Context, res *domain.Reservation) error {
	return p.sendWebhook(ctx, "booking_modified", res)
}

func (p *WebhookPublisher) sendWebhook(ctx context.Context, eventType string, res *domain.Reservation) error {
	// Construct the payload expected by PMS
	// We map res to ChannelManagerReservation
	propID := res.ExternalPropertyID
	if propID == "" {
		propID = res.PropertyID
	}
	payload := map[string]any{
		"event":       eventType,
		"property_id": propID,
		"data": map[string]any{
			"id":               res.ChannelConfirmationID,
			"status":           res.Status,
			"check_in":         res.CheckIn.Format("2006-01-02"),
			"check_out":        res.CheckOut.Format("2006-01-02"),
			"guest_name":       res.GuestName,
			"room_type_id":     res.RoomTypeID,
			"total_price":      res.TotalAmount,
			"currency":         res.Currency,
			"ota_reference":    res.ChannelID,
			"number_of_guests": 2, // Default
			"created_at":       time.Now().Format(time.RFC3339),
			"updated_at":       time.Now().Format(time.RFC3339),
		},
	}

	body, _ := json.Marshal(payload)
	req, _ := http.NewRequestWithContext(ctx, "POST", p.baseURL, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	// For MVP, we don't sign the webhook, but in production we'd add x-channel-manager-signature

	resp, err := p.client.Do(req)
	if err != nil {
		slog.Error("failed to send webhook to PMS", "error", err, "event", eventType)
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		slog.Error("webhook to PMS returned error", "status", resp.StatusCode)
	}

	return nil
}
