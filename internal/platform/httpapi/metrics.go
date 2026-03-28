package httpapi

import (
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"
)

type MetricsSource interface {
	WritePrometheusMetrics(w io.Writer)
}

type requestMetricKey struct {
	Method     string
	Pattern    string
	StatusCode int
}

type requestMetricValue struct {
	Count          uint64
	DurationSumSec float64
}

type serviceMetrics struct {
	name      string
	version   string
	startedAt time.Time

	mu       sync.Mutex
	inFlight int
	requests map[requestMetricKey]requestMetricValue
	sources  []MetricsSource
}

var metricsRegistry = struct {
	mu       sync.Mutex
	services map[string]*serviceMetrics
}{
	services: make(map[string]*serviceMetrics),
}

func lookupServiceMetrics(info ServiceInfo) *serviceMetrics {
	metricsRegistry.mu.Lock()
	defer metricsRegistry.mu.Unlock()

	if metrics, ok := metricsRegistry.services[info.Name]; ok {
		if info.Version != "" {
			metrics.version = info.Version
		}
		return metrics
	}

	metrics := &serviceMetrics{
		name:      info.Name,
		version:   info.Version,
		startedAt: time.Now().UTC(),
		requests:  make(map[requestMetricKey]requestMetricValue),
	}
	metricsRegistry.services[info.Name] = metrics
	return metrics
}

func RegisterMetricsSource(serviceName string, source MetricsSource) {
	serviceName = strings.TrimSpace(serviceName)
	if serviceName == "" || source == nil {
		return
	}
	metrics := lookupServiceMetrics(ServiceInfo{Name: serviceName})
	metrics.mu.Lock()
	defer metrics.mu.Unlock()
	metrics.sources = append(metrics.sources, source)
}

func instrumentHandler(info ServiceInfo, next http.Handler) http.Handler {
	metrics := lookupServiceMetrics(info)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/metrics" {
			next.ServeHTTP(w, r)
			return
		}

		started := time.Now()
		metrics.mu.Lock()
		metrics.inFlight++
		metrics.mu.Unlock()

		recorder := &statusCapturingResponseWriter{ResponseWriter: w, statusCode: http.StatusOK}
		defer func() {
			durationSec := time.Since(started).Seconds()
			pattern := strings.TrimSpace(r.Pattern)
			if pattern == "" {
				pattern = r.URL.Path
			}

			metrics.mu.Lock()
			metrics.inFlight--
			key := requestMetricKey{
				Method:     r.Method,
				Pattern:    pattern,
				StatusCode: recorder.statusCode,
			}
			value := metrics.requests[key]
			value.Count++
			value.DurationSumSec += durationSec
			metrics.requests[key] = value
			metrics.mu.Unlock()
		}()

		next.ServeHTTP(recorder, r)
	})
}

type statusCapturingResponseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (w *statusCapturingResponseWriter) WriteHeader(statusCode int) {
	w.statusCode = statusCode
	w.ResponseWriter.WriteHeader(statusCode)
}

func (m *serviceMetrics) writePrometheus(w io.Writer) {
	type requestSample struct {
		Method         string
		Pattern        string
		StatusCode     int
		Count          uint64
		DurationSumSec float64
	}

	m.mu.Lock()
	inFlight := m.inFlight
	uptime := time.Since(m.startedAt).Seconds()
	requests := make([]requestSample, 0, len(m.requests))
	for key, value := range m.requests {
		requests = append(requests, requestSample{
			Method:         key.Method,
			Pattern:        key.Pattern,
			StatusCode:     key.StatusCode,
			Count:          value.Count,
			DurationSumSec: value.DurationSumSec,
		})
	}
	sources := append([]MetricsSource(nil), m.sources...)
	version := m.version
	serviceName := m.name
	m.mu.Unlock()

	sort.Slice(requests, func(i, j int) bool {
		if requests[i].Method != requests[j].Method {
			return requests[i].Method < requests[j].Method
		}
		if requests[i].Pattern != requests[j].Pattern {
			return requests[i].Pattern < requests[j].Pattern
		}
		return requests[i].StatusCode < requests[j].StatusCode
	})

	fmt.Fprintln(w, "# HELP adplatform_service_info Static service metadata.")
	fmt.Fprintln(w, "# TYPE adplatform_service_info gauge")
	fmt.Fprintf(w, "adplatform_service_info{service=%q,version=%q} 1\n", prometheusLabelValue(serviceName), prometheusLabelValue(version))

	fmt.Fprintln(w, "# HELP adplatform_service_uptime_seconds Service uptime in seconds.")
	fmt.Fprintln(w, "# TYPE adplatform_service_uptime_seconds gauge")
	fmt.Fprintf(w, "adplatform_service_uptime_seconds{service=%q} %.6f\n", prometheusLabelValue(serviceName), uptime)

	fmt.Fprintln(w, "# HELP adplatform_http_in_flight_requests Current in-flight HTTP requests.")
	fmt.Fprintln(w, "# TYPE adplatform_http_in_flight_requests gauge")
	fmt.Fprintf(w, "adplatform_http_in_flight_requests{service=%q} %d\n", prometheusLabelValue(serviceName), inFlight)

	fmt.Fprintln(w, "# HELP adplatform_http_requests_total Total HTTP requests handled by the service.")
	fmt.Fprintln(w, "# TYPE adplatform_http_requests_total counter")
	for _, request := range requests {
		fmt.Fprintf(
			w,
			"adplatform_http_requests_total{service=%q,method=%q,pattern=%q,status_code=%q} %d\n",
			prometheusLabelValue(serviceName),
			prometheusLabelValue(request.Method),
			prometheusLabelValue(request.Pattern),
			prometheusLabelValue(fmt.Sprintf("%d", request.StatusCode)),
			request.Count,
		)
	}

	fmt.Fprintln(w, "# HELP adplatform_http_request_duration_seconds_sum Total HTTP request duration in seconds.")
	fmt.Fprintln(w, "# TYPE adplatform_http_request_duration_seconds_sum counter")
	for _, request := range requests {
		fmt.Fprintf(
			w,
			"adplatform_http_request_duration_seconds_sum{service=%q,method=%q,pattern=%q,status_code=%q} %.6f\n",
			prometheusLabelValue(serviceName),
			prometheusLabelValue(request.Method),
			prometheusLabelValue(request.Pattern),
			prometheusLabelValue(fmt.Sprintf("%d", request.StatusCode)),
			request.DurationSumSec,
		)
	}

	fmt.Fprintln(w, "# HELP adplatform_http_request_duration_seconds_count Total observed HTTP requests for duration accounting.")
	fmt.Fprintln(w, "# TYPE adplatform_http_request_duration_seconds_count counter")
	for _, request := range requests {
		fmt.Fprintf(
			w,
			"adplatform_http_request_duration_seconds_count{service=%q,method=%q,pattern=%q,status_code=%q} %d\n",
			prometheusLabelValue(serviceName),
			prometheusLabelValue(request.Method),
			prometheusLabelValue(request.Pattern),
			prometheusLabelValue(fmt.Sprintf("%d", request.StatusCode)),
			request.Count,
		)
	}

	for _, source := range sources {
		source.WritePrometheusMetrics(w)
	}
}

func prometheusLabelValue(value string) string {
	value = strings.ReplaceAll(value, `\`, `\\`)
	value = strings.ReplaceAll(value, "\n", `\n`)
	value = strings.ReplaceAll(value, `"`, `\"`)
	return value
}
