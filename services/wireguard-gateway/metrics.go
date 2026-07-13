package main

import (
	"fmt"
	"io"
	"sync"
	"time"
)

const (
	wireGuardOperationStatus    = "status"
	wireGuardOperationReconcile = "reconcile"
	wireGuardOperationTeardown  = "teardown"
)

var wireGuardMetricOperations = []string{
	wireGuardOperationStatus,
	wireGuardOperationReconcile,
	wireGuardOperationTeardown,
}

type wireGuardOperationAggregate struct {
	requestsTotal        uint64
	failuresTotal        uint64
	durationSecondsSum   float64
	durationSecondsCount uint64
}

type wireGuardGatewayMetrics struct {
	mu sync.Mutex

	operations           map[string]wireGuardOperationAggregate
	reconciledPeersTotal uint64
}

func newWireGuardGatewayMetrics() *wireGuardGatewayMetrics {
	metrics := &wireGuardGatewayMetrics{
		operations: make(map[string]wireGuardOperationAggregate, len(wireGuardMetricOperations)),
	}
	for _, operation := range wireGuardMetricOperations {
		metrics.operations[operation] = wireGuardOperationAggregate{}
	}
	return metrics
}

func (m *wireGuardGatewayMetrics) recordReconcile(duration time.Duration, peers int, failed bool) {
	m.recordOperation(wireGuardOperationReconcile, duration, failed)
	if peers <= 0 {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.reconciledPeersTotal += uint64(peers)
}

func (m *wireGuardGatewayMetrics) recordOperation(operation string, duration time.Duration, failed bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.operations == nil {
		m.operations = make(map[string]wireGuardOperationAggregate, len(wireGuardMetricOperations))
	}
	entry := m.operations[operation]
	entry.requestsTotal++
	if failed {
		entry.failuresTotal++
	}
	entry.durationSecondsSum += duration.Seconds()
	entry.durationSecondsCount++
	m.operations[operation] = entry
}

func (s *wireGuardGatewayServer) WritePrometheusMetrics(w io.Writer) {
	status := s.status()

	s.metrics.mu.Lock()
	if s.metrics.operations == nil {
		s.metrics.operations = make(map[string]wireGuardOperationAggregate, len(wireGuardMetricOperations))
	}
	operations := make(map[string]wireGuardOperationAggregate, len(s.metrics.operations))
	for _, operation := range wireGuardMetricOperations {
		operations[operation] = s.metrics.operations[operation]
	}
	reconciledPeersTotal := s.metrics.reconciledPeersTotal
	s.metrics.mu.Unlock()

	fmt.Fprintln(w, "# HELP adplatform_wireguard_gateway_metrics_collection_success Whether wireguard-gateway metrics collection succeeded.")
	fmt.Fprintln(w, "# TYPE adplatform_wireguard_gateway_metrics_collection_success gauge")
	fmt.Fprintln(w, "adplatform_wireguard_gateway_metrics_collection_success 1")

	fmt.Fprintln(w, "# HELP adplatform_wireguard_gateway_operation_requests_total Total wireguard-gateway requests grouped by operation.")
	fmt.Fprintln(w, "# TYPE adplatform_wireguard_gateway_operation_requests_total counter")
	for _, operation := range wireGuardMetricOperations {
		fmt.Fprintf(w, "adplatform_wireguard_gateway_operation_requests_total{operation=%q} %d\n", operation, operations[operation].requestsTotal)
	}

	fmt.Fprintln(w, "# HELP adplatform_wireguard_gateway_operation_failures_total Total failed wireguard-gateway requests grouped by operation.")
	fmt.Fprintln(w, "# TYPE adplatform_wireguard_gateway_operation_failures_total counter")
	for _, operation := range wireGuardMetricOperations {
		fmt.Fprintf(w, "adplatform_wireguard_gateway_operation_failures_total{operation=%q} %d\n", operation, operations[operation].failuresTotal)
	}

	fmt.Fprintln(w, "# HELP adplatform_wireguard_gateway_operation_duration_seconds_sum Total wireguard-gateway operation duration in seconds.")
	fmt.Fprintln(w, "# TYPE adplatform_wireguard_gateway_operation_duration_seconds_sum counter")
	for _, operation := range wireGuardMetricOperations {
		fmt.Fprintf(w, "adplatform_wireguard_gateway_operation_duration_seconds_sum{operation=%q} %.6f\n", operation, operations[operation].durationSecondsSum)
	}

	fmt.Fprintln(w, "# HELP adplatform_wireguard_gateway_operation_duration_seconds_count Total observed wireguard-gateway operations for duration accounting.")
	fmt.Fprintln(w, "# TYPE adplatform_wireguard_gateway_operation_duration_seconds_count counter")
	for _, operation := range wireGuardMetricOperations {
		fmt.Fprintf(w, "adplatform_wireguard_gateway_operation_duration_seconds_count{operation=%q} %d\n", operation, operations[operation].durationSecondsCount)
	}

	fmt.Fprintln(w, "# HELP adplatform_wireguard_gateway_reconciled_peers_total Total peers included in reconcile attempts.")
	fmt.Fprintln(w, "# TYPE adplatform_wireguard_gateway_reconciled_peers_total counter")
	fmt.Fprintf(w, "adplatform_wireguard_gateway_reconciled_peers_total %d\n", reconciledPeersTotal)

	fmt.Fprintln(w, "# HELP adplatform_wireguard_gateway_peer_counts Current peer counts grouped by status.")
	fmt.Fprintln(w, "# TYPE adplatform_wireguard_gateway_peer_counts gauge")
	fmt.Fprintf(w, "adplatform_wireguard_gateway_peer_counts{status=%q} %d\n", "total", status.PeersTotal)
	fmt.Fprintf(w, "adplatform_wireguard_gateway_peer_counts{status=%q} %d\n", "active", status.PeersActive)
	fmt.Fprintf(w, "adplatform_wireguard_gateway_peer_counts{status=%q} %d\n", "revoked", status.PeersRevoked)

	fmt.Fprintln(w, "# HELP adplatform_wireguard_gateway_state Whether the wireguard gateway currently reports the given state.")
	fmt.Fprintln(w, "# TYPE adplatform_wireguard_gateway_state gauge")
	for _, state := range []string{"idle", "applied", "error"} {
		fmt.Fprintf(w, "adplatform_wireguard_gateway_state{state=%q} %.0f\n", state, metricBool(status.State == state))
	}

	fmt.Fprintln(w, "# HELP adplatform_wireguard_gateway_last_apply_success Whether the wireguard gateway last reported a successful apply.")
	fmt.Fprintln(w, "# TYPE adplatform_wireguard_gateway_last_apply_success gauge")
	fmt.Fprintf(w, "adplatform_wireguard_gateway_last_apply_success %.0f\n", metricBool(status.State == "applied" && status.LastError == ""))

	fmt.Fprintln(w, "# HELP adplatform_wireguard_gateway_last_error Whether the wireguard gateway currently reports a last_error value.")
	fmt.Fprintln(w, "# TYPE adplatform_wireguard_gateway_last_error gauge")
	fmt.Fprintf(w, "adplatform_wireguard_gateway_last_error %.0f\n", metricBool(status.LastError != ""))

	if appliedAt := metricParseTime(status.AppliedAt); !appliedAt.IsZero() {
		fmt.Fprintln(w, "# HELP adplatform_wireguard_gateway_applied_at_unixtime Last gateway apply time as a Unix timestamp.")
		fmt.Fprintln(w, "# TYPE adplatform_wireguard_gateway_applied_at_unixtime gauge")
		fmt.Fprintf(w, "adplatform_wireguard_gateway_applied_at_unixtime %.0f\n", float64(appliedAt.Unix()))
	}
}

var _ interface{ WritePrometheusMetrics(io.Writer) } = (*wireGuardGatewayServer)(nil)

func metricBool(value bool) float64 {
	if value {
		return 1
	}
	return 0
}

func metricParseTime(value string) time.Time {
	if value == "" {
		return time.Time{}
	}
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return time.Time{}
	}
	return parsed.UTC()
}
