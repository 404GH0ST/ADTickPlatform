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
