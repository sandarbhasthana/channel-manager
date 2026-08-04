package http

import (
	"encoding/json"
	"net/http"

	platformauth "github.com/channel-manager/channel-manager/platform/auth"
	pmspostgres "github.com/channel-manager/channel-manager/services/pms/adapters/postgres"
)

// DefaultPropertyHandler serves the dashboard's org-level default-property
// endpoints (/admin/default-property) — the star in the property picker.
//
// The default is org-level rather than a per-user preference because the
// booking engine reads the same value through the storefront using an
// org-scoped integration key, with no membership behind it. A default stored on
// tenancy.memberships.preferences would be invisible to it.
type DefaultPropertyHandler struct {
	repo *pmspostgres.PropertyRepository
}

// NewDefaultPropertyHandler creates a handler backed by repo.
func NewDefaultPropertyHandler(repo *pmspostgres.PropertyRepository) *DefaultPropertyHandler {
	return &DefaultPropertyHandler{repo: repo}
}

// Get handles GET /admin/default-property.
//
// Readable by any member, not just admins: every user's property picker needs
// to know which entry to star, and the value is not sensitive.
func (h *DefaultPropertyHandler) Get(w http.ResponseWriter, r *http.Request) {
	if _, err := platformauth.FromContext(r.Context()); err != nil {
		jsonError(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	listings, err := h.repo.ListListings(r.Context())
	if err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	for _, p := range listings {
		if p.IsDefault {
			jsonOK(w, map[string]any{"propertyId": p.ID})
			return
		}
	}
	// No default set yet — a valid state for an org whose properties were all
	// created after the backfill migration ran.
	jsonOK(w, map[string]any{"propertyId": nil})
}

// Set handles PUT /admin/default-property, promoting one property org-wide.
func (h *DefaultPropertyHandler) Set(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(r, w) {
		return
	}
	var body struct {
		PropertyID string `json:"propertyId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.PropertyID == "" {
		jsonError(w, "propertyId is required", http.StatusBadRequest)
		return
	}
	if err := h.repo.SetDefault(r.Context(), body.PropertyID); err != nil {
		jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}
	jsonOK(w, map[string]any{"propertyId": body.PropertyID})
}
