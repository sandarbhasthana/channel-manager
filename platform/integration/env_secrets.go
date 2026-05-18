package integration

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// EnvSecrets maps integration API tokens to local org IDs (Phase A).
type EnvSecrets struct {
	tokenToOrg map[string]string
}

// LoadEnvSecrets parses CM_INTEGRATION_SECRETS JSON: { "org_uuid": "token", ... }.
func LoadEnvSecrets() (*EnvSecrets, error) {
	return LoadEnvSecretsFromJSON(os.Getenv("CM_INTEGRATION_SECRETS"))
}

// LoadEnvSecretsFromJSON parses the same map from a JSON string.
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
