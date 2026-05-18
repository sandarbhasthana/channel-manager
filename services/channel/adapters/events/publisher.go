// Package events implements ports.ChannelEventPublisher using the platform EventBus.
package events

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"

	platformevents "github.com/channel-manager/channel-manager/platform/events"
	"github.com/channel-manager/channel-manager/services/channel/domain"
)

const (
	eventTypeSyncSucceeded = "channel.sync.succeeded"
	eventTypeSyncFailed    = "channel.sync.failed"
)

// Publisher implements ports.ChannelEventPublisher.
type Publisher struct {
	bus platformevents.EventBus
}

// NewPublisher creates a new channel event publisher.
func NewPublisher(bus platformevents.EventBus) *Publisher {
	return &Publisher{bus: bus}
}

func (p *Publisher) PublishSyncSucceeded(ctx context.Context, job domain.SyncJob) error {
	return p.publish(ctx, eventTypeSyncSucceeded, job)
}

func (p *Publisher) PublishSyncFailed(ctx context.Context, job domain.SyncJob) error {
	return p.publish(ctx, eventTypeSyncFailed, job)
}

func (p *Publisher) publish(ctx context.Context, eventType string, job domain.SyncJob) error {
	payload, err := json.Marshal(job)
	if err != nil {
		return fmt.Errorf("channel/events: marshal: %w", err)
	}
	return p.bus.Publish(ctx, platformevents.Event{
		ID:          uuid.NewString(),
		Type:        eventType,
		AggregateID: job.ConnectionID,
		OrgID:       job.OrgID,
		Payload:     payload,
		Timestamp:   time.Now().UTC(),
	})
}
