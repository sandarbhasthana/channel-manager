package auth

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"

	workos "github.com/workos/workos-go/v7"
)

// WebhookHandler handles incoming WorkOS webhook deliveries.
// Signature verification is performed via workos.WebhookVerifier before any
// event data is processed.
type WebhookHandler struct {
	verifier *workos.WebhookVerifier
	store    *Store
}

// NewWebhookHandler returns an http.Handler for POST /auth/webhook.
func NewWebhookHandler(secret string, store *Store) *WebhookHandler {
	return &WebhookHandler{
		verifier: workos.NewWebhookVerifier(secret),
		store:    store,
	}
}

// ServeHTTP implements http.Handler.
func (h *WebhookHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "cannot read body", http.StatusBadRequest)
		return
	}

	event, err := h.verifier.ConstructEvent(r.Header.Get("WorkOS-Signature"), string(body))
	if err != nil {
		http.Error(w, "invalid signature", http.StatusUnauthorized)
		return
	}

	ctx := r.Context()
	switch event.Event {
	case "organization.created", "organization.updated":
		h.handleOrg(w, r, event)
	case "user.created", "user.updated":
		h.handleUser(w, r, event)
	case "organization_membership.created", "organization_membership.deleted",
		"organization_membership.updated":
		h.handleMembership(w, r, event)
	default:
		slog.InfoContext(ctx, "webhook: unhandled event", "event", event.Event)
	}
	w.WriteHeader(http.StatusNoContent)
}

// orgEventData extracts the fields we care about from an org event payload.
type orgEventData struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

func (h *WebhookHandler) handleOrg(w http.ResponseWriter, r *http.Request, event *workos.EventSchema) {
	raw, _ := json.Marshal(event.Data)
	var d orgEventData
	if err := json.Unmarshal(raw, &d); err != nil || d.ID == "" {
		slog.ErrorContext(r.Context(), "webhook: malformed org event", "err", err)
		http.Error(w, "bad payload", http.StatusBadRequest)
		return
	}
	if _, err := h.store.UpsertOrg(r.Context(), d.ID, d.Name); err != nil {
		slog.ErrorContext(r.Context(), "webhook: upsert org", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
}

// userEventData extracts the user fields we care about from a user event payload.
type userEventData struct {
	ID        string  `json:"id"`
	Email     string  `json:"email"`
	FirstName *string `json:"first_name"`
	LastName  *string `json:"last_name"`
}

func (h *WebhookHandler) handleUser(w http.ResponseWriter, r *http.Request, event *workos.EventSchema) {
	raw, _ := json.Marshal(event.Data)
	var d userEventData
	if err := json.Unmarshal(raw, &d); err != nil || d.ID == "" {
		slog.ErrorContext(r.Context(), "webhook: malformed user event", "err", err)
		http.Error(w, "bad payload", http.StatusBadRequest)
		return
	}
	u := &workos.User{
		ID:        d.ID,
		Email:     d.Email,
		FirstName: d.FirstName,
		LastName:  d.LastName,
	}
	// Upsert with no org context; a subsequent membership event will link the user.
	if err := h.store.UpsertUser(r.Context(), u, "", ""); err != nil {
		slog.ErrorContext(r.Context(), "webhook: upsert user", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
}

// membershipEventData extracts the fields we need from a membership event payload.
type membershipEventData struct {
	OrganizationID string `json:"organization_id"`
	UserID         string `json:"user_id"`
	Role           struct {
		Slug string `json:"slug"`
	} `json:"role"`
}

func (h *WebhookHandler) handleMembership(w http.ResponseWriter, r *http.Request, event *workos.EventSchema) {
	raw, _ := json.Marshal(event.Data)
	var d membershipEventData
	if err := json.Unmarshal(raw, &d); err != nil || d.OrganizationID == "" || d.UserID == "" {
		slog.ErrorContext(r.Context(), "webhook: malformed membership event", "err", err)
		http.Error(w, "bad payload", http.StatusBadRequest)
		return
	}
	localOrgID, err := h.store.ResolveOrgID(r.Context(), d.OrganizationID)
	if err != nil {
		slog.WarnContext(r.Context(), "webhook: org not mirrored yet, asking for redelivery", "org", d.OrganizationID)
		http.Error(w, "org not mirrored yet", http.StatusServiceUnavailable)
		return
	}

	if event.Event == "organization_membership.deleted" {
		if err := h.store.DeleteMembership(r.Context(), localOrgID, d.UserID); err != nil {
			slog.ErrorContext(r.Context(), "webhook: delete membership failed", "err", err, "user", d.UserID)
			http.Error(w, "delete membership failed", http.StatusInternalServerError)
			return
		}
		slog.InfoContext(r.Context(), "webhook: membership deleted", "org", localOrgID, "user", d.UserID)
		return
	}

	// memberships.user_id references tenancy.users, so a membership event that
	// overtakes its user.created event cannot be persisted yet. Fail the
	// delivery so WorkOS redelivers, rather than silently dropping the role.
	mirrored, err := h.store.UserExists(r.Context(), d.UserID)
	if err != nil {
		slog.ErrorContext(r.Context(), "webhook: user lookup failed", "err", err, "user", d.UserID)
		http.Error(w, "user lookup failed", http.StatusInternalServerError)
		return
	}
	if !mirrored {
		slog.WarnContext(r.Context(), "webhook: user not mirrored yet, asking for redelivery", "user", d.UserID)
		http.Error(w, "user not mirrored yet", http.StatusServiceUnavailable)
		return
	}

	role := NormalizeRole(d.Role.Slug)
	if err := h.store.UpsertMembership(r.Context(), localOrgID, d.UserID, role); err != nil {
		slog.ErrorContext(r.Context(), "webhook: upsert membership failed", "err", err, "user", d.UserID)
		http.Error(w, "upsert membership failed", http.StatusInternalServerError)
		return
	}

	slog.InfoContext(r.Context(), "webhook: membership event processed",
		"event", event.Event, "org", localOrgID, "user", d.UserID,
		"slug", d.Role.Slug, "role", role)
}
