package integration

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	platformauth "github.com/channel-manager/channel-manager/platform/auth"
)

// OrgProvisioner materializes a bundled tenant from the identity a PMS asserts.
//
// An interface rather than a concrete *auth.Store so this package keeps depending
// only on the auth package's context helpers, and so tests can authenticate
// without a database.
type OrgProvisioner interface {
	EnsureOrgByExternalID(ctx context.Context, externalID, name string) (string, error)
}

// Authenticator resolves inbound machine credentials to an org.
//
// Three branches, in priority order:
//
//  1. A PMS assertion — a short-lived Ed25519 JWT from a bundled tenant's PMS.
//     No credential is stored on either side; the organization is named in the
//     claims and created on first sight.
//  2. A database API key — `cm_live_…`, issued to standalone customers who
//     connect their own PMS.
//  3. Env secrets — a static token→org map, dev only (see NewEnvSecrets).
//
// Order matters: assertions are checked first and, once a value looks like a JWT,
// a verification failure is terminal. Falling through to the key lookup would let
// a forged assertion be reported as an unknown API key, which is both the wrong
// status and the wrong thing in the logs.
type Authenticator struct {
	env      *EnvSecrets
	keystore *KeyStore
	pms      *PmsVerifier
	orgs     OrgProvisioner
}

// NewAuthenticator creates an authenticator using env secrets and optional DB keystore.
func NewAuthenticator(env *EnvSecrets, keystore *KeyStore) *Authenticator {
	return &Authenticator{env: env, keystore: keystore}
}

// WithPmsAssertions enables the bundled-tenant branch. Both arguments are
// required for it to activate: a verifier with no provisioner could authenticate
// a tenant it cannot resolve to an org id, which would pass auth and then fail
// every query underneath it.
func (a *Authenticator) WithPmsAssertions(v *PmsVerifier, orgs OrgProvisioner) *Authenticator {
	if v == nil || orgs == nil {
		return a
	}
	a.pms = v
	a.orgs = orgs
	return a
}

// Middleware validates Authorization: Bearer and attaches TenantContext for RLS.
func (a *Authenticator) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := bearerToken(r.Header.Get("Authorization"))
		if token == "" {
			jsonError(w, "missing bearer token", http.StatusUnauthorized)
			return
		}

		// Branch 1: bundled PMS assertion.
		if a.pms != nil {
			claims, err := a.pms.Verify(token)
			switch {
			case err == nil:
				orgID, err := a.orgs.EnsureOrgByExternalID(r.Context(), claims.OrgExternalID, claims.OrgName)
				if err != nil {
					slog.Error("integration.auth: provision org failed",
						"external_id", claims.OrgExternalID, "err", err)
					jsonError(w, "internal error", http.StatusInternalServerError)
					return
				}
				next.ServeHTTP(w, r.WithContext(
					platformauth.WithTenantContext(r.Context(), tenantContextFor(claims, orgID)),
				))
				return
			case errors.Is(err, ErrNoPmsToken):
				// Not a JWT — fall through to the API-key branches below.
			default:
				slog.Warn("integration.auth: rejected PMS assertion", "err", err)
				jsonError(w, "invalid token", http.StatusUnauthorized)
				return
			}
		}

		// Branch 2 and 3: API keys.
		orgID := ""
		if a.keystore != nil {
			var err error
			orgID, err = a.keystore.ResolveOrgID(r.Context(), token)
			if err != nil {
				slog.Error("integration.auth: keystore lookup failed", "err", err)
				jsonError(w, "internal error", http.StatusInternalServerError)
				return
			}
		}
		if orgID == "" && a.env != nil {
			orgID = a.env.ResolveOrgID(token)
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

// tenantContextFor carries the acting PMS user through as the actor, so audit
// entries name a person instead of the service. The id is namespaced because it
// comes from a different identity system than the WorkOS subjects in
// tenancy.users — an unprefixed id would look like a local user that does not
// exist.
//
// Role stays "integration" regardless of the user's PMS role: authorization here
// is about what the machine channel may do, and the PMS has already decided
// whether this user may perform the action. Promoting a PMS role to a CM role
// would let the caller pick its own permissions.
func tenantContextFor(claims *PmsClaims, orgID string) platformauth.TenantContext {
	actor := "integration:pms"
	if claims.ActorUserID != "" {
		actor = "pms:" + claims.ActorUserID
	}
	return platformauth.TenantContext{
		UserID: actor,
		OrgID:  orgID,
		Role:   "integration",
	}
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
