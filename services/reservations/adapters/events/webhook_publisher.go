package events

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
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
		// ota_reference stays the channel's readable name, which is what
		// the PMS files the booking under. The connector id travels on its
		// own field so the PMS can match the exact connector.
		"ota_reference":         otaReference(res),
		"channel_provider":      res.ChannelProvider,
		"channel_connection_id": res.ChannelID,
		"created_at":            time.Now().Format(time.RFC3339),
		"updated_at":            time.Now().Format(time.RFC3339),
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
		return fmt.Errorf("%w: %v", domain.ErrPMSUnavailable, err)
	}
	defer resp.Body.Close()

	if retryableStatus(resp.StatusCode) {
		slog.Error("webhook to PMS not taken; will resend", "status", resp.StatusCode, "event", eventType)
		return fmt.Errorf("%w: status %d", domain.ErrPMSUnavailable, resp.StatusCode)
	}
	if resp.StatusCode >= 400 {
		// Taken but refused, e.g. 422: the PMS recorded the booking as one
		// it couldn't import and imports it itself once that is fixed.
		slog.Warn("webhook to PMS refused", "status", resp.StatusCode, "event", eventType)
	}
	return nil
}

// retryableStatus: the PMS didn't take the booking and may later — it was
// down or overloaded (5xx, 408, 429), or not reachable as configured (401,
// 403, 404 while a deploy or a secret is wrong). Any other 4xx is a decision
// about the booking itself and is not resent.
func retryableStatus(code int) bool {
	switch {
	case code >= 500:
		return true
	case code == 401, code == 403, code == 404, code == 408, code == 429:
		return true
	}
	return false
}

// otaReference is the channel's display name, falling back to the provider id
// and, for reservations that carry neither, the connector id (the old value).
func otaReference(res *domain.Reservation) string {
	switch {
	case res.ChannelName != "":
		return res.ChannelName
	case res.ChannelProvider != "":
		return res.ChannelProvider
	default:
		return res.ChannelID
	}
}
