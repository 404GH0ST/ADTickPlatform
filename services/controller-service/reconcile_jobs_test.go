package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"adplatform/internal/services/apigateway"
)

type blockingReconcileAccess struct {
	fakeAccess
	entered chan struct{}
	release chan struct{}
}

func (a *blockingReconcileAccess) Apply(ctx context.Context, _ []apigateway.ControllerServiceAccessPolicy, _ time.Time) (apigateway.ControllerAccessStatus, error) {
	close(a.entered)
	select {
	case <-a.release:
		return apigateway.ControllerAccessStatus{}, nil
	case <-ctx.Done():
		return apigateway.ControllerAccessStatus{}, ctx.Err()
	}
}

func TestDeploymentReconcileJobSurvivesRequestCancellation(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	access := &blockingReconcileAccess{entered: make(chan struct{}), release: make(chan struct{})}
	server := &controllerServer{
		adminToken: controllerTestToken,
		store:      apigateway.NewMemoryStore(101),
		executor:   &fakeRuntime{},
		access:     access,
		metrics:    newControllerServiceMetrics(),
		now:        time.Now,
	}
	server.reconcileJobs.ctx, server.reconcileJobs.cancel = ctx, cancel
	defer server.reconcileJobs.shutdown()
	mux := newControllerReconcileTestMux(server)
	const reconcilePath = "/internal/v1/deployments/reconcile"
	const jobPath = reconcilePath + "/job"
	readJob := func(response *httptest.ResponseRecorder, wantStatus int) apigateway.ControllerDeploymentReconcileJob {
		t.Helper()
		if response.Code != wantStatus {
			t.Fatalf("want HTTP %d, got %d: %s", wantStatus, response.Code, response.Body)
		}
		var job apigateway.ControllerDeploymentReconcileJob
		if err := json.Unmarshal(response.Body.Bytes(), &job); err != nil {
			t.Fatal(err)
		}
		return job
	}
	status := func() apigateway.ControllerDeploymentReconcileJob {
		return readJob(controllerRequest(mux, http.MethodGet, jobPath, controllerTestToken, ""), http.StatusOK)
	}

	if idle := status(); idle.Status != "idle" {
		t.Fatalf("initial status: %+v", idle)
	}
	requestCtx, cancelRequest := context.WithCancel(ctx)
	request := httptest.NewRequestWithContext(requestCtx, http.MethodPost, jobPath, nil)
	request.Header.Set("Authorization", "Bearer "+controllerTestToken)
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, request)
	cancelRequest()
	started := readJob(response, http.StatusAccepted)
	if started.ID == "" || started.Status != "running" {
		t.Fatalf("started job: %+v", started)
	}
	select {
	case <-access.entered:
	case <-ctx.Done():
		t.Fatal("background reconcile did not reach host access")
	}
	if running := status(); running.ID != started.ID || running.Status != "running" || running.Phase != "access" {
		t.Fatalf("running job: %+v", running)
	}
	same := readJob(controllerRequest(mux, http.MethodPost, jobPath, controllerTestToken, ""), http.StatusAccepted)
	if same.ID != started.ID {
		t.Fatalf("concurrent request replaced the active job: %+v", same)
	}
	if response := controllerRequest(mux, http.MethodPost, reconcilePath, controllerTestToken, ""); response.Code != http.StatusConflict {
		t.Fatalf("synchronous reconcile overlapped the job: %d", response.Code)
	}
	close(access.release)
	select {
	case <-server.reconcileJobs.done:
	case <-ctx.Done():
		t.Fatal("background reconcile did not complete")
	}
	if completed := status(); completed.ID != started.ID || completed.Status != "completed" || completed.Result == nil {
		t.Fatalf("completed job: %+v", completed)
	}
}
