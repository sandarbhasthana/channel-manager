package inproc

import (
	"context"
	"sync"

	"github.com/channel-manager/channel-manager/platform/events"
)

// Bus is an in-process event bus implementation using Go channels.
type Bus struct {
	mu       sync.RWMutex
	handlers map[string][]events.EventHandler
}

// New creates a new in-process event bus.
func New() *Bus {
	return &Bus{
		handlers: make(map[string][]events.EventHandler),
	}
}

// Publish sends an event to all registered handlers for that event type.
func (b *Bus) Publish(ctx context.Context, event events.Event) error {
	b.mu.RLock()
	handlers, ok := b.handlers[event.Type]
	b.mu.RUnlock()

	if !ok {
		return nil
	}

	for _, handler := range handlers {
		if err := handler(ctx, event); err != nil {
			return err
		}
	}

	return nil
}

// Subscribe registers a handler for a given event type.
func (b *Bus) Subscribe(eventType string, handler events.EventHandler) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.handlers[eventType] = append(b.handlers[eventType], handler)
	return nil
}
