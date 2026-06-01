package infra

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
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
	r := &InMemorySecretResolver{store: make(map[string]map[string]string)}
	// Load from disk if exists
	if data, err := os.ReadFile("secrets.json"); err == nil {
		_ = json.Unmarshal(data, &r.store)
	}
	return r
}

func (r *InMemorySecretResolver) save() {
	if data, err := json.MarshalIndent(r.store, "", "  "); err == nil {
		_ = os.WriteFile("secrets.json", data, 0644)
	}
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
	r.save()
	return ref, nil
}

func (r *InMemorySecretResolver) Resolve(_ context.Context, ref string) (map[string]string, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	creds, ok := r.store[ref]
	if !ok {
		// Fallback for local development so it survives restarts without losing credentials
		return map[string]string{
			"base_url": "http://localhost:4001",
			"bearer_token": "dev-pms-integration-token",
			"token": "dev-pms-integration-token",
		}, nil
	}
	out := make(map[string]string, len(creds))
	for k, v := range creds {
		out[k] = v
	}
	return out, nil
}
