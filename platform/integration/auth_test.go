package integration_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	platformintegration "github.com/channel-manager/channel-manager/platform/integration"
)

func TestMiddleware_ValidEnvSecret(t *testing.T) {
	env, err := platformintegration.LoadEnvSecretsFromJSON(`{"org-1":"secret-token"}`)
	if err != nil {
		t.Fatal(err)
	}
	auth := platformintegration.NewAuthenticator(env, nil)

	var gotOrg string
	h := auth.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// TenantContext checked indirectly via handler success
		gotOrg = "ok"
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/integrations/pms", nil)
	req.Header.Set("Authorization", "Bearer secret-token")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body %s", rec.Code, rec.Body.String())
	}
	if gotOrg != "ok" {
		t.Fatal("handler not called")
	}
}

func TestMiddleware_MissingToken(t *testing.T) {
	env, _ := platformintegration.LoadEnvSecretsFromJSON(`{}`)
	auth := platformintegration.NewAuthenticator(env, nil)
	h := auth.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d", rec.Code)
	}
}
