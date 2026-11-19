package main

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"adplatform/internal/platform/database/dbtest"
	"adplatform/internal/platform/servicetest"
	"adplatform/internal/services/apigateway"
)

func setControllerEnv(t *testing.T) {
	t.Helper()
	t.Setenv("CONTROLLER_SERVICE_ADDR", "127.0.0.1:0")
	t.Setenv("CONTROLLER_STATE_BACKEND", "memory")
	t.Setenv("CONTROLLER_RUNTIME_MODE", "dry-run")
	t.Setenv("CONTROLLER_ACCESS_MODE", "dry-run")
	t.Setenv("ADMIN_API_TOKEN", "unit-test-admin-token-7f3a")
	t.Setenv("WIREGUARD_GATEWAY_INTERNAL_URL", "")
	t.Setenv("CONTROLLER_RECONCILE_ACCESS_ON_STARTUP", "false")
}

func TestControllerRunServesUntilCancelled(t *testing.T) {
	setControllerEnv(t)
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	if err := run(ctx); err != nil {
		t.Fatalf("run: %v", err)
	}
}

func TestControllerRunWithDockerExecutor(t *testing.T) {
	setControllerEnv(t)
	t.Setenv("CONTROLLER_RUNTIME_MODE", "docker")
	t.Setenv("CONTROLLER_CAPTURE_MODE", "disabled")
	t.Setenv("CONTROLLER_SERVICE_CAP_DROP", "ALL")
	t.Setenv("CONTROLLER_RECONCILE_DEPLOYMENTS_ON_STARTUP", "false")
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	if err := run(ctx); err != nil {
		t.Fatalf("docker-mode run: %v", err)
	}
	t.Setenv("CONTROLLER_CAPTURE_MODE", "bogus")
	if err := run(context.Background()); err == nil {
		t.Fatal("an invalid capture mode must abort startup")
	}
}

func TestControllerRunFailsOnStartupRestore(t *testing.T) {
	setControllerEnv(t)
	t.Setenv("CONTROLLER_RECONCILE_ACCESS_ON_STARTUP", "true")
	t.Setenv("CONTROLLER_ACCESS_MODE", "files")
	t.Setenv("CONTROLLER_ACCESS_RULES_PATH", "/proc/forbidden/rules")
	t.Setenv("CONTROLLER_ACCESS_STATUS_PATH", "/proc/forbidden/status")
	if err := run(context.Background()); err == nil {
		t.Fatal("a failed startup restore must abort startup")
	}
}

func TestControllerMainLifecycle(t *testing.T) {
	setControllerEnv(t)
	got := servicetest.CaptureFatal(t, &fatal)
	servicetest.RunMainUntilSIGTERM(t, main)
	if len(got()) != 0 {
		t.Fatalf("clean shutdown must not be fatal: %v", got())
	}
	t.Setenv("CONTROLLER_STATE_BACKEND", "postgres")
	t.Setenv("POSTGRES_DSN", "postgres://nobody@127.0.0.1:1/none?sslmode=disable&connect_timeout=1")
	main()
	if len(got()) == 0 {
		t.Fatal("an unreachable database must be fatal")
	}
}

func TestControllerBuildStorePostgres(t *testing.T) {
	db := dbtest.IsolatedSchema(t, "TEST_POSTGRES_DSN", "ctl_buildstore")
	t.Setenv("CONTROLLER_STATE_BACKEND", "postgres")
	t.Setenv("CONTROLLER_AUTO_MIGRATE", "true")
	t.Setenv("POSTGRES_DSN", dbtest.SchemaDSN(t, "TEST_POSTGRES_DSN", db))
	store, err := buildStore(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	store.Close()
	readOnly := dbtest.IsolatedSchema(t, "TEST_POSTGRES_DSN", "ctl_buildstore_ro")
	t.Setenv("POSTGRES_DSN", dbtest.SchemaDSN(t, "TEST_POSTGRES_DSN", readOnly)+"&default_transaction_read_only=on")
	if _, err := buildStore(context.Background()); err == nil {
		t.Fatal("a migration failure must be returned")
	}
}

// closedStore is a Postgres store over a closed pool: every call fails.
func closedStore(t *testing.T) apigateway.Store {
	t.Helper()
	db, _ := sql.Open("pgx", "postgres://nobody@127.0.0.1:1/none")
	db.Close()
	return apigateway.NewPostgresStore(db)
}

func TestControllerHandlersOverFailingStore(t *testing.T) {
	mux := newHandlerTestServer(closedStore(t), &fakeRuntime{}, &fakeAccess{}, noopControllerWireGuardReconciler{})
	for _, tc := range []struct{ method, path string }{
		{http.MethodGet, "/internal/v1/deployments"},
		{http.MethodPost, "/internal/v1/deployments/reconcile"},
	} {
		if response := controllerRequest(mux, tc.method, tc.path, controllerTestToken, ""); response.Code != http.StatusInternalServerError {
			t.Fatalf("%s %s: want 500, got %d", tc.method, tc.path, response.Code)
		}
	}
}

type scriptedReconcileStore struct {
	tasks      []apigateway.ControllerRuntimeTask
	tasksErr   error
	reconErr   error
	policyErr  error
	reconState controllerDeploymentReconcileState
}

func (s scriptedReconcileStore) ListControllerRuntimeTasks(context.Context) ([]apigateway.ControllerRuntimeTask, error) {
	return s.tasks, s.tasksErr
}
func (s scriptedReconcileStore) ListControllerServiceAccessPolicies(context.Context) ([]apigateway.ControllerServiceAccessPolicy, error) {
	return nil, s.policyErr
}
func (s scriptedReconcileStore) ReconcileDeployments(context.Context, time.Time) (controllerDeploymentReconcileState, error) {
	return s.reconState, s.reconErr
}

func TestTrustedReconcilerBranches(t *testing.T) {
	ctx := context.Background()
	boom := errors.New("boom")
	task := []apigateway.ControllerRuntimeTask{{TeamID: 1, ChallengeID: 2}}
	host := &fakeAccess{status: apigateway.ControllerAccessStatus{Mode: "host"}}
	cases := map[string]struct {
		r    controllerTrustedReconciler
		want string
	}{
		"list tasks":     {newControllerTrustedReconciler(scriptedReconcileStore{tasksErr: boom}, &fakeRuntime{}, &fakeAccess{}, nil, nil), "boom"},
		"ensure":         {newControllerTrustedReconciler(scriptedReconcileStore{tasks: task}, &fakeRuntime{err: boom}, &fakeAccess{}, nil, nil), "team 1 challenge 2"},
		"reconcile":      {newControllerTrustedReconciler(scriptedReconcileStore{reconErr: boom}, &fakeRuntime{}, &fakeAccess{}, nil, nil), "boom"},
		"policies":       {newControllerTrustedReconciler(scriptedReconcileStore{policyErr: boom}, &fakeRuntime{}, &fakeAccess{}, nil, nil), "boom"},
		"access":         {newControllerTrustedReconciler(scriptedReconcileStore{}, &fakeRuntime{}, &fakeAccess{applyErr: boom}, nil, nil), controllerAccessTruthUnknownDetail},
		"wireguard":      {newControllerTrustedReconciler(scriptedReconcileStore{}, &fakeRuntime{}, host, nil, nil), controllerWireGuardTruthUnknownDetail},
		"wg not applied": {newControllerTrustedReconciler(scriptedReconcileStore{}, &fakeRuntime{}, host, fakeWireGuard{status: apigateway.WireGuardGatewayStatus{State: "pending"}}, time.Now), controllerWireGuardTruthUnknownDetail},
	}
	for name, tc := range cases {
		if _, err := tc.r.Reconcile(ctx); err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: want error containing %q, got %v", name, tc.want, err)
		}
	}
	state, err := newControllerTrustedReconciler(scriptedReconcileStore{reconState: controllerDeploymentReconcileState{ProcessedJobs: 3}}, &fakeRuntime{}, &fakeAccess{}, nil, nil).
		ReconcileWithOptions(ctx, controllerTrustedReconcileOptions{includeRuntime: true})
	if err != nil || state.ProcessedJobs != 3 {
		t.Fatalf("runtime-only reconcile: %+v %v", state, err)
	}
	if _, err := newControllerTrustedReconciler(scriptedReconcileStore{}, &fakeRuntime{}, host, fakeWireGuard{status: apigateway.WireGuardGatewayStatus{State: "applied"}}, nil).Reconcile(ctx); err != nil {
		t.Fatalf("converged reconcile: %v", err)
	}
	var phase *controllerReconcilePhaseError
	wrapped := wrapControllerReconcilePhaseError(502, "t", "d", nil)
	if !errors.As(wrapped, &phase) || phase.Unwrap() != nil || phase.Title() != "t" || phase.Detail() != "d" {
		t.Fatalf("a causeless phase error: %#v", wrapped)
	}
	t.Setenv("CONTROLLER_RECONCILE_DEPLOYMENTS_ON_STARTUP", "false")
	t.Setenv("CONTROLLER_RECONCILE_ACCESS_ON_STARTUP", "false")
	if err := restoreControllerState(ctx, scriptedReconcileStore{tasksErr: boom}, &fakeRuntime{}, &fakeAccess{}, nil, nil); err != nil {
		t.Fatal("a disabled startup restore must be a no-op")
	}
}

func TestReconcileStoreAdapterSurfacesStoreFailure(t *testing.T) {
	if _, err := (controllerReconcileStoreAdapter{store: closedStore(t)}).ReconcileDeployments(context.Background(), time.Now()); err == nil {
		t.Fatal("a store failure must surface")
	}
}

func TestHTTPWireGuardReconciler(t *testing.T) {
	t.Setenv("WIREGUARD_GATEWAY_INTERNAL_URL", "")
	if _, ok := newControllerWireGuardReconciler().(noopControllerWireGuardReconciler); !ok {
		t.Fatal("no URL must select the noop reconciler")
	}
	if _, err := (noopControllerWireGuardReconciler{}).Reconcile(context.Background()); !errors.Is(err, errWireGuardGatewayUnavailable) {
		t.Fatal(err)
	}
	cases := []struct {
		status int
		body   string
		want   string
	}{
		{200, `{"state":"applied"}`, ""},
		{200, `{bad`, "invalid"},
		{502, `{"title":"t","detail":" wg down "}`, "wg down"},
		{502, `{"title":" Gateway "}`, "Gateway"},
		{502, `{"message":"legacy"}`, "legacy"},
		{502, `{}`, "status 502"},
		{502, `oops`, "status 502"},
	}
	for _, tc := range cases {
		var auth string
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			auth = r.Header.Get("Authorization")
			w.WriteHeader(tc.status)
			_, _ = w.Write([]byte(tc.body))
		}))
		t.Setenv("WIREGUARD_GATEWAY_INTERNAL_URL", server.URL)
		t.Setenv("WIREGUARD_GATEWAY_INTERNAL_TOKEN", "wg-internal-token-1234")
		status, err := newControllerWireGuardReconciler().Reconcile(context.Background())
		server.Close()
		if tc.want == "" {
			if err != nil || status.State != "applied" || auth != "Bearer wg-internal-token-1234" {
				t.Fatalf("success: %+v %v %q", status, err, auth)
			}
			continue
		}
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Fatalf("status %d body %s: want %q, got %v", tc.status, tc.body, tc.want, err)
		}
	}
	tokenless := &httpControllerWireGuardReconciler{baseURL: "http://127.0.0.1:1", client: http.DefaultClient}
	if _, err := tokenless.Reconcile(context.Background()); err == nil {
		t.Fatal("transport failure must error")
	}
	if _, err := (&httpControllerWireGuardReconciler{baseURL: "://bad", client: http.DefaultClient}).Reconcile(context.Background()); err == nil {
		t.Fatal("invalid URL must error")
	}
	if _, err := decodeControllerProblemBody(errReader{}); err == nil {
		t.Fatal("a body read failure must error")
	}
}

type errReader struct{}

func (errReader) Read([]byte) (int, error) { return 0, errors.New("read failed") }

func TestHTTPCheckerValidationClient(t *testing.T) {
	t.Setenv("CHECKER_RUNNER_INTERNAL_URL", "")
	if newCheckerValidationClient() != nil {
		t.Fatal("no URL means no checker validation client")
	}
	cases := []struct {
		status int
		body   string
		want   string
	}{
		{200, `{"status":"valid","contract_ok":true}`, ""},
		{200, `{bad`, "invalid"},
		{502, `{"title":"t","detail":" docker down "}`, "docker down"},
		{502, `{"title":" Runner "}`, "Runner"},
		{502, `{}`, "status 502"},
		{502, `oops`, "status 502"},
	}
	for _, tc := range cases {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(tc.status)
			_, _ = w.Write([]byte(tc.body))
		}))
		t.Setenv("CHECKER_RUNNER_INTERNAL_URL", server.URL)
		t.Setenv("CHECKER_RUNNER_INTERNAL_TOKEN", "runner-token-5678")
		result, err := newCheckerValidationClient().Validate(context.Background(), apigateway.CheckerValidationRequest{ChallengeID: 1})
		server.Close()
		if tc.want == "" {
			if err != nil || result.Status != "valid" {
				t.Fatalf("success: %+v %v", result, err)
			}
			continue
		}
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Fatalf("status %d: want %q, got %v", tc.status, tc.want, err)
		}
	}
	if _, err := cloneCheckerValidationClient("http://127.0.0.1:1", "", http.DefaultClient).Validate(context.Background(), apigateway.CheckerValidationRequest{}); err == nil {
		t.Fatal("transport failure must error")
	}
	if _, err := cloneCheckerValidationClient("://bad", "", http.DefaultClient).Validate(context.Background(), apigateway.CheckerValidationRequest{}); err == nil {
		t.Fatal("invalid URL must error")
	}
}

func TestControllerMetricsAndArtifactHelpers(t *testing.T) {
	metrics := &controllerServiceMetrics{}
	metrics.ensureMapsLocked()
	if len(metrics.operations) == 0 || len(metrics.accessPoliciesAppliedTotal) == 0 {
		t.Fatal("zero-value metrics must be initialized")
	}
	if !metricParseTime("").IsZero() || !metricParseTime("nope").IsZero() || metricParseTime("2026-01-02T03:04:05Z").IsZero() {
		t.Fatal("metricParseTime")
	}
	if err := ensureControllerParentDir("  "); err != nil {
		t.Fatal("an empty path needs no directory")
	}
	original := marshalAccessStatus
	t.Cleanup(func() { marshalAccessStatus = original })
	marshalAccessStatus = func(any, string, string) ([]byte, error) { return nil, errors.New("encode failed") }
	if err := writeControllerAccessStatus(t.TempDir()+"/s.json", apigateway.ControllerAccessStatus{}); err == nil {
		t.Fatal("encoding failures must surface")
	}
}
