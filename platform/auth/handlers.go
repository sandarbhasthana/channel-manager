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
// User-level sessions without an org are attached to the deployment's
// configured default tenant, which is provisioned locally on first login.
func syncIdentity(r *http.Request, wos *workos.Client, store *Store, orgID *string, user *workos.User, defaultOrganizationID string) string {
	var localOrgID string
	if orgID != nil && *orgID != "" {
		org, err := wos.Organizations().Get(r.Context(), *orgID)
		if err != nil {
			slog.WarnContext(r.Context(), "auth: fetch org failed", "err", err)
		} else {
			localOrgID, _ = store.UpsertOrg(r.Context(), org.ID, org.Name)
		}
	} else if defaultOrganizationID != "" {
		var err error
		localOrgID, err = store.UpsertOrg(r.Context(), defaultOrganizationID, "Default Organization")
		if err != nil {
			slog.ErrorContext(r.Context(), "auth: provision default org failed", "err", err)
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
func CallbackHandler(wos *workos.Client, store *Store, defaultOrganizationID string) http.HandlerFunc {
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

		syncIdentity(r, wos, store, resp.OrganizationID, resp.User, defaultOrganizationID)
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
func PasswordLoginHandler(wos *workos.Client, store *Store, defaultOrganizationID string) http.HandlerFunc {
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

		syncIdentity(r, wos, store, resp.OrganizationID, resp.User, defaultOrganizationID)
		setAuthCookies(w, resp.AccessToken, resp.RefreshToken)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]bool{"ok": true})
	}
}

// MeHandler returns the TenantContext attached by the auth middleware as JSON.
// Mount it behind NewMiddleware to ensure the context is always present.
func MeHandler(store *Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tc, err := FromContext(r.Context())
		if err != nil {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		tx, err := store.pool.Begin(r.Context())
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		defer tx.Rollback(r.Context())

		if _, err := tx.Exec(r.Context(), "SELECT set_config('app.current_org_id', $1, true)", tc.OrgID); err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		var email, fullName string
		var preferences []byte
		err = tx.QueryRow(r.Context(), `
			SELECT u.email, u.full_name, m.preferences
			FROM tenancy.users u
			LEFT JOIN tenancy.memberships m ON u.id = m.user_id AND m.org_id = $2
			WHERE u.id = $1
		`, tc.UserID, tc.OrgID).Scan(&email, &fullName, &preferences)

		_ = tx.Commit(r.Context())

		if err != nil {
			http.Error(w, "user not found", http.StatusInternalServerError)
			return
		}

		var prefs map[string]interface{}
		if len(preferences) > 0 {
			_ = json.Unmarshal(preferences, &prefs)
		} else {
			prefs = make(map[string]interface{})
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"user_id":     tc.UserID,
			"org_id":      tc.OrgID,
			"role":        tc.Role,
			"email":       email,
			"full_name":   fullName,
			"preferences": prefs,
		})
	}
}

// UpdatePreferencesHandler updates the user's preferences in tenancy.memberships.
func UpdatePreferencesHandler(store *Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tc, err := FromContext(r.Context())
		if err != nil {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		var req struct {
			Preferences map[string]interface{} `json:"preferences"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}

		prefsBytes, err := json.Marshal(req.Preferences)
		if err != nil {
			http.Error(w, "invalid preferences", http.StatusBadRequest)
			return
		}

		tx, err := store.pool.Begin(r.Context())
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		defer tx.Rollback(r.Context())

		if _, err := tx.Exec(r.Context(), "SELECT set_config('app.current_org_id', $1, true)", tc.OrgID); err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		_, err = tx.Exec(r.Context(), `
			INSERT INTO tenancy.memberships (org_id, user_id, preferences)
			VALUES ($3, $2, $1)
			ON CONFLICT (org_id, user_id)
			DO UPDATE SET preferences = $1, updated_at = now()
		`, prefsBytes, tc.UserID, tc.OrgID)

		if err != nil {
			slog.Error("failed to update preferences", "err", err)
			http.Error(w, "failed to update preferences: "+err.Error(), http.StatusInternalServerError)
			return
		}

		if err := tx.Commit(r.Context()); err != nil {
			http.Error(w, "failed to commit", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]bool{"ok": true})
	}
}
