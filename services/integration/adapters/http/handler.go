// Package http exposes PMS-facing outbound REST APIs.
package http

import (
	"encoding/json"
	"net/http"

	platformauth "github.com/channel-manager/channel-manager/platform/auth"
	"github.com/channel-manager/channel-manager/services/integration/usecases"
)

// Handler serves /api/integrations/pms routes.
type Handler struct {
	svc *usecases.Service
}

// NewHandler creates a new handler.
func NewHandler(svc *usecases.Service) *Handler {
	return &Handler{svc: svc}
}

// OrgHealth handles GET /api/integrations/pms.
func (h *Handler) OrgHealth(w http.ResponseWriter, r *http.Request) {
	tc, err := platformauth.FromContext(r.Context())
	if err != nil {
		jsonError(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	jsonOK(w, h.svc.OrgHealth(r.Context(), tc.OrgID))
}

// PropertyHealth handles GET /api/integrations/pms/{propertyId}.
func (h *Handler) PropertyHealth(w http.ResponseWriter, r *http.Request) {
	propertyID := r.PathValue("propertyId")
	if propertyID == "" {
		jsonError(w, "propertyId required", http.StatusBadRequest)
		return
	}
	resp, err := h.svc.PropertyHealth(r.Context(), propertyID)
	if err != nil {
		jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}
	jsonOK(w, resp)
}

// Dispatch handles POST /api/integrations/pms/{propertyId}.
func (h *Handler) Dispatch(w http.ResponseWriter, r *http.Request) {
	propertyID := r.PathValue("propertyId")
	if propertyID == "" {
		jsonError(w, "propertyId required", http.StatusBadRequest)
		return
	}
	var body map[string]any
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		jsonError(w, "invalid json body", http.StatusBadRequest)
		return
	}
	action, _ := body["action"].(string)
	if action == "" {
		jsonError(w, "action is required", http.StatusBadRequest)
		return
	}
	resp, err := h.svc.Dispatch(r.Context(), propertyID, action, body)
	if err != nil {
		jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}
	jsonOK(w, resp)
}

func jsonOK(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

func jsonError(w http.ResponseWriter, msg string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
