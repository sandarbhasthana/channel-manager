package infra

import (
	"context"
	"strings"
	"testing"
)

// Stored credentials outrank the environment.
//
// This assertion is the inverse of what it was before 2026-08-15, when
// PMS_BASE_URL / PMS_BEARER_TOKEN were applied on top of whatever was stored.
// That precedence works for a single tenant and breaks silently for more than
// one: the override is process-wide, so every organization's connection resolved
// to the same PMS regardless of the credentials registered for it. Each bundled
// tenant registers its own callback credential, and those must not be replaceable
// by one shared env var.
func TestResolvePrefersStoredCredentialsOverEnvironment(t *testing.T) {
	t.Setenv("PMS_BASE_URL", "https://shared.example")
	t.Setenv("PMS_BEARER_TOKEN", "shared-token")

	resolver := &InMemorySecretResolver{
		store: map[string]map[string]string{
			"ref": {
				"base_url":     "https://tenant.example",
				"bearer_token": "tenant-token",
				"token":        "tenant-token",
			},
		},
	}

	credentials, err := resolver.Resolve(context.Background(), "ref")
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	for key, want := range map[string]string{
		"base_url":     "https://tenant.example",
		"bearer_token": "tenant-token",
		"token":        "tenant-token",
	} {
		if got := credentials[key]; got != want {
			t.Errorf("%s = %q, want the stored value %q", key, got, want)
		}
	}
}

func TestResolveKeepsStoredCredentialsWithoutOverrides(t *testing.T) {
	t.Setenv("PMS_BASE_URL", "")
	t.Setenv("PMS_BEARER_TOKEN", "")

	resolver := &InMemorySecretResolver{
		store: map[string]map[string]string{
			"ref": {
				"base_url":     "https://stored.example",
				"bearer_token": "stored-token",
			},
		},
	}

	credentials, err := resolver.Resolve(context.Background(), "ref")
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if got := credentials["base_url"]; got != "https://stored.example" {
		t.Fatalf("base_url = %q, want stored URL", got)
	}
	if got := credentials["bearer_token"]; got != "stored-token" {
		t.Fatalf("bearer_token = %q, want stored token", got)
	}
}

// With nothing stored, the environment supplies the deployment's PMS — the case
// the override was originally added for, still working.
func TestResolveFallsBackToEnvironmentWhenNothingStored(t *testing.T) {
	t.Setenv("PMS_BASE_URL", "https://pms.example")
	t.Setenv("PMS_BEARER_TOKEN", "deployment-token")

	resolver := &InMemorySecretResolver{store: map[string]map[string]string{}}

	credentials, err := resolver.Resolve(context.Background(), "missing-ref")
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if got := credentials["base_url"]; got != "https://pms.example" {
		t.Fatalf("base_url = %q, want the configured URL", got)
	}
	if got := credentials["token"]; got != "deployment-token" {
		t.Fatalf("token = %q, want the configured token", got)
	}
}

// An unknown ref with no configuration is an error, not a guess.
//
// This used to return a hardcoded localhost URL and a dev token, so a store that
// had never loaded looked exactly like a healthy connection — and the resulting
// failure read as "the PMS is down" rather than "these credentials do not exist".
func TestResolveErrorsWhenNothingStoredAndNothingConfigured(t *testing.T) {
	t.Setenv("PMS_BASE_URL", "")
	t.Setenv("PMS_BEARER_TOKEN", "")

	resolver := &InMemorySecretResolver{store: map[string]map[string]string{}}

	if _, err := resolver.Resolve(context.Background(), "missing-ref"); err == nil {
		t.Fatal("Resolve() returned no error for an unknown ref with no configuration")
	} else if !strings.Contains(err.Error(), "PMS_BASE_URL") {
		t.Fatalf("error = %q, want it to name the missing configuration", err)
	}
}
