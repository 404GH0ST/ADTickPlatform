package apigateway

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func decodeAppearance(t *testing.T, rec *httptest.ResponseRecorder) platformAppearance {
	t.Helper()
	var appearance platformAppearance
	if err := json.Unmarshal(rec.Body.Bytes(), &appearance); err != nil {
		t.Fatalf("decode appearance: %v (body %s)", err, rec.Body.String())
	}
	return appearance
}

func TestPlatformAppearanceDefaultsAndIsPublic(t *testing.T) {
	mux := newTestMux()
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v2/platform/appearance", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 without auth, got %d: %s", rec.Code, rec.Body.String())
	}
	if got := decodeAppearance(t, rec).ThemeID; got != defaultPlatformThemeID {
		t.Fatalf("expected default theme %q, got %q", defaultPlatformThemeID, got)
	}
}

func TestAdminUpdatePlatformAppearance(t *testing.T) {
	mux := newTestMux()

	unauth := httptest.NewRecorder()
	mux.ServeHTTP(unauth, httptest.NewRequest(http.MethodPut, "/api/v2/admin/platform/appearance", strings.NewReader(`{"theme_id":"ink"}`)))
	if unauth.Code != http.StatusUnauthorized && unauth.Code != http.StatusForbidden {
		t.Fatalf("expected auth failure, got %d", unauth.Code)
	}

	for _, bad := range []string{`{"theme_id":""}`, `{"theme_id":"Ink"}`, `{"theme_id":"../x"}`, `{"theme_id":"` + strings.Repeat("a", 49) + `"}`} {
		req := httptest.NewRequest(http.MethodPut, "/api/v2/admin/platform/appearance", strings.NewReader(bad))
		req.Header.Set("Authorization", "Bearer dev-admin-token")
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("payload %s: expected 400, got %d", bad, rec.Code)
		}
	}

	req := httptest.NewRequest(http.MethodPut, "/api/v2/admin/platform/appearance", strings.NewReader(`{"theme_id":"neon-ops"}`))
	req.Header.Set("Authorization", "Bearer dev-admin-token")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if got := decodeAppearance(t, rec).ThemeID; got != "neon-ops" {
		t.Fatalf("expected neon-ops, got %q", got)
	}

	read := httptest.NewRecorder()
	mux.ServeHTTP(read, httptest.NewRequest(http.MethodGet, "/api/v2/platform/appearance", nil))
	if got := decodeAppearance(t, read).ThemeID; got != "neon-ops" {
		t.Fatalf("public read expected neon-ops, got %q", got)
	}
}
