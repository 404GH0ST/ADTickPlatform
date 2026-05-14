package apigateway

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClientRateLimitKeyIgnoresForwardedForByDefault(t *testing.T) {
	t.Setenv("API_GATEWAY_TRUST_PROXY_HEADERS", "")
	request := httptest.NewRequest(http.MethodGet, "/api/v2/challenges", nil)
	request.RemoteAddr = "198.51.100.20:44123"
	request.Header.Set("X-Forwarded-For", "203.0.113.9")

	if got := clientRateLimitKey(request); got != "198.51.100.20" {
		t.Fatalf("expected remote address rate-limit key, got %q", got)
	}
}

func TestClientRateLimitKeyUsesForwardedForWhenTrusted(t *testing.T) {
	t.Setenv("API_GATEWAY_TRUST_PROXY_HEADERS", "true")
	request := httptest.NewRequest(http.MethodGet, "/api/v2/challenges", nil)
	request.RemoteAddr = "198.51.100.20:44123"
	request.Header.Set("X-Forwarded-For", "203.0.113.9, 198.51.100.20")

	if got := clientRateLimitKey(request); got != "203.0.113.9" {
		t.Fatalf("expected forwarded rate-limit key, got %q", got)
	}
}
