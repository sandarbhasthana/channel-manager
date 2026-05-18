// Package infra provides stub implementations for infrastructure ports.
package infra

import (
	"context"
	"fmt"
	"sync"

	"github.com/google/uuid"
)

// InMemorySecretResolver is a dev/test implementation of ports.SecretResolver
// that stores credentials in memory. Replace with a real Vault / AWS Secrets
// Manager integration before production.
type InMemorySecretResolver struct {
	mu    sync.RWMutex
	store map[string]map[string]string
}

// NewInMemorySecretResolver creates a new in-memory secret resolver.
func NewInMemorySecretResolver() *InMemorySecretResolver {
	return &InMemorySecretResolver{
		store: make(map[string]map[string]string),
	}
}

// Store persists credentials in memory and returns a generated secret_ref.
// If ref is empty a new ref is generated; otherwise the existing ref is
// overwritten (credential rotation).
func (r *InMemorySecretResolver) Store(_ context.Context, ref string, creds map[string]string) (string, error) {
	if len(creds) == 0 {
		return ref, nil // nothing to store
	}
	r.mu.Lock()
	defer r.mu.Unlock()

	if ref == "" {
		ref = fmt.Sprintf("inmem://secrets/%s", uuid.NewString())
	}
	// deep copy so the caller can't mutate the stored map
	stored := make(map[string]string, len(creds))
	for k, v := range creds {
		stored[k] = v
	}
	r.store[ref] = stored
	return ref, nil
}

// Resolve retrieves credentials by secret_ref.
func (r *InMemorySecretResolver) Resolve(_ context.Context, ref string) (map[string]string, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	creds, ok := r.store[ref]
	if !ok {
		return map[string]string{}, nil
	}
	// return a copy
	out := make(map[string]string, len(creds))
	for k, v := range creds {
		out[k] = v
	}
	return out, nil
}
