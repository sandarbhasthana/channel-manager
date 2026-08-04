package http

import (
	"encoding/json"
	"net/http"

	pricingpostgres "github.com/channel-manager/channel-manager/services/pricing/adapters/postgres"
)

// ChannelRatesHandler serves the dashboard's per-channel pricing endpoints
// (/admin/channel-rates). Rules are the Channel Manager-owned adjustment applied
// on top of the PMS base rate.
type ChannelRatesHandler struct {
	repo *pricingpostgres.ChannelRateRepository
}

// NewChannelRatesHandler creates a handler backed by repo.
func NewChannelRatesHandler(repo *pricingpostgres.ChannelRateRepository) *ChannelRatesHandler {
	return &ChannelRatesHandler{repo: repo}
}

// List handles GET /admin/channel-rates?propertyId=...
func (h *ChannelRatesHandler) List(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(r, w) {
		return
	}
	propertyID := r.URL.Query().Get("propertyId")
	if propertyID == "" {
		jsonError(w, "propertyId is required", http.StatusBadRequest)
		return
	}
	rules, err := h.repo.List(r.Context(), propertyID)
	if err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	jsonOK(w, map[string]any{"rules": rules})
}

// Save handles PUT /admin/channel-rates — upserts a property's per-channel rules.
func (h *ChannelRatesHandler) Save(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(r, w) {
		return
	}
	var body struct {
		PropertyID string                            `json:"propertyId"`
		Rules      []pricingpostgres.ChannelRateRule `json:"rules"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.PropertyID == "" {
		jsonError(w, "propertyId and rules are required", http.StatusBadRequest)
		return
	}
	if err := h.repo.Upsert(r.Context(), body.PropertyID, body.Rules); err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	jsonOK(w, map[string]any{"saved": len(body.Rules)})
}
