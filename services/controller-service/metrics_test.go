package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"adplatform/internal/platform/httpapi"
	"adplatform/internal/services/apigateway"
)

type controllerMetricsAccessStub struct {
	status apigateway.ControllerAccessStatus
}

func (s controllerMetricsAccessStub) Status() apigateway.ControllerAccessStatus {
	return s.status
}

func (controllerMetricsAccessStub) Apply(_ context.Context, _ []apigateway.ControllerServiceAccessPolicy, _ time.Time) (apigateway.ControllerAccessStatus, error) {
	return apigateway.ControllerAccessStatus{}, nil
}

func (controllerMetricsAccessStub) Teardown(_ context.Context) error {
	return nil
}

func TestControllerServiceMetricsEndpoint(t *testing.T) {
	info := httpapi.ServiceInfo{Name: "controller-service-metrics-" + strings.ToLower(strings.ReplaceAll(t.Name(), "/", "-")), Version: "test", Addr: ":0"}
	server := &controllerServer{
		access: controllerMetricsAccessStub{status: apigateway.ControllerAccessStatus{
			State:             "applied",
			Mode:              "host",
			PoliciesTotal:     2,
			SSHOpenServices:   1,
			SSHLockedServices: 1,
			AllowedPeersTotal: 3,
			AppliedAt:         "2026-03-28T12:34:56Z",
		}},
		metrics: newControllerServiceMetrics(),
	}
	server.metrics.recordDeploymentReconcile(250*time.Millisecond, 4, false)
	server.metrics.recordAccessReconcile(controllerAccessScopeGlobal, 120*time.Millisecond, 2, false)
	server.metrics.recordAccessReconcile(controllerAccessScopeService, 80*time.Millisecond, 2, false)
	server.metrics.recordOperation(controllerOperationSSHCredential, 65*time.Millisecond, true)
	httpapi.RegisterMetricsSource(info.Name, server)

	mux := httpapi.NewBaseMux(info)

	request := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, request)

	body := response.Body.String()
	for _, fragment := range []string{
		"adplatform_controller_service_metrics_collection_success 1",
		`adplatform_controller_service_operation_requests_total{operation="deployment_reconcile"} 1`,
		`adplatform_controller_service_operation_requests_total{operation="ssh_credential"} 1`,
		`adplatform_controller_service_operation_failures_total{operation="ssh_credential"} 1`,
		"adplatform_controller_service_deployment_tasks_ensured_total 4",
		`adplatform_controller_service_access_policies_applied_total{scope="global"} 2`,
		`adplatform_controller_service_access_policies_applied_total{scope="service"} 2`,
		`adplatform_controller_service_access_state{state="applied"} 1`,
		"adplatform_controller_service_access_policies_total 2",
		"adplatform_controller_service_access_ssh_open_services 1",
		"adplatform_controller_service_access_allowed_peers_total 3",
	} {
		if !strings.Contains(body, fragment) {
			t.Fatalf("expected metrics output to contain %q, got:\n%s", fragment, body)
		}
	}
}
