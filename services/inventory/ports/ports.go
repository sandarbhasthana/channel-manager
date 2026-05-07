package ports

import (
	"context"
	"time"

	"github.com/channel-manager/channel-manager/services/inventory/domain"
)

// InventoryRepository provides tenant-scoped persistence for inventory data.
// Implementations must extract the org_id from ctx (via auth.TenantContext) and
// use it to satisfy both the RLS WITH CHECK policy and the composite PK.
type InventoryRepository interface {
	// ListByRange returns all inventory days for a room type within [from, to] inclusive.
	ListByRange(ctx context.Context, roomTypeID string, from, to time.Time) ([]domain.InventoryDay, error)

	// UpsertBatch writes a batch of inventory days using INSERT … ON CONFLICT DO UPDATE.
	// All days must belong to the org resolved from ctx.
	UpsertBatch(ctx context.Context, days []domain.InventoryDay) error
}

// IdempotencyStore tracks processed request keys to prevent duplicate writes.
type IdempotencyStore interface {
	// Exists reports whether the given key has already been processed.
	Exists(ctx context.Context, key string) (bool, error)

	// Mark records the key as processed. Implementations should set a TTL
	// (recommended: 24 h) so the store self-cleans over time.
	Mark(ctx context.Context, key string) error
}

// InventoryEventPublisher publishes domain events for inventory changes.
type InventoryEventPublisher interface {
	// PublishInventoryUpdated emits an event for every day that was upserted.
	PublishInventoryUpdated(ctx context.Context, days []domain.InventoryDay) error
}
