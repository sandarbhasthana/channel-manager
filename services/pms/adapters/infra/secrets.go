package infra

import (
	"context"
	"fmt"
	"sync"

	"github.com/google/uuid"
)

// InMemorySecretResolver stores PMS credentials in memory for local development.
type InMemorySecretResolver struct {
	mu    sync.RWMutex
	store map[string]map[string]string
}

// NewInMemorySecretResolver creates a new in-memory secret resolver.
func NewInMemorySecretResolver() *InMemorySecretResolver {
	return &InMemorySecretResolver{store: make(map[string]map[string]string)}
}

func (r *InMemorySecretResolver) Store(_ context.Context, ref string, creds map[string]string) (string, error) {
	if len(creds) == 0 {
		return ref, nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if ref == "" {
		ref = fmt.Sprintf("inmem://pms-secrets/%s", uuid.NewString())
	}
	stored := make(map[string]string, len(creds))
	for k, v := range creds {
		stored[k] = v
	}
	r.store[ref] = stored
	return ref, nil
}

func (r *InMemorySecretResolver) Resolve(_ context.Context, ref string) (map[string]string, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	creds, ok := r.store[ref]
	if !ok {
		return map[string]string{}, nil
	}
	out := make(map[string]string, len(creds))
	for k, v := range creds {
		out[k] = v
	}
	return out, nil
}
