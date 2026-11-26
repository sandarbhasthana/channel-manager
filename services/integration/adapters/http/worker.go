package http

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/channel-manager/channel-manager/services/integration/usecases"
)

// JobRunner runs one background job across all tenants.
type JobRunner interface {
	RunJob(ctx context.Context, orgs usecases.OrgLister, job string) (usecases.JobResult, error)
}

// WorkerHandler serves POST /internal/worker/jobs/{job}, the endpoint the sync
// worker calls on its schedule. It is guarded by a shared token
// (SYNC_WORKER_TOKEN) rather than a tenant credential, because a job spans
// every tenant. With no token configured the endpoint stays closed.
type WorkerHandler struct {
	runner JobRunner
	orgs   usecases.OrgLister
	token  string
}

// NewWorkerHandler creates the worker endpoint.
func NewWorkerHandler(runner JobRunner, orgs usecases.OrgLister, token string) *WorkerHandler {
	return &WorkerHandler{runner: runner, orgs: orgs, token: token}
}

func (h *WorkerHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if !h.authorized(r) {
		jsonError(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	job := r.PathValue("job")
	res, err := h.runner.RunJob(r.Context(), h.orgs, job)
	if err != nil {
		jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(res)
}

func (h *WorkerHandler) authorized(r *http.Request) bool {
	if h.token == "" {
		return false
	}
	got, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
	if !ok {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(got), []byte(h.token)) == 1
}
