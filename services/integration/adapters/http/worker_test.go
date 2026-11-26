package http

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/channel-manager/channel-manager/services/integration/usecases"
)

type fakeRunner struct{ ran []string }

func (f *fakeRunner) RunJob(_ context.Context, _ usecases.OrgLister, job string) (usecases.JobResult, error) {
	f.ran = append(f.ran, job)
	return usecases.JobResult{Job: job}, nil
}

func serve(h http.Handler, auth string) int {
	mux := http.NewServeMux()
	mux.Handle("POST /internal/worker/jobs/{job}", h)
	req := httptest.NewRequest(http.MethodPost, "/internal/worker/jobs/redeliver", nil)
	if auth != "" {
		req.Header.Set("Authorization", auth)
	}
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	return rec.Code
}

func TestWorkerEndpointNeedsTheToken(t *testing.T) {
	runner := &fakeRunner{}
	h := NewWorkerHandler(runner, nil, "s3cret")
	if code := serve(h, ""); code != http.StatusUnauthorized {
		t.Errorf("no token: %d", code)
	}
	if code := serve(h, "Bearer wrong"); code != http.StatusUnauthorized {
		t.Errorf("wrong token: %d", code)
	}
	if len(runner.ran) != 0 {
		t.Fatal("ran a job without the token")
	}
	if code := serve(h, "Bearer s3cret"); code != http.StatusOK {
		t.Errorf("right token: %d", code)
	}
	if len(runner.ran) != 1 || runner.ran[0] != "redeliver" {
		t.Errorf("ran %v", runner.ran)
	}
}

func TestWorkerEndpointClosedWithoutAConfiguredToken(t *testing.T) {
	runner := &fakeRunner{}
	if code := serve(NewWorkerHandler(runner, nil, ""), "Bearer "); code != http.StatusUnauthorized {
		t.Errorf("unconfigured endpoint answered %d", code)
	}
	if len(runner.ran) != 0 {
		t.Fatal("ran a job with no token configured")
	}
}
