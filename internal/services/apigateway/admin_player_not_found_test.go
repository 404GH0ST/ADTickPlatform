package apigateway

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// An unknown player id is a client error, like an unknown team or challenge,
// not an internal outage that would page operators.
func TestAdminPlayerActionsReportUnknownPlayerAsBadRequest(t *testing.T) {
	mux := newTestMux()
	for _, tc := range []struct{ method, path string }{
		{http.MethodPost, "/api/v2/admin/players/9999/deactivate"},
		{http.MethodPost, "/api/v2/admin/players/9999/reactivate"},
		{http.MethodDelete, "/api/v2/admin/players/9999"},
	} {
		request := httptest.NewRequest(tc.method, tc.path, nil)
		setTestAdminAuthHeader(request)
		response := httptest.NewRecorder()
		mux.ServeHTTP(response, request)
		if response.Code != http.StatusBadRequest {
			t.Errorf("%s %s: want 400 for an unknown player, got %d: %s", tc.method, tc.path, response.Code, response.Body.String())
		}
	}
}
