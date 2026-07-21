package auth

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/jackc/pgx/v5"
	workos "github.com/workos/workos-go/v7"
)

// NewMiddleware returns an HTTP middleware that:
//  1. Extracts and verifies the Bearer JWT — first from the Authorization header,
//     then from the "access_token" HttpOnly cookie set by CallbackHandler.
//  2. Resolves the WorkOS org_id claim to the local UUID (via the identity Store).
//  3. Ensures the user's role is bound to Casbin policy for that org.
//  4. Attaches a TenantContext to the request context for downstream handlers.
//
// Step 3 is what makes RBAC self-healing: a user who signs in with a role has
// their `p` and `g` rules materialised before any procedure is enforced, so a
// newly created org is never authorised against an empty policy set. The binder
// memoises, so this costs a map load once the rules exist.
//
// RLS (SET LOCAL app.current_org_id) is applied at the DB layer inside
// platform/db.Pool.WithTenant; this middleware only sets the Go context.
func NewMiddleware(v *Verifier, s *Store, wos *workos.Client, binder *RoleBinder, defaultOrganizationID string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Prefer the Authorization header; fall back to the HttpOnly cookie.
			rawToken := r.Header.Get("Authorization")
			if rawToken == "" {
				if c, err := r.Cookie(accessTokenCookieName); err == nil {
					rawToken = c.Value
				}
			}
			claims, err := v.Verify(r.Context(), rawToken)
			if err != nil {
				refreshed := false
				if rc, rErr := r.Cookie(refreshTokenCookieName); rErr == nil && rc.Value != "" {
					params := &workos.UserManagementAuthenticateWithRefreshTokenParams{
						RefreshToken: rc.Value,
					}
					resp, wErr := wos.UserManagement().AuthenticateWithRefreshToken(r.Context(), params)
					if wErr == nil {
						setAuthCookies(w, resp.AccessToken, resp.RefreshToken)
						claims, err = v.Verify(r.Context(), resp.AccessToken)
						if err == nil {
							refreshed = true
						} else {
							slog.Error("auth.middleware: refreshed token verification failed", "err", err)
						}
					} else {
						slog.Error("auth.middleware: token refresh failed", "err", wErr)
					}
				}

				if !refreshed {
					slog.Error("auth.middleware: token verification failed", "err", err, "path", r.URL.Path)
					http.Error(w, "unauthorized", http.StatusUnauthorized)
					return
				}
			}

			slog.Info("auth.middleware: token verified",
				"sub", claims.Subject(), "org_id", claims.OrganizationID, "role", claims.Role, "path", r.URL.Path)

			orgID := claims.OrganizationID
			if orgID == "" {
				// Password and legacy social-login tokens may lack org_id. Bind them
				// to the deployment's explicitly configured tenant.
				orgID = defaultOrganizationID
			}

			if orgID == "" {
				slog.Error("auth.middleware: empty org_id in token", "sub", claims.Subject())
				http.Error(w, "organization context required", http.StatusUnauthorized)
				return
			}

			localOrgID, err := s.ResolveOrgID(r.Context(), orgID)
			if err != nil {
				if errors.Is(err, pgx.ErrNoRows) {
					slog.Error("auth.middleware: org not mirrored", "workos_org_id", orgID)
					http.Error(w, "organization not registered", http.StatusForbidden)
					return
				}
				slog.Error("auth.middleware: resolve org failed", "err", err)
				http.Error(w, "internal error", http.StatusInternalServerError)
				return
			}

			role := claims.Role
			if defaultOrganizationID != "" && orgID == defaultOrganizationID {
				// Local dev fallback: elevate to admin to avoid local WorkOS RBAC snags
				role = "admin"
			}

			// Materialise the role's policy in this org before enforcement runs.
			// A failure here would silently deny every procedure, so it is fatal
			// to the request rather than logged and ignored.
			if err := binder.Ensure(claims.Subject(), role, localOrgID); err != nil {
				slog.Error("auth.middleware: ensure role bindings failed", "err", err)
				http.Error(w, "internal error", http.StatusInternalServerError)
				return
			}

			ctx := WithTenantContext(r.Context(), TenantContext{
				UserID: claims.Subject(),
				OrgID:  localOrgID,
				Role:   role,
			})
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
