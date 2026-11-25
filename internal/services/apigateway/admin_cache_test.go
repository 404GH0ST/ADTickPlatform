package apigateway

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAdminResponsesDoNotCacheSecrets(t *testing.T) {
	mux := newTestMux()
	for _, path := range []string{
		"/api/v2/admin/teams",
		"/api/v2/admin/players/1/wireguard",
		"/api/v2/admin/platform/settings",
	} {
		request := httptest.NewRequest(http.MethodGet, path, nil)
		setTestAdminAuthHeader(request)
		response := httptest.NewRecorder()
		mux.ServeHTTP(response, request)
		if response.Code != http.StatusOK {
			t.Fatalf("%s: status %d: %s", path, response.Code, response.Body.String())
		}
		if got := response.Header().Get("Cache-Control"); got != "no-store" {
			t.Fatalf("%s: Cache-Control = %q", path, got)
		}
	}
}
