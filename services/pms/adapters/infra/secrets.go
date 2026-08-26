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

// Resolve returns the credentials for a connection.
//
// Precedence is deliberate and was inverted on 2026-08-15: **stored credentials
// win over the environment**, and the environment supplies the default only when
// nothing is stored.
//
// The previous order applied PMS_BASE_URL / PMS_BEARER_TOKEN on top of stored
// credentials, which is correct for one tenant and wrong for more than one: the
// override is process-wide, so every organization's connection resolved to the
// same PMS and the same bearer token no matter what was registered for it. Each
// bundled tenant registers its own callback credential (see the PMS's
// register-pms module), and an env var that quietly replaces all of them defeats
// the isolation those credentials exist to provide.
//
// The environment still does the job it was added for — pointing a deployment at
// its PMS without a stored secret — it just no longer outranks per-tenant state.
func (r *InMemorySecretResolver) Resolve(_ context.Context, ref string) (map[string]string, error) {
	r.mu.RLock()
	creds, ok := r.store[ref]
	r.mu.RUnlock()

	if ok {
		out := make(map[string]string, len(creds))
		for k, v := range creds {
			out[k] = v
		}
		return out, nil
	}

	// Nothing stored for this ref. Fall back to the deployment's configured PMS.
	return pmsCredentialsFromEnv(ref)
}

// pmsCredentialsFromEnv builds credentials from PMS_BASE_URL / PMS_BEARER_TOKEN.
//
// Returns an error when they are unset, rather than the hardcoded
// "http://localhost:4001" + "dev-pms-integration-token" this used to return. That
// constant made a lost or unpopulated store look like a working connection: every
// call went to localhost, which in a container is the container itself, and the
// failure surfaced as "the PMS is not responding" rather than "these credentials
// were never loaded". An error names the actual problem.
func pmsCredentialsFromEnv(ref string) (map[string]string, error) {
	baseURL := os.Getenv("PMS_BASE_URL")
	token := os.Getenv("PMS_BEARER_TOKEN")

	if baseURL == "" {
		return nil, fmt.Errorf(
			"pms/secrets: no credentials stored for %q and PMS_BASE_URL is not set", ref)
	}
	if token == "" {
		return nil, fmt.Errorf(
			"pms/secrets: no credentials stored for %q and PMS_BEARER_TOKEN is not set", ref)
	}

	return map[string]string{
		"base_url":     baseURL,
		"bearer_token": token,
		"token":        token,
	}, nil
}
