// Package events implements ports.InventoryEventPublisher using the platform EventBus.
package events

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"

	platformevents "github.com/channel-manager/channel-manager/platform/events"
	"github.com/channel-manager/channel-manager/services/inventory/domain"
)

const eventTypeInventoryUpdated = "inventory.updated"

// Publisher implements ports.InventoryEventPublisher.
type Publisher struct {
	bus platformevents.EventBus
}

// NewPublisher creates a new inventory event publisher.
func NewPublisher(bus platformevents.EventBus) *Publisher {
	return &Publisher{bus: bus}
}

// inventoryUpdatedPayload is the JSON payload for inventory.updated events.
type inventoryUpdatedPayload struct {
	OrgID      string    `json:"org_id"`
	RoomTypeID string    `json:"room_type_id"`
	StayDate   string    `json:"stay_date"`
	Available  int       `json:"available"`
	Sold       int       `json:"sold"`
	Blocked    int       `json:"blocked"`
	StopSell   bool      `json:"stop_sell"`
	CTA        bool      `json:"cta"`
	CTD        bool      `json:"ctd"`
	Version    int64     `json:"version"`
}

// PublishInventoryUpdated emits one event per updated inventory day.
func (p *Publisher) PublishInventoryUpdated(ctx context.Context, days []domain.InventoryDay) error {
	for _, day := range days {
		payload := inventoryUpdatedPayload{
			OrgID:      day.OrgID,
			RoomTypeID: day.RoomTypeID,
			StayDate:   day.StayDate.Format("2006-01-02"),
			Available:  day.Available,
			Sold:       day.Sold,
			Blocked:    day.Blocked,
			StopSell:   day.StopSell,
			CTA:        day.CTA,
			CTD:        day.CTD,
			Version:    day.Version,
		}
		raw, err := json.Marshal(payload)
		if err != nil {
			return fmt.Errorf("inventory/events: marshal payload: %w", err)
		}
		event := platformevents.Event{
			ID:          uuid.NewString(),
			Type:        eventTypeInventoryUpdated,
			AggregateID: day.RoomTypeID,
			OrgID:       day.OrgID,
			Payload:     raw,
			Timestamp:   time.Now().UTC(),
		}
		if err := p.bus.Publish(ctx, event); err != nil {
			return fmt.Errorf("inventory/events: publish day %s: %w", payload.StayDate, err)
		}
	}
	return nil
}
