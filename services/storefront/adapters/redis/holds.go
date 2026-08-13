// Package redis implements the storefront hold and idempotency stores.
package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/channel-manager/channel-manager/services/storefront/domain"
	"github.com/channel-manager/channel-manager/services/storefront/ports"
)

const (
	holdKeyPrefix = "storefront:hold:"
	holdIndexKey  = "storefront:holds:"
	idemKeyPrefix = "storefront:idempotency:"
	idemKeyTTL    = 24 * time.Hour
)

// HoldStore implements ports.HoldStore using Redis string keys with a TTL,
// plus a per-property set used as an index.
//
// The TTL on the hold key is the source of truth for expiry; the index set is
// pruned lazily in ActiveForProperty when a member's key has vanished. This
// keeps Place on the hot path down to two commands and avoids a sweeper.
type HoldStore struct {
	client *redis.Client
}

// NewHoldStore creates a hold store backed by the given Redis client.
func NewHoldStore(client *redis.Client) *HoldStore {
	return &HoldStore{client: client}
}

func holdKey(token string) string       { return holdKeyPrefix + token }
func indexKey(propertyID string) string { return holdIndexKey + propertyID }

// Place stores h with a TTL derived from h.ExpiresAt.
func (s *HoldStore) Place(ctx context.Context, h domain.Hold) error {
	ttl := time.Until(h.ExpiresAt)
	if ttl <= 0 {
		return fmt.Errorf("storefront: hold already expired")
	}
	payload, err := json.Marshal(h)
	if err != nil {
		return fmt.Errorf("storefront: marshal hold: %w", err)
	}
	pipe := s.client.TxPipeline()
	pipe.Set(ctx, holdKey(h.Token), payload, ttl)
	pipe.SAdd(ctx, indexKey(h.PropertyID), h.Token)
	// Keep the index from outliving its members indefinitely.
	pipe.Expire(ctx, indexKey(h.PropertyID), ttl+time.Hour)
	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("storefront: place hold: %w", err)
	}
	return nil
}

// Get returns the hold for token, or domain.ErrHoldNotFound.
func (s *HoldStore) Get(ctx context.Context, token string) (domain.Hold, error) {
	raw, err := s.client.Get(ctx, holdKey(token)).Bytes()
	if err == redis.Nil {
		return domain.Hold{}, domain.ErrHoldNotFound
	}
	if err != nil {
		return domain.Hold{}, fmt.Errorf("storefront: get hold: %w", err)
	}
	var h domain.Hold
	if err := json.Unmarshal(raw, &h); err != nil {
		return domain.Hold{}, fmt.Errorf("storefront: unmarshal hold: %w", err)
	}
	return h, nil
}

// Release removes the hold for token. Unknown tokens are a no-op.
func (s *HoldStore) Release(ctx context.Context, token string) error {
	h, err := s.Get(ctx, token)
	if err == domain.ErrHoldNotFound {
		return nil
	}
	if err != nil {
		return err
	}
	pipe := s.client.TxPipeline()
	pipe.Del(ctx, holdKey(token))
	pipe.SRem(ctx, indexKey(h.PropertyID), token)
	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("storefront: release hold: %w", err)
	}
	return nil
}

// ActiveForProperty returns unexpired holds for propertyID, pruning index
// entries whose hold key has already expired.
func (s *HoldStore) ActiveForProperty(ctx context.Context, propertyID string) ([]domain.Hold, error) {
	tokens, err := s.client.SMembers(ctx, indexKey(propertyID)).Result()
	if err != nil {
		return nil, fmt.Errorf("storefront: list holds: %w", err)
	}
	holds := make([]domain.Hold, 0, len(tokens))
	stale := make([]string, 0)
	for _, tok := range tokens {
		h, err := s.Get(ctx, tok)
		if err == domain.ErrHoldNotFound {
			stale = append(stale, tok)
			continue
		}
		if err != nil {
			return nil, err
		}
		holds = append(holds, h)
	}
	if len(stale) > 0 {
		// Best-effort prune; a failure here only costs a wasted lookup later.
		_ = s.client.SRem(ctx, indexKey(propertyID), stale).Err()
	}
	return holds, nil
}

// IdempotencyStore implements ports.IdempotencyStore using Redis.
type IdempotencyStore struct {
	client *redis.Client
}

// NewIdempotencyStore creates an idempotency store backed by the given client.
func NewIdempotencyStore(client *redis.Client) *IdempotencyStore {
	return &IdempotencyStore{client: client}
}

// Get returns the cached result for a completed booking request.
func (s *IdempotencyStore) Get(ctx context.Context, key string) (ports.IdempotencyRecord, bool, error) {
	raw, err := s.client.Get(ctx, idemKeyPrefix+key).Bytes()
	if err == redis.Nil {
		return ports.IdempotencyRecord{}, false, nil
	}
	if err != nil {
		return ports.IdempotencyRecord{}, false, fmt.Errorf("storefront: idempotency get: %w", err)
	}
	// Older deployments stored a marker only. Keep treating it as a duplicate,
	// but do not pretend that a replayable response exists.
	if string(raw) == "1" {
		return ports.IdempotencyRecord{}, true, nil
	}
	var record ports.IdempotencyRecord
	if err := json.Unmarshal(raw, &record); err != nil {
		return ports.IdempotencyRecord{}, false, fmt.Errorf("storefront: decode idempotency record: %w", err)
	}
	return record, true, nil
}

// Put stores a replayable booking response with a 24-hour TTL.
func (s *IdempotencyStore) Put(ctx context.Context, key string, record ports.IdempotencyRecord) error {
	raw, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("storefront: encode idempotency record: %w", err)
	}
	if err := s.client.Set(ctx, idemKeyPrefix+key, raw, idemKeyTTL).Err(); err != nil {
		return fmt.Errorf("storefront: idempotency put: %w", err)
	}
	return nil
}
