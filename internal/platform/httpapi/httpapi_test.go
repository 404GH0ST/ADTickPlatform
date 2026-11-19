package httpapi

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type stubMetricsSource struct{}

func (stubMetricsSource) WritePrometheusMetrics(w io.Writer) {
	_, _ = io.WriteString(w, "# HELP adplatform_stub_metric Stub metric.\n")
	_, _ = io.WriteString(w, "# TYPE adplatform_stub_metric gauge\n")
	_, _ = io.WriteString(w, "adplatform_stub_metric 7\n")
}

func TestHTTPServerHasResourceTimeouts(t *testing.T) {
	server := newHTTPServer(ServiceInfo{Name: "timeouts", Addr: ":0"}, http.NewServeMux())
	if server.ReadHeaderTimeout != 5*time.Second || server.ReadTimeout != 15*time.Second || server.IdleTimeout != 120*time.Second {
		t.Fatalf("unexpected server timeouts: header=%s read=%s idle=%s", server.ReadHeaderTimeout, server.ReadTimeout, server.IdleTimeout)
	}
	if server.MaxHeaderBytes != 1<<20 {
		t.Fatalf("unexpected max header bytes: %d", server.MaxHeaderBytes)
	}
}

func TestRunServerShutdownAndListenFailure(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	mux := http.NewServeMux()
	if err := RunServer(ctx, ServiceInfo{Name: "shutdown-test", Addr: "127.0.0.1:0"}, mux); err != nil {
		t.Fatalf("shutdown: %v", err)
	}

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	ctx, cancel = context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := RunServer(ctx, ServiceInfo{Name: "listen-test", Addr: listener.Addr().String()}, mux); err == nil {
		t.Fatal("expected a listen error when the address is already bound")
	}
}

func TestBaseMuxJSONResponses(t *testing.T) {
	info := ServiceInfo{Name: "base-test", Version: "test", Addr: "127.0.0.1:0"}
	mux := NewBaseMux(info)
	for path, want := range map[string]any{
		"/healthz":  map[string]string{"status": "ok", "service": info.Name},
		"/readyz":   map[string]string{"status": "ready", "service": info.Name},
		"/metadata": info,
	} {
		recorder := httptest.NewRecorder()
		mux.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, path, nil))
		assertJSONResponse(t, recorder, http.StatusOK, "application/json", want)
	}
}

func TestWriteProblemPreservesFieldsAndSuppliesDefaults(t *testing.T) {
	for _, given := range []ProblemDetails{{}, {Title: "Invalid input", Status: 422, Detail: "Name is required"}} {
		recorder := httptest.NewRecorder()
		WriteProblem(recorder, 422, given)
		want := given
		if want.Title == "" {
			want = ProblemDetails{Title: http.StatusText(422), Status: 422}
		}
		assertJSONResponse(t, recorder, 422, "application/problem+json", want)
	}
}

func assertJSONResponse(t *testing.T, recorder *httptest.ResponseRecorder, status int, contentType string, want any) {
	t.Helper()
	if recorder.Code != status || recorder.Header().Get("Content-Type") != contentType {
		t.Fatalf("response status = %d, headers = %v", recorder.Code, recorder.Header())
	}
	encoded, err := json.Marshal(want)
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(recorder.Body.String()); got != string(encoded) {
		t.Fatalf("response = %s, want %s", got, encoded)
	}
}

func TestBearerToken(t *testing.T) {
	for _, tt := range []struct{ header, token string }{
		{"", ""}, {"Bearer", ""}, {"Basic example", ""}, {"  bEaReR  session-token  ", "session-token"},
	} {
		r := httptest.NewRequest(http.MethodGet, "/healthz", nil)
		r.Header.Set("Authorization", tt.header)
		if token, ok := BearerToken(r); token != tt.token || ok != (tt.token != "") {
			t.Errorf("BearerToken(%q) = %q, %t, want %q", tt.header, token, ok, tt.token)
		}
	}
}

func TestMetricsEndpointIncludesHTTPAndCustomMetrics(t *testing.T) {
	info := ServiceInfo{Name: "metrics-test-service", Version: "test", Addr: ":0"}
	RegisterMetricsSource(info.Name, stubMetricsSource{})

	mux := NewBaseMux(info)
	mux.HandleFunc("GET /demo/{id}", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte("created"))
	})
	mux.HandleFunc("POST /demo", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	handler := instrumentHandler(info, mux)

	request := httptest.NewRequest(http.MethodGet, "/demo/42", nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d", response.Code)
	}
	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/demo", nil))

	metricsRequest := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	metricsResponse := httptest.NewRecorder()
	handler.ServeHTTP(metricsResponse, metricsRequest)

	body := metricsResponse.Body.String()
	for _, fragment := range []string{
		`adplatform_service_info{service="metrics-test-service",version="test"} 1`,
		`adplatform_http_requests_total{service="metrics-test-service",method="GET",pattern="GET /demo/{id}",status_code="201"} 1`,
		`adplatform_http_requests_total{service="metrics-test-service",method="POST",pattern="POST /demo",status_code="204"} 1`,
		`adplatform_http_request_duration_seconds_count{service="metrics-test-service",method="GET",pattern="GET /demo/{id}",status_code="201"} 1`,
		`adplatform_stub_metric 7`,
	} {
		if !strings.Contains(body, fragment) {
			t.Fatalf("expected metrics output to contain %q, got:\n%s", fragment, body)
		}
	}
}

type hijackingRecorder struct {
	*httptest.ResponseRecorder
}

func (*hijackingRecorder) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	return nil, nil, net.ErrClosed
}

func TestMetricsWriterPreservesStreamingInterfaces(t *testing.T) {
	recorder := httptest.NewRecorder()
	writer := &statusCapturingResponseWriter{ResponseWriter: recorder}
	writer.Flush()
	if !recorder.Flushed {
		t.Fatal("expected Flush to reach response writer")
	}
	if _, _, err := writer.Hijack(); !errors.Is(err, http.ErrNotSupported) {
		t.Fatalf("Hijack = %v, want ErrNotSupported", err)
	}
	writer.ResponseWriter = &hijackingRecorder{recorder}
	if _, _, err := writer.Hijack(); !errors.Is(err, net.ErrClosed) {
		t.Fatalf("Hijack = %v, want the wrapped writer's error", err)
	}
}

func TestRequestMetricLabels(t *testing.T) {
	for _, tt := range []struct {
		method, pattern, wantMethod, wantPattern string
	}{
		{http.MethodGet, "GET /demo/{id}", "GET", "GET /demo/{id}"},
		{http.MethodGet, "", "GET", "unmatched"},
		{http.MethodConnect, "/demo/", "CONNECT", "unmatched"},
		{"CUSTOM", "", "OTHER", "unmatched"},
	} {
		r := &http.Request{Method: tt.method, Pattern: tt.pattern}
		if got := requestMetricMethod(r.Method); got != tt.wantMethod {
			t.Errorf("metric method for %q = %q, want %q", tt.method, got, tt.wantMethod)
		}
		if got := requestMetricPattern(r); got != tt.wantPattern {
			t.Errorf("metric pattern for %q %q = %q, want %q", tt.method, tt.pattern, got, tt.wantPattern)
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

func TestDecodeJSONValidatesPayload(t *testing.T) {
	for _, tt := range []struct {
		body string
		ok   bool
	}{
		{`{"name":"alpha"}`, true}, {`{"unknown":true}`, false}, {`{"name":`, false},
	} {
		var target struct{ Name string }
		r := httptest.NewRequest(http.MethodPost, "/demo", strings.NewReader(tt.body))
		if err := DecodeJSON(r, &target); (err == nil) != tt.ok || (tt.ok && target.Name != "alpha") {
			t.Errorf("DecodeJSON(%s) = %+v, %v", tt.body, target, err)
		}
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
		{name: "empty", raw: " ", ok: false},
		{name: "invalid URL", raw: "http://%invalid", ok: false},
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

func TestBearerTokenRejectsBlankToken(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.Header.Set("Authorization", "Bearer    ")
	if token, ok := BearerToken(request); ok || token != "" {
		t.Fatalf("a blank bearer token must be rejected, got %q", token)
	}
}

func TestRegisterMetricsSourceIgnoresBlankNameAndNilSource(t *testing.T) {
	registered := func(name string) bool {
		metricsRegistry.mu.Lock()
		defer metricsRegistry.mu.Unlock()
		_, ok := metricsRegistry.services[name]
		return ok
	}
	noop := stubMetricsSource{}
	RegisterMetricsSource("  ", noop)
	if registered("") {
		t.Error("a blank service name must not create a registry entry")
	}
	RegisterMetricsSource("svc-nil-source", nil)
	if registered("svc-nil-source") {
		t.Error("a nil source must not create a registry entry")
	}
}

func TestMetricsSortByPatternThenStatus(t *testing.T) {
	info := ServiceInfo{Name: "sort-probe", Version: "test", Addr: ":0"}
	mux := NewBaseMux(info)
	mux.HandleFunc("GET /zeta", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	mux.HandleFunc("GET /alpha/{id}", func(w http.ResponseWriter, r *http.Request) {
		if r.PathValue("id") == "missing" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusOK)
	})
	handler := newHTTPServer(info, mux).Handler
	for _, path := range []string{"/zeta", "/alpha/missing", "/alpha/1"} {
		handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, path, nil))
	}
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	body := recorder.Body.String()
	alphaOK := strings.Index(body, `pattern="GET /alpha/{id}",status_code="200"`)
	alphaMissing := strings.Index(body, `pattern="GET /alpha/{id}",status_code="404"`)
	zeta := strings.Index(body, `pattern="GET /zeta"`)
	if alphaOK < 0 || alphaMissing < 0 || zeta < 0 || !(alphaOK < alphaMissing && alphaMissing < zeta) {
		t.Fatalf("metrics must sort by pattern, then status:\n%s", body)
	}
}
