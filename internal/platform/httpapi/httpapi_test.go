package httpapi

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type stubMetricsSource struct{}

func (stubMetricsSource) WritePrometheusMetrics(w io.Writer) {
	_, _ = io.WriteString(w, "# HELP adplatform_stub_metric Stub metric.\n")
	_, _ = io.WriteString(w, "# TYPE adplatform_stub_metric gauge\n")
	_, _ = io.WriteString(w, "adplatform_stub_metric 7\n")
}

func TestMetricsEndpointIncludesHTTPAndCustomMetrics(t *testing.T) {
	info := ServiceInfo{Name: "metrics-test-service", Version: "test", Addr: ":0"}
	RegisterMetricsSource(info.Name, stubMetricsSource{})

	mux := NewBaseMux(info)
	mux.HandleFunc("GET /demo/{id}", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte("created"))
	})

	handler := instrumentHandler(info, mux)

	request := httptest.NewRequest(http.MethodGet, "/demo/42", nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d", response.Code)
	}

	metricsRequest := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	metricsResponse := httptest.NewRecorder()
	handler.ServeHTTP(metricsResponse, metricsRequest)

	body := metricsResponse.Body.String()
	for _, fragment := range []string{
		`adplatform_service_info{service="metrics-test-service",version="test"} 1`,
		`adplatform_http_requests_total{service="metrics-test-service",method="GET",pattern="GET /demo/{id}",status_code="201"} 1`,
		`adplatform_http_request_duration_seconds_count{service="metrics-test-service",method="GET",pattern="GET /demo/{id}",status_code="201"} 1`,
		`adplatform_stub_metric 7`,
	} {
		if !strings.Contains(body, fragment) {
			t.Fatalf("expected metrics output to contain %q, got:\n%s", fragment, body)
		}
	}
}

func TestDecodeJSONRejectsTrailingJSONValue(t *testing.T) {
	request := httptest.NewRequest(
		http.MethodPost,
		"/demo",
		strings.NewReader(`{"name":"alpha"}{"name":"beta"}`),
	)

	var payload struct {
		Name string `json:"name"`
	}
	err := DecodeJSON(request, &payload)

	if err == nil || !strings.Contains(err.Error(), "unexpected trailing json") {
		t.Fatalf("expected trailing json error, got %v", err)
	}
}

func TestNormalizeInternalBaseURL(t *testing.T) {
	testCases := []struct {
		name string
		raw  string
		want string
		ok   bool
	}{
		{name: "http origin", raw: " http://game-core:8080/ ", want: "http://game-core:8080", ok: true},
		{name: "https path", raw: "https://api.internal/v1", want: "https://api.internal/v1", ok: true},
		{name: "missing scheme", raw: "game-core:8080", ok: false},
		{name: "unsupported scheme", raw: "file:///etc/passwd", ok: false},
		{name: "userinfo", raw: "http://user:pass@game-core:8080", ok: false},
		{name: "query", raw: "http://game-core:8080?next=http://metadata", ok: false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := NormalizeInternalBaseURL(tc.raw)
			if ok != tc.ok || got != tc.want {
				t.Fatalf("NormalizeInternalBaseURL(%q) = %q, %v; want %q, %v", tc.raw, got, ok, tc.want, tc.ok)
			}
		})
	}
}
