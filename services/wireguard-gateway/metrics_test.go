package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"adplatform/internal/platform/httpapi"
	"adplatform/internal/services/apigateway"
)

func TestWireGuardGatewayMetricsEndpoint(t *testing.T) {
	info := httpapi.ServiceInfo{Name: "wireguard-gateway-metrics-" + strings.ToLower(strings.ReplaceAll(t.Name(), "/", "-")), Version: "test", Addr: ":0"}
	server := newWireGuardGatewayServer("admin-token", nil, dryRunWireGuardApplier{})
	server.rememberStatus(apigateway.WireGuardGatewayStatus{
		State:        "applied",
		Mode:         "host",
		PeersTotal:   4,
		PeersActive:  3,
		PeersRevoked: 1,
		AppliedAt:    "2026-03-28T04:00:00Z",
	})
	server.metrics.recordReconcile(150000000, 4, false)
	server.metrics.recordOperation(wireGuardOperationStatus, 5000000, false)
	server.metrics.recordOperation(wireGuardOperationTeardown, 7000000, true)
	httpapi.RegisterMetricsSource(info.Name, server)

	mux := httpapi.NewBaseMux(info)

	request := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, request)

	body := response.Body.String()
	for _, fragment := range []string{
		"adplatform_wireguard_gateway_metrics_collection_success 1",
		`adplatform_wireguard_gateway_operation_requests_total{operation="reconcile"} 1`,
		`adplatform_wireguard_gateway_operation_requests_total{operation="status"} 1`,
		`adplatform_wireguard_gateway_operation_failures_total{operation="teardown"} 1`,
		"adplatform_wireguard_gateway_reconciled_peers_total 4",
		`adplatform_wireguard_gateway_peer_counts{status="active"} 3`,
		`adplatform_wireguard_gateway_state{state="applied"} 1`,
		"adplatform_wireguard_gateway_last_apply_success 1",
	} {
		if !strings.Contains(body, fragment) {
			t.Fatalf("expected metrics output to contain %q, got:\n%s", fragment, body)
		}
	}
}
