package http

import (
	"encoding/json"
	"net/http"

	platformauth "github.com/channel-manager/channel-manager/platform/auth"
	platformintegration "github.com/channel-manager/channel-manager/platform/integration"
)

// AdminKeysHandler manages integration API keys (WorkOS-authenticated admins).
type AdminKeysHandler struct {
	keystore *platformintegration.KeyStore
}

// NewAdminKeysHandler creates an admin handler.
func NewAdminKeysHandler(keystore *platformintegration.KeyStore) *AdminKeysHandler {
	return &AdminKeysHandler{keystore: keystore}
}

// ListKeys handles GET /admin/integration-keys.
func (h *AdminKeysHandler) ListKeys(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(r, w) {
		return
	}
	keys, err := h.keystore.ListKeys(r.Context())
	if err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	jsonOK(w, map[string]any{"keys": keys})
}

// CreateKey handles POST /admin/integration-keys.
func (h *AdminKeysHandler) CreateKey(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(r, w) {
		return
	}
	var body struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Name == "" {
		jsonError(w, "name is required", http.StatusBadRequest)
		return
	}
	tc, _ := platformauth.FromContext(r.Context())
	result, err := h.keystore.CreateKey(r.Context(), body.Name, tc.UserID, nil)
	if err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusCreated)
	jsonOK(w, result)
}

// RevokeKey handles DELETE /admin/integration-keys/{id}.
func (h *AdminKeysHandler) RevokeKey(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(r, w) {
		return
	}
	id := r.PathValue("id")
	if id == "" {
		jsonError(w, "id required", http.StatusBadRequest)
		return
	}
	if err := h.keystore.RevokeKey(r.Context(), id); err != nil {
		jsonError(w, err.Error(), http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func requireAdmin(r *http.Request, w http.ResponseWriter) bool {
	tc, err := platformauth.FromContext(r.Context())
	if err != nil {
		jsonError(w, "unauthorized", http.StatusUnauthorized)
		return false
	}
	if tc.Role != platformauth.RoleOwner && tc.Role != platformauth.RoleAdmin {
		jsonError(w, "forbidden", http.StatusForbidden)
		return false
	}
	return true
}
