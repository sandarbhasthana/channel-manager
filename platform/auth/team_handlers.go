package auth

import (
	"encoding/json"
	"log/slog"
	"net/http"

	workos "github.com/workos/workos-go/v7"
)

type teamMember struct {
	ID        string  `json:"id"`
	UserID    string  `json:"userId"`
	Email     string  `json:"email"`
	FullName  string  `json:"fullName"`
	Role      string  `json:"role"`
	Status    string  `json:"status"`
	AvatarURL *string `json:"avatarUrl,omitempty"`
}

type teamInvitation struct {
	ID        string `json:"id"`
	Email     string `json:"email"`
	State     string `json:"state"`
	CreatedAt string `json:"createdAt"`
	ExpiresAt string `json:"expiresAt"`
}

func jsonError(w http.ResponseWriter, msg string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

func jsonOK(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

// ListTeamMembersHandler returns org members by querying WorkOS memberships.
func ListTeamMembersHandler(wos *workos.Client, store *Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tc, err := FromContext(r.Context())
		if err != nil {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		wOrgID, err := store.ResolveWorkosOrgID(r.Context(), tc.OrgID)
		if err != nil {
			slog.Error("team: resolve workos org", "err", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		iter := wos.UserManagement().ListOrganizationMemberships(r.Context(),
			&workos.UserManagementListOrganizationMembershipsParams{OrganizationID: &wOrgID},
		)
		var members []teamMember
		for iter.Next() {
			m := iter.Current()
			user, uerr := wos.UserManagement().Get(r.Context(), m.UserID)
			if uerr != nil {
				slog.Warn("team: fetch user", "user_id", m.UserID, "err", uerr)
				continue
			}
			members = append(members, teamMember{
				ID: m.ID, UserID: m.UserID, Email: user.Email,
				FullName:  buildFullName(user.FirstName, user.LastName),
				Role:      string(m.Role.Slug),
				Status:    string(m.Status),
				AvatarURL: user.ProfilePictureURL,
			})
		}
		if err := iter.Err(); err != nil {
			slog.Error("team: list memberships", "err", err)
			http.Error(w, "failed to list team", http.StatusInternalServerError)
			return
		}
		if members == nil {
			members = []teamMember{}
		}
		jsonOK(w, map[string]any{"members": members})
	}
}

// SendInvitationHandler sends a WorkOS email invitation to join the org.
func SendInvitationHandler(wos *workos.Client, store *Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tc, err := FromContext(r.Context())
		if err != nil {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		var req struct {
			Email string `json:"email"`
			Role  string `json:"role"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Email == "" {
			jsonError(w, "email is required", http.StatusBadRequest)
			return
		}
		wOrgID, err := store.ResolveWorkosOrgID(r.Context(), tc.OrgID)
		if err != nil {
			slog.Error("team: resolve org", "err", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		role := req.Role
		if role == "" {
			role = "member"
		}
		invite, err := wos.UserManagement().SendInvitation(r.Context(),
			&workos.UserManagementSendInvitationParams{
				Email: req.Email, OrganizationID: &wOrgID,
				RoleSlug: &role, InviterUserID: &tc.UserID,
			},
		)
		if err != nil {
			slog.Error("team: send invite", "err", err, "email", req.Email)
			jsonError(w, err.Error(), http.StatusBadRequest)
			return
		}
		jsonOK(w, map[string]any{"invitation": invite})
	}
}
