package http

import (
	"encoding/json"
	"net/http"

	pricingpostgres "github.com/channel-manager/channel-manager/services/pricing/adapters/postgres"
)

// RoomBaseRatesHandler serves the dashboard's CM-stored base-rate endpoints
// (/admin/room-base-rates). Used when the live PMS quote is unavailable.
type RoomBaseRatesHandler struct {
	repo *pricingpostgres.RoomBaseRateRepository
}

// NewRoomBaseRatesHandler creates a handler backed by repo.
func NewRoomBaseRatesHandler(repo *pricingpostgres.RoomBaseRateRepository) *RoomBaseRatesHandler {
	return &RoomBaseRatesHandler{repo: repo}
}

// List handles GET /admin/room-base-rates?propertyId=...
func (h *RoomBaseRatesHandler) List(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(r, w) {
		return
	}
	propertyID := r.URL.Query().Get("propertyId")
	if propertyID == "" {
		jsonError(w, "propertyId is required", http.StatusBadRequest)
		return
	}
	rates, err := h.repo.List(r.Context(), propertyID)
	if err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	jsonOK(w, map[string]any{"baseRates": rates})
}

// Save handles PUT /admin/room-base-rates — upserts a property's base rates.
func (h *RoomBaseRatesHandler) Save(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(r, w) {
		return
	}
	var body struct {
		PropertyID string                         `json:"propertyId"`
		BaseRates  []pricingpostgres.RoomBaseRate `json:"baseRates"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.PropertyID == "" {
		jsonError(w, "propertyId and baseRates are required", http.StatusBadRequest)
		return
	}
	if err := h.repo.Upsert(r.Context(), body.PropertyID, body.BaseRates); err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	jsonOK(w, map[string]any{"saved": len(body.BaseRates)})
}
