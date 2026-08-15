package integration

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"strings"
)

// countEntries reports how many mappings were ignored, so the warning says how
// much was skipped without printing the tokens themselves.
func countEntries(raw string) int {
	var m map[string]string
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		return -1
	}
	return len(m)
}

// EnvSecrets maps integration API tokens to local org IDs.
//
// DEVELOPMENT ONLY. This is the last mechanism in the service that turns a static
// string into a tenant with no database validation of any kind: whatever UUID
// appears in the map becomes `app.current_org_id`, whether or not an organization
// by that id exists. It cannot be revoked, cannot expire, has no scopes, and is
// stored in plaintext in the environment.
//
// It survives because it makes local setup a one-line env var. It is gated behind
// an explicit opt-in so that surviving cannot mean silently authenticating a
// request in a deployed environment — see LoadEnvSecrets.
type EnvSecrets struct {
	tokenToOrg map[string]string
}

// DevSecretsFlag is the env var that must be set to "true" for CM_INTEGRATION_SECRETS
// to be honoured.
const DevSecretsFlag = "CM_ALLOW_DEV_INTEGRATION_SECRETS"

// LoadEnvSecrets parses the CM_INTEGRATION_SECRETS value: { "org_uuid": "token", ... }.
// Takes the raw string so configuration stays sourced from AppConfig rather than
// being read from the environment twice in two different places.
//
// Returns an empty (but usable) set unless DevSecretsFlag is explicitly enabled,
// so shipping a stale CM_INTEGRATION_SECRETS value to production is inert rather
// than a working credential. Opt-in rather than opt-out on purpose: a guard you
// must remember to turn on is one someone will forget to turn on, and forgetting
// here fails closed.
func LoadEnvSecrets(raw string) (*EnvSecrets, error) {
	if strings.TrimSpace(raw) == "" {
		return &EnvSecrets{tokenToOrg: map[string]string{}}, nil
	}
	if !strings.EqualFold(strings.TrimSpace(os.Getenv(DevSecretsFlag)), "true") {
		slog.Warn("integration: CM_INTEGRATION_SECRETS is set but ignored; "+
			"set "+DevSecretsFlag+"=true to enable it (development only)",
			"entries", countEntries(raw))
		return &EnvSecrets{tokenToOrg: map[string]string{}}, nil
	}
	slog.Warn("integration: dev env secrets ENABLED — static tokens can authenticate as any org id")
	return LoadEnvSecretsFromJSON(raw)
}

// LoadEnvSecretsFromJSON parses the same map from a JSON string, without the dev
// gate. Used by tests and by LoadEnvSecrets once the gate has been passed.
func LoadEnvSecretsFromJSON(raw string) (*EnvSecrets, error) {
	if strings.TrimSpace(raw) == "" {
		return &EnvSecrets{tokenToOrg: map[string]string{}}, nil
	}
	var orgToToken map[string]string
	if err := json.Unmarshal([]byte(raw), &orgToToken); err != nil {
		return nil, fmt.Errorf("integration: parse CM_INTEGRATION_SECRETS: %w", err)
	}
	tokenToOrg := make(map[string]string, len(orgToToken))
	for orgID, token := range orgToToken {
		if orgID == "" || token == "" {
			continue
		}
		tokenToOrg[token] = orgID
	}
	return &EnvSecrets{tokenToOrg: tokenToOrg}, nil
}

// ResolveOrgID returns the org for a bearer token, or "" if unknown.
func (e *EnvSecrets) ResolveOrgID(token string) string {
	if e == nil {
		return ""
	}
	return e.tokenToOrg[token]
}
