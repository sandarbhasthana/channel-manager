package auth

import (
	"encoding/json"
	"log/slog"
	"net/http"

	workos "github.com/workos/workos-go/v7"
)

// ListInvitationsHandler returns pending invitations for the org.
func ListInvitationsHandler(wos *workos.Client, store *Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tc, err := FromContext(r.Context())
		if err != nil {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		wOrgID, err := store.ResolveWorkosOrgID(r.Context(), tc.OrgID)
		if err != nil {
			slog.Error("team: resolve org for invites", "err", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		iter := wos.UserManagement().ListInvitations(r.Context(),
			&workos.UserManagementListInvitationsParams{OrganizationID: &wOrgID},
		)
		var invites []teamInvitation
		for iter.Next() {
			inv := iter.Current()
			invites = append(invites, teamInvitation{
				ID: inv.ID, Email: inv.Email,
				State:     string(inv.State),
				CreatedAt: inv.CreatedAt,
				ExpiresAt: inv.ExpiresAt,
			})
		}
		if err := iter.Err(); err != nil {
			slog.Error("team: list invitations", "err", err)
			http.Error(w, "failed to list invitations", http.StatusInternalServerError)
			return
		}
		if invites == nil {
			invites = []teamInvitation{}
		}
		jsonOK(w, map[string]any{"invitations": invites})
	}
}

// RevokeInvitationHandler revokes a pending invitation by ID.
func RevokeInvitationHandler(wos *workos.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, err := FromContext(r.Context()); err != nil {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		var req struct {
			ID string `json:"id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.ID == "" {
			jsonError(w, "invitation id is required", http.StatusBadRequest)
			return
		}
		if _, err := wos.UserManagement().RevokeInvitation(r.Context(), req.ID); err != nil {
			slog.Error("team: revoke invitation", "err", err, "id", req.ID)
			jsonError(w, err.Error(), http.StatusBadRequest)
			return
		}
		jsonOK(w, map[string]bool{"ok": true})
	}
}

// RemoveMemberHandler removes an org membership. Cannot remove yourself.
func RemoveMemberHandler(wos *workos.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tc, err := FromContext(r.Context())
		if err != nil {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		var req struct {
			MembershipID string `json:"membershipId"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.MembershipID == "" {
			jsonError(w, "membershipId is required", http.StatusBadRequest)
			return
		}
		// Verify membership and prevent self-removal.
		membership, err := wos.UserManagement().GetOrganizationMembership(r.Context(), req.MembershipID)
		if err != nil {
			slog.Error("team: get membership", "err", err)
			jsonError(w, "membership not found", http.StatusNotFound)
			return
		}
		if membership.UserID == tc.UserID {
			jsonError(w, "cannot remove yourself", http.StatusBadRequest)
			return
		}
		if err := wos.UserManagement().DeleteOrganizationMembership(r.Context(), req.MembershipID); err != nil {
			slog.Error("team: remove member", "err", err)
			jsonError(w, err.Error(), http.StatusBadRequest)
			return
		}
		jsonOK(w, map[string]bool{"ok": true})
	}
}
