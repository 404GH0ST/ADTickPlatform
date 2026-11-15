package apigateway

import (
	"context"
	"errors"
	"math"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"
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

func TestRefillRateKeepsSlowPoliciesExact(t *testing.T) {
	for _, policy := range []rateLimitPolicy{registrationEmailRateLimitPolicy, passwordChangeRateLimitPolicy, authRateLimitPolicy, submitRateLimitPolicy} {
		perMillisecond, err := strconv.ParseFloat(formatRefillPerMillisecond(policy.refillPerSecond), 64)
		if err != nil {
			t.Fatal(err)
		}
		if got := perMillisecond * 1000; math.Abs(got-policy.refillPerSecond) > policy.refillPerSecond*1e-12 {
			t.Fatalf("refill %.9g/s sent to redis as %.9g/s", policy.refillPerSecond, got)
		}
	}
}

func TestServiceLifecycleCooldownsAllowOneActionPerWindow(t *testing.T) {
	for name, tc := range map[string]struct {
		policy rateLimitPolicy
		window time.Duration
	}{
		"factory reset": {factoryResetRateLimitPolicy, 90 * time.Second},
		"restart":       {restartRateLimitPolicy, 30 * time.Second},
	} {
		if tc.policy.capacity != 1 {
			t.Fatalf("%s: expected single-action cooldown, got capacity %d", name, tc.policy.capacity)
		}
		if got := time.Duration(float64(time.Second) / tc.policy.refillPerSecond); got != tc.window {
			t.Fatalf("%s: expected %s cooldown, got %s", name, tc.window, got)
		}
		if !tc.policy.failClosedOnError {
			t.Fatalf("%s: cooldown must fail closed", name)
		}
	}
}

func TestCooldownFailureReportsRemainingSeconds(t *testing.T) {
	response := httptest.NewRecorder()
	writeCooldownFailure(response, rateLimitDecision{retryAfter: 41200 * time.Millisecond}, "Factory reset")
	if response.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429, got %d", response.Code)
	}
	if got := response.Header().Get("Retry-After"); got != "42" {
		t.Fatalf("expected Retry-After 42, got %q", got)
	}
	if !strings.Contains(response.Body.String(), "Factory reset is on cooldown for this service. Wait 42 seconds before trying again.") {
		t.Fatalf("unexpected body %s", response.Body.String())
	}

	response = httptest.NewRecorder()
	writeCooldownFailure(response, rateLimitDecision{retryAfter: 300 * time.Millisecond}, "Restart")
	if !strings.Contains(response.Body.String(), "Wait 1 second before") {
		t.Fatalf("expected singular second, got %s", response.Body.String())
	}

	response = httptest.NewRecorder()
	writeCooldownFailure(response, rateLimitDecision{retryAfter: 5 * time.Second, backendUnavailable: true}, "Restart")
	if strings.Contains(response.Body.String(), "cooldown") {
		t.Fatalf("limiter outage must not be reported as a cooldown: %s", response.Body.String())
	}
}
