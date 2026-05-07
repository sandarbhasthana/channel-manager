// Package redis implements ports.IdempotencyStore backed by Redis.
package redis

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	keyPrefix = "inventory:idempotency:"
	keyTTL    = 24 * time.Hour
)

// IdempotencyStore implements ports.IdempotencyStore using Redis SETNX.
type IdempotencyStore struct {
	client *redis.Client
}

// NewIdempotencyStore creates an idempotency store backed by the given Redis client.
func NewIdempotencyStore(client *redis.Client) *IdempotencyStore {
	return &IdempotencyStore{client: client}
}

// Exists reports whether the key has already been processed.
func (s *IdempotencyStore) Exists(ctx context.Context, key string) (bool, error) {
	n, err := s.client.Exists(ctx, keyPrefix+key).Result()
	if err != nil {
		return false, fmt.Errorf("idempotency: exists check: %w", err)
	}
	return n > 0, nil
}

// Mark records the key as processed with a 24-hour TTL.
func (s *IdempotencyStore) Mark(ctx context.Context, key string) error {
	if err := s.client.Set(ctx, keyPrefix+key, "1", keyTTL).Err(); err != nil {
		return fmt.Errorf("idempotency: mark: %w", err)
	}
	return nil
}
