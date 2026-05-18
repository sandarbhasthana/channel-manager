package integration

import (
	"log/slog"
	"net/http"
	"strings"

	platformauth "github.com/channel-manager/channel-manager/platform/auth"
)

// Authenticator resolves PMS integration API keys to org IDs.
type Authenticator struct {
	env     *EnvSecrets
	keystore *KeyStore
}

// NewAuthenticator creates an authenticator using env secrets and optional DB keystore.
func NewAuthenticator(env *EnvSecrets, keystore *KeyStore) *Authenticator {
	return &Authenticator{env: env, keystore: keystore}
}

// Middleware validates Authorization: Bearer and attaches TenantContext for RLS.
func (a *Authenticator) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := bearerToken(r.Header.Get("Authorization"))
		if token == "" {
			jsonError(w, "missing bearer token", http.StatusUnauthorized)
			return
		}

		orgID := ""
		if a.env != nil {
			orgID = a.env.ResolveOrgID(token)
		}
		if orgID == "" && a.keystore != nil {
			var err error
			orgID, err = a.keystore.ResolveOrgID(r.Context(), token)
			if err != nil {
				slog.Error("integration.auth: keystore lookup failed", "err", err)
				jsonError(w, "internal error", http.StatusInternalServerError)
				return
			}
		}
		if orgID == "" {
			jsonError(w, "invalid api key", http.StatusUnauthorized)
			return
		}

		ctx := platformauth.WithTenantContext(r.Context(), platformauth.TenantContext{
			UserID: "integration:pms",
			OrgID:  orgID,
			Role:   "integration",
		})
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func bearerToken(header string) string {
	const prefix = "Bearer "
	if !strings.HasPrefix(header, prefix) {
		return ""
	}
	return strings.TrimSpace(strings.TrimPrefix(header, prefix))
}

func jsonError(w http.ResponseWriter, msg string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_, _ = w.Write([]byte(`{"error":"` + msg + `"}`))
}
