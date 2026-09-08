package events

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
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
	// HMAC-SHA256 key for x-channel-manager-signature. Empty = unsigned
	// (development only; the PMS enforces the signature in production).
	secret string
}

// NewWebhookPublisher creates a new publisher.
func NewWebhookPublisher(baseURL string) *WebhookPublisher {
	return &WebhookPublisher{
		client:  &http.Client{Timeout: 10 * time.Second},
		baseURL: baseURL,
	}
}

// WithSecret signs every webhook body with HMAC-SHA256(secret), hex-encoded,
// in the x-channel-manager-signature header — the format the PMS verifies.
func (p *WebhookPublisher) WithSecret(secret string) *WebhookPublisher {
	p.secret = secret
	return p
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
	// No room id, ever: the channel manager is room-type-level. The PMS
	// creates the stay UNASSIGNED against the mapped type and assigns the
	// physical room itself (see pms-app/docs/ROOM_ASSIGNMENT_PLAN.md).
	data := map[string]any{
		"id":                   res.ChannelConfirmationID,
		"status":               res.Status,
		"check_in":             res.CheckIn.Format("2006-01-02"),
		"check_out":            res.CheckOut.Format("2006-01-02"),
		"guest_name":           res.GuestName,
		"room_type_id":         res.RoomTypeID,
		"channel_room_type_id": res.ChannelRoomTypeID,
		"rate_plan_id":         res.RatePlanID,
		"total_price":          res.TotalAmount,
		"currency":             res.Currency,
		"ota_reference":        res.ChannelID,
		"created_at":           time.Now().Format(time.RFC3339),
		"updated_at":           time.Now().Format(time.RFC3339),
	}
	// Party size only when the channel reported it. `number_of_guests` is
	// kept for the PMS's existing decoder; adults/children are the real fields.
	if res.Adults > 0 {
		data["adults"] = res.Adults
		data["children"] = res.Children
		data["number_of_guests"] = res.Adults + res.Children
	}
	payload := map[string]any{
		"event":       eventType,
		"property_id": propID,
		"data":        data,
	}

	body, _ := json.Marshal(payload)
	req, _ := http.NewRequestWithContext(ctx, "POST", p.baseURL, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if p.secret != "" {
		mac := hmac.New(sha256.New, []byte(p.secret))
		mac.Write(body)
		req.Header.Set("x-channel-manager-signature", hex.EncodeToString(mac.Sum(nil)))
	}

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
