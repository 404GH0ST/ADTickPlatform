package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"adplatform/internal/platform/httpapi"
	"adplatform/internal/services/apigateway"
)

func newControllerReconcileTestMux(server *controllerServer) *http.ServeMux {
	mux := httpapi.NewBaseMux(httpapi.ServiceInfo{Name: "controller-service-test", Version: "test", Addr: ":0"})
	server.RegisterRoutes(mux)
	return mux
}

func TestControllerInternalRoutesRequireAdminAuth(t *testing.T) {
	server := &controllerServer{
		adminToken: "dev-admin-token",
		store:      apigateway.NewMemoryStore(101),
		executor:   &runtimeExecutorStub{},
		access:     &serviceAccessExecutorStub{},
		wireGuard:  &controllerWireGuardReconcilerStub{},
		metrics:    newControllerServiceMetrics(),
		now:        time.Now,
	}
	mux := newControllerReconcileTestMux(server)
	cases := []struct {
		method string
		path   string
		body   string
	}{
		{http.MethodGet, "/internal/v1/deployments", ""},
		{http.MethodPost, "/internal/v1/deployments/reconcile", `{}`},
		{http.MethodPost, "/internal/v1/challenges/validate", `{}`},
		{http.MethodGet, "/internal/v1/access/status", ""},
		{http.MethodPost, "/internal/v1/access/reconcile", `{}`},
		{http.MethodPost, "/internal/v1/access/teardown", `{}`},
		{http.MethodPost, "/internal/v1/teams/101/services/1/access/reconcile", `{}`},
		{http.MethodPost, "/internal/v1/teams/101/services/1/ssh-credential", `{}`},
		{http.MethodPost, "/internal/v1/teams/101/services/1/reset/factory", `{}`},
		{http.MethodPost, "/internal/v1/teams/101/services/1/restart", `{}`},
		{http.MethodPost, "/internal/v1/teams/101/services/1/remove", `{}`},
		{http.MethodPost, "/internal/v1/teams/101/remove", `{}`},
		{http.MethodPost, "/internal/v1/challenges/1/remove", `{}`},
	}

	for _, tc := range cases {
		t.Run(tc.method+" "+tc.path+" unauthenticated", func(t *testing.T) {
			request := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
			response := httptest.NewRecorder()

			mux.ServeHTTP(response, request)

			if response.Code != http.StatusForbidden {
				t.Fatalf("expected 403, got %d: %s", response.Code, response.Body.String())
			}
		})

		t.Run(tc.method+" "+tc.path+" wrong token", func(t *testing.T) {
			request := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
			request.Header.Set("Authorization", "Bearer wrong-token")
			response := httptest.NewRecorder()

			mux.ServeHTTP(response, request)

			if response.Code != http.StatusForbidden {
				t.Fatalf("expected 403, got %d: %s", response.Code, response.Body.String())
			}
		})
	}
}

func TestControllerReconcileDeploymentsReturnsTrustedSuccess(t *testing.T) {
	server := &controllerServer{
		adminToken: "dev-admin-token",
		store:      apigateway.NewMemoryStore(101),
		executor:   &runtimeExecutorStub{},
		access:     &serviceAccessExecutorStub{},
		wireGuard: &controllerWireGuardReconcilerStub{
			status: apigateway.WireGuardGatewayStatus{State: "applied", Mode: "host"},
		},
		metrics: newControllerServiceMetrics(),
		now:     func() time.Time { return time.Date(2026, time.March, 11, 9, 0, 0, 0, time.UTC) },
	}
	mux := newControllerReconcileTestMux(server)

	request := httptest.NewRequest(http.MethodPost, "/internal/v1/deployments/reconcile", nil)
	request.Header.Set("Authorization", "Bearer dev-admin-token")
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected reconcile 200, got %d", response.Code)
	}

	var payload controllerDeploymentReconcileState
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
}

func TestControllerReconcileDeploymentsReturnsBadGatewayOnAccessFailure(t *testing.T) {
	server := &controllerServer{
		adminToken: "dev-admin-token",
		store:      apigateway.NewMemoryStore(101),
		executor:   &runtimeExecutorStub{},
		access:     &serviceAccessExecutorStub{err: errors.New("nft apply failed")},
		wireGuard:  &controllerWireGuardReconcilerStub{},
		metrics:    newControllerServiceMetrics(),
		now:        time.Now,
	}
	mux := newControllerReconcileTestMux(server)

	request := httptest.NewRequest(http.MethodPost, "/internal/v1/deployments/reconcile", nil)
	request.Header.Set("Authorization", "Bearer dev-admin-token")
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, request)

	if response.Code != http.StatusBadGateway {
		t.Fatalf("expected reconcile 502, got %d", response.Code)
	}

	var problem httpapi.ProblemDetails
	if err := json.Unmarshal(response.Body.Bytes(), &problem); err != nil {
		t.Fatalf("decode problem: %v", err)
	}
	if problem.Title != "Deployment reconcile failed" || !strings.Contains(problem.Detail, controllerAccessTruthUnknownDetail) {
		t.Fatalf("unexpected problem %+v", problem)
	}
}

func TestControllerReconcileDeploymentsReturnsBadGatewayOnWireGuardFailure(t *testing.T) {
	server := &controllerServer{
		adminToken: "dev-admin-token",
		store:      apigateway.NewMemoryStore(101),
		executor:   &runtimeExecutorStub{},
		access:     &serviceAccessExecutorStub{},
		wireGuard:  &controllerWireGuardReconcilerStub{err: errors.New("wg sync failed")},
		metrics:    newControllerServiceMetrics(),
		now:        time.Now,
	}
	mux := newControllerReconcileTestMux(server)

	request := httptest.NewRequest(http.MethodPost, "/internal/v1/deployments/reconcile", nil)
	request.Header.Set("Authorization", "Bearer dev-admin-token")
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, request)

	if response.Code != http.StatusBadGateway {
		t.Fatalf("expected reconcile 502, got %d", response.Code)
	}

	var problem httpapi.ProblemDetails
	if err := json.Unmarshal(response.Body.Bytes(), &problem); err != nil {
		t.Fatalf("decode problem: %v", err)
	}
	if problem.Title != "Deployment reconcile failed" || !strings.Contains(problem.Detail, controllerWireGuardTruthUnknownDetail) {
		t.Fatalf("unexpected problem %+v", problem)
	}
}

func TestControllerReconcileAccessReturnsBadGatewayOnWireGuardFailure(t *testing.T) {
	server := &controllerServer{
		adminToken: "dev-admin-token",
		store:      apigateway.NewMemoryStore(101),
		executor:   &runtimeExecutorStub{},
		access:     &serviceAccessExecutorStub{},
		wireGuard:  &controllerWireGuardReconcilerStub{err: errors.New("wg sync failed")},
		metrics:    newControllerServiceMetrics(),
		now:        time.Now,
	}
	mux := newControllerReconcileTestMux(server)

	request := httptest.NewRequest(http.MethodPost, "/internal/v1/access/reconcile", nil)
	request.Header.Set("Authorization", "Bearer dev-admin-token")
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, request)

	if response.Code != http.StatusBadGateway {
		t.Fatalf("expected reconcile 502, got %d: %s", response.Code, response.Body.String())
	}

	var problem httpapi.ProblemDetails
	if err := json.Unmarshal(response.Body.Bytes(), &problem); err != nil {
		t.Fatalf("decode problem: %v", err)
	}
	if problem.Title != "Access reconcile failed" || !strings.Contains(problem.Detail, controllerWireGuardTruthUnknownDetail) {
		t.Fatalf("unexpected problem %+v", problem)
	}
}

func TestControllerReconcileAccessRejectsNonAppliedWireGuardStatus(t *testing.T) {
	server := &controllerServer{
		adminToken: "dev-admin-token",
		store:      apigateway.NewMemoryStore(101),
		executor:   &runtimeExecutorStub{},
		access:     &serviceAccessExecutorStub{},
		wireGuard: &controllerWireGuardReconcilerStub{
			status: apigateway.WireGuardGatewayStatus{State: "error", LastError: "peer apply incomplete"},
		},
		metrics: newControllerServiceMetrics(),
		now:     time.Now,
	}
	mux := newControllerReconcileTestMux(server)

	request := httptest.NewRequest(http.MethodPost, "/internal/v1/access/reconcile", nil)
	request.Header.Set("Authorization", "Bearer dev-admin-token")
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, request)

	if response.Code != http.StatusBadGateway {
		t.Fatalf("expected reconcile 502, got %d: %s", response.Code, response.Body.String())
	}
}
