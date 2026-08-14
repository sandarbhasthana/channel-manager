// Package http exposes the guest-facing storefront REST API.
//
// Responses wrap their payload in a top-level "data" object, matching the PMS
// booking webhook envelope, so an existing booking engine can be repointed at
// Channel Manager without changing how it parses responses.
package http

import (
	"encoding/json"
	"errors"
	"net/http"

	platformauth "github.com/channel-manager/channel-manager/platform/auth"
	pricingdomain "github.com/channel-manager/channel-manager/services/pricing/domain"
	"github.com/channel-manager/channel-manager/services/storefront/domain"
	"github.com/channel-manager/channel-manager/services/storefront/usecases"
)

// Handler serves /api/storefront/v1 routes.
type Handler struct {
	svc *usecases.Service
}

// NewHandler creates a new storefront handler.
func NewHandler(svc *usecases.Service) *Handler {
	return &Handler{svc: svc}
}

// Health handles GET /api/storefront/v1/health.
func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	tc, err := platformauth.FromContext(r.Context())
	if err != nil {
		jsonError(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	jsonOK(w, h.svc.Health(r.Context(), tc.OrgID))
}

// Dispatch handles POST /api/storefront/v1/{propertyId}.
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

	// Allow the idempotency key to arrive as a header, which is the more
	// conventional transport for it, while still honouring a body field.
	if key := r.Header.Get("Idempotency-Key"); key != "" {
		if _, present := body["idempotency_key"]; !present {
			body["idempotency_key"] = key
		}
	}

	resp, err := h.svc.Dispatch(r.Context(), propertyID, action, body)
	if err != nil {
		jsonError(w, err.Error(), statusFor(err))
		return
	}
	jsonOK(w, map[string]any{"data": resp})
}

type httpStatusError interface {
	HTTPStatus() int
}

// statusFor maps domain errors onto HTTP status codes. A replayed idempotency
// key and an expired hold are both client-correctable conflicts, not 500s.
// Upstream PMS 404/409 (and other 4xx/5xx) are preserved so the agent can tell
// "not found" and "already booked" apart from a malformed request.
func statusFor(err error) int {
	var coded httpStatusError
	if errors.As(err, &coded) {
		if status := coded.HTTPStatus(); status >= 400 && status <= 599 {
			return status
		}
	}
	switch {
	case errors.Is(err, domain.ErrDuplicateRequest):
		return http.StatusConflict
	case errors.Is(err, domain.ErrHoldNotFound):
		return http.StatusGone
	case errors.Is(err, domain.ErrBookingEngineDisabled):
		return http.StatusForbidden
	case errors.Is(err, pricingdomain.ErrPromoNotFound):
		return http.StatusNotFound
	// A code that exists but cannot be used right now is a conflict with the
	// code's state, not a malformed request.
	case errors.Is(err, pricingdomain.ErrPromoExhausted),
		errors.Is(err, pricingdomain.ErrPromoExpired),
		errors.Is(err, pricingdomain.ErrPromoInactive),
		errors.Is(err, pricingdomain.ErrPromoNotYetValid),
		errors.Is(err, pricingdomain.ErrPromoWrongScope):
		return http.StatusConflict
	default:
		return http.StatusBadRequest
	}
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
