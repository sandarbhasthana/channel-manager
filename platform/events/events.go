package events

import (
	"context"
	"encoding/json"
	"time"
)

// Event represents a domain event in the system.
type Event struct {
	ID          string          `json:"id"`
	Type        string          `json:"type"`
	AggregateID string          `json:"aggregate_id"`
	OrgID       string          `json:"org_id"`
	Payload     json.RawMessage `json:"payload"`
	Timestamp   time.Time       `json:"timestamp"`
}

// EventHandler is a function that processes an event.
type EventHandler func(ctx context.Context, event Event) error

// EventBus defines the interface for publishing and subscribing to events.
type EventBus interface {
	Publish(ctx context.Context, event Event) error
	Subscribe(eventType string, handler EventHandler) error
}
