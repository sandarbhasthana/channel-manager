package infra

import (
	"context"
	"testing"
)

func TestInMemorySecretResolverResolveAppliesProductionOverrides(t *testing.T) {
	t.Setenv("PMS_BASE_URL", "https://pms.example")
	t.Setenv("PMS_BEARER_TOKEN", "production-token")

	resolver := &InMemorySecretResolver{
		store: map[string]map[string]string{
			"ref": {
				"base_url":     "http://localhost:4001",
				"bearer_token": "development-token",
				"token":        "development-token",
			},
		},
	}

	credentials, err := resolver.Resolve(context.Background(), "ref")
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if got := credentials["base_url"]; got != "https://pms.example" {
		t.Fatalf("base_url = %q, want production URL", got)
	}
	if got := credentials["bearer_token"]; got != "production-token" {
		t.Fatalf("bearer_token = %q, want production token", got)
	}
	if got := credentials["token"]; got != "production-token" {
		t.Fatalf("token = %q, want production token", got)
	}
}

func TestInMemorySecretResolverResolveKeepsStoredCredentialsWithoutOverrides(t *testing.T) {
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
