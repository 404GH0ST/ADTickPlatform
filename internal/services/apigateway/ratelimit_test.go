package apigateway

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

type unavailableRateLimiter struct{}

func (unavailableRateLimiter) Allow(context.Context, string, rateLimitPolicy) (rateLimitDecision, error) {
	return rateLimitDecision{}, errors.New("redis unavailable")
}

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

func TestPublicReadRateLimitFailsOpenWhenBackendIsUnavailable(t *testing.T) {
	server := &Server{
		rateLimiter:      unavailableRateLimiter{},
		rateLimitMetrics: newRateLimitMetrics(),
	}
	if _, allowed := server.allowRateLimit(context.Background(), "challenges:client:198.51.100.1", challengesRateLimitPolicy); !allowed {
		t.Fatal("expected public challenge reads to remain available during a rate-limit backend outage")
	}
	if _, allowed := server.allowRateLimit(context.Background(), "auth:client:198.51.100.1", authClientRateLimitPolicy); allowed {
		t.Fatal("expected authentication to remain fail-closed during a rate-limit backend outage")
	}
	if _, allowed := server.allowRateLimit(context.Background(), "register:client:198.51.100.1", registrationIPRateLimitPolicy); allowed {
		t.Fatal("expected registration IP limiting to remain fail-closed during a backend outage")
	}
	if _, allowed := server.allowRateLimit(context.Background(), "register:email:user@example.com", registrationEmailRateLimitPolicy); allowed {
		t.Fatal("expected registration email limiting to remain fail-closed during a backend outage")
	}
}
