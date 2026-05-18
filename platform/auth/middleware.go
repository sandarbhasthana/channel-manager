package auth

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/jackc/pgx/v5"
)

// NewMiddleware returns an HTTP middleware that:
//  1. Extracts and verifies the Bearer JWT — first from the Authorization header,
//     then from the "access_token" HttpOnly cookie set by CallbackHandler.
//  2. Resolves the WorkOS org_id claim to the local UUID (via the identity Store).
//  3. Attaches a TenantContext to the request context for downstream handlers.
//
// RLS (SET LOCAL app.current_org_id) is applied at the DB layer inside
// platform/db.Pool.WithTenant; this middleware only sets the Go context.
func NewMiddleware(v *Verifier, s *Store) func(http.Handler) http.Handler {
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
				slog.Error("auth.middleware: token verification failed", "err", err, "path", r.URL.Path)
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}

			slog.Info("auth.middleware: token verified",
				"sub", claims.Subject(), "org_id", claims.OrganizationID, "role", claims.Role, "path", r.URL.Path)

			if claims.OrganizationID == "" {
				slog.Error("auth.middleware: empty org_id in token", "sub", claims.Subject())
				http.Error(w, "organization context required", http.StatusUnauthorized)
				return
			}

			localOrgID, err := s.ResolveOrgID(r.Context(), claims.OrganizationID)
			if err != nil {
				if errors.Is(err, pgx.ErrNoRows) {
					slog.Error("auth.middleware: org not mirrored", "workos_org_id", claims.OrganizationID)
					http.Error(w, "organization not registered", http.StatusForbidden)
					return
				}
				slog.Error("auth.middleware: resolve org failed", "err", err)
				http.Error(w, "internal error", http.StatusInternalServerError)
				return
			}

			ctx := WithTenantContext(r.Context(), TenantContext{
				UserID: claims.Subject(),
				OrgID:  localOrgID,
				Role:   claims.Role,
			})
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
