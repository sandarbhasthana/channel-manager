package auth

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"os"

	workos "github.com/workos/workos-go/v7"
)

// LoginHandler redirects the browser to the WorkOS AuthKit authorization URL.
// Optional query params:
//   - ?org_id=<workos-org-id>   — lands on the org's SSO screen directly.
//   - ?provider=GoogleOAuth|AppleOAuth — selects a specific OAuth provider.
func LoginHandler(wos *workos.Client, clientID, redirectURI string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		params := &workos.UserManagementGetAuthorizationURLParams{
			RedirectURI: redirectURI,
		}
		if orgID := r.URL.Query().Get("org_id"); orgID != "" {
			params.OrganizationID = &orgID
		}
		if p := r.URL.Query().Get("provider"); p != "" {
			provider := workos.UserManagementAuthenticationProvider(p)
			params.Provider = &provider
		}
		authURL := wos.UserManagement().GetAuthorizationURL(params)
		http.Redirect(w, r, authURL, http.StatusFound)
	}
}

// accessTokenCookieName and refreshTokenCookieName are the HttpOnly cookie names.
const (
	accessTokenCookieName  = "access_token"
	refreshTokenCookieName = "refresh_token"
)

// setAuthCookies writes the access and (if non-empty) refresh token as HttpOnly
// cookies. SameSite=Lax allows top-level navigation redirects (needed for OAuth)
// while blocking cross-site sub-resource requests. Set Secure=true in production.
func setAuthCookies(w http.ResponseWriter, accessToken, refreshToken string) {
	http.SetCookie(w, &http.Cookie{
		Name:     accessTokenCookieName,
		Value:    accessToken,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
	if refreshToken != "" {
		http.SetCookie(w, &http.Cookie{
			Name:     refreshTokenCookieName,
			Value:    refreshToken,
			Path:     "/",
			HttpOnly: true,
			SameSite: http.SameSiteLaxMode,
		})
	}
}

// syncIdentity upserts the WorkOS org and user into the local tenancy mirror.
// Returns the local org UUID (empty when no org is present in the response).
func syncIdentity(r *http.Request, wos *workos.Client, store *Store, orgID *string, user *workos.User) string {
	var localOrgID string
	if orgID != nil && *orgID != "" {
		org, err := wos.Organizations().Get(r.Context(), *orgID)
		if err != nil {
			slog.WarnContext(r.Context(), "auth: fetch org failed", "err", err)
		} else {
			localOrgID, _ = store.UpsertOrg(r.Context(), org.ID, org.Name)
		}
	}
	if user != nil {
		if err := store.UpsertUser(r.Context(), user, localOrgID, ""); err != nil {
			slog.ErrorContext(r.Context(), "auth: upsert user failed", "err", err)
		}
	}
	return localOrgID
}

// CallbackHandler handles the OAuth code exchange, syncs the WorkOS identity
// into the local tenancy schema, and stores the tokens as HttpOnly cookies.
// The browser is then redirected to /me so clients can verify their session.
func CallbackHandler(wos *workos.Client, store *Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		code := r.URL.Query().Get("code")
		if code == "" {
			http.Error(w, "missing code", http.StatusBadRequest)
			return
		}

		resp, err := wos.UserManagement().AuthenticateWithCode(r.Context(),
			&workos.UserManagementAuthenticateWithCodeParams{Code: code},
		)
		if err != nil {
			slog.ErrorContext(r.Context(), "auth: code exchange failed", "err", err)
			http.Error(w, "authentication failed", http.StatusUnauthorized)
			return
		}

		syncIdentity(r, wos, store, resp.OrganizationID, resp.User)
		setAuthCookies(w, resp.AccessToken, resp.RefreshToken)

		// Redirect to dashboard after successful auth (for local dev).
		// In production, use the same domain for API and dashboard to avoid CORS.
		dashboardURL := os.Getenv("DASHBOARD_URL")
		if dashboardURL == "" {
			dashboardURL = "http://localhost:3000"
		}
		http.Redirect(w, r, dashboardURL+"/dashboard", http.StatusFound)
	}
}

// passwordLoginRequest is the JSON body accepted by PasswordLoginHandler.
type passwordLoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// PasswordLoginHandler authenticates a user with email and password via WorkOS
// User Management. On success it sets HttpOnly cookies and returns {"ok":true}.
// On failure it returns 401 with {"error":"..."}.
func PasswordLoginHandler(wos *workos.Client, store *Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req passwordLoginRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Email == "" || req.Password == "" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "email and password are required"})
			return
		}

		resp, err := wos.UserManagement().AuthenticateWithPassword(r.Context(),
			&workos.UserManagementAuthenticateWithPasswordParams{
				Email:    req.Email,
				Password: req.Password,
			},
		)
		if err != nil {
			slog.WarnContext(r.Context(), "auth: password login failed", "email", req.Email, "err", err)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "Invalid email or password"})
			return
		}

		syncIdentity(r, wos, store, resp.OrganizationID, resp.User)
		setAuthCookies(w, resp.AccessToken, resp.RefreshToken)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]bool{"ok": true})
	}
}

// MeHandler returns the TenantContext attached by the auth middleware as JSON.
// Mount it behind NewMiddleware to ensure the context is always present.
func MeHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tc, err := FromContext(r.Context())
		if err != nil {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{
			"user_id": tc.UserID,
			"org_id":  tc.OrgID,
			"role":    tc.Role,
		})
	}
}
