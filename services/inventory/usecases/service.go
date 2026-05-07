package usecases

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/channel-manager/channel-manager/services/inventory/domain"
	"github.com/channel-manager/channel-manager/services/inventory/ports"
)

// ErrDuplicateRequest is returned when an idempotency key has already been processed.
var ErrDuplicateRequest = errors.New("inventory: duplicate request")

// GetInventoryInput carries parameters for a GetInventory call.
type GetInventoryInput struct {
	RoomTypeID string
	From       time.Time
	To         time.Time
}

// BulkUpsertInput carries parameters for a BulkUpsertInventory call.
type BulkUpsertInput struct {
	Days           []domain.InventoryDay
	IdempotencyKey string
}

// BulkUpsertResult is the result of a successful BulkUpsertInventory call.
type BulkUpsertResult struct {
	RowsAffected int32
	EventID      string
}

// InventoryService orchestrates inventory operations.
type InventoryService struct {
	repo      ports.InventoryRepository
	idem      ports.IdempotencyStore
	publisher ports.InventoryEventPublisher
}

// NewInventoryService creates a new InventoryService.
func NewInventoryService(
	repo ports.InventoryRepository,
	idem ports.IdempotencyStore,
	publisher ports.InventoryEventPublisher,
) *InventoryService {
	return &InventoryService{repo: repo, idem: idem, publisher: publisher}
}

// GetInventory fetches inventory days for a room type within the given date range.
func (s *InventoryService) GetInventory(ctx context.Context, in GetInventoryInput) ([]domain.InventoryDay, error) {
	days, err := s.repo.ListByRange(ctx, in.RoomTypeID, in.From, in.To)
	if err != nil {
		return nil, fmt.Errorf("inventory: get: %w", err)
	}
	return days, nil
}

// BulkUpsertInventory atomically upserts many inventory days.
// Returns ErrDuplicateRequest when the idempotency key was already processed.
func (s *InventoryService) BulkUpsertInventory(ctx context.Context, in BulkUpsertInput) (BulkUpsertResult, error) {
	// 1. Idempotency guard.
	if in.IdempotencyKey != "" {
		exists, err := s.idem.Exists(ctx, in.IdempotencyKey)
		if err != nil {
			return BulkUpsertResult{}, fmt.Errorf("inventory: idempotency check: %w", err)
		}
		if exists {
			return BulkUpsertResult{}, ErrDuplicateRequest
		}
	}

	// 2. Persist.
	if err := s.repo.UpsertBatch(ctx, in.Days); err != nil {
		return BulkUpsertResult{}, fmt.Errorf("inventory: upsert batch: %w", err)
	}

	// 3. Publish domain events (best-effort — upsert is already committed).
	eventID := uuid.NewString()
	if err := s.publisher.PublishInventoryUpdated(ctx, in.Days); err != nil {
		// Log-worthy but non-fatal: downstream sync will catch up via polling.
		_ = err
	}

	// 4. Mark idempotency key as processed.
	if in.IdempotencyKey != "" {
		if err := s.idem.Mark(ctx, in.IdempotencyKey); err != nil {
			// Non-fatal: at-most-once is best-effort; we already committed.
			_ = err
		}
	}

	return BulkUpsertResult{
		RowsAffected: int32(len(in.Days)), //nolint:gosec
		EventID:      eventID,
	}, nil
}
