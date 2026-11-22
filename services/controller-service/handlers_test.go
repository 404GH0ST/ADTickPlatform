package main

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

	"adplatform/internal/platform/httpapi"
	"adplatform/internal/services/apigateway"
)

var errFakeRuntime = errors.New("simulated docker failure")

type fakeRuntime struct {
	err        error
	resetErr   error
	validation apigateway.ChallengeValidationResult
	calls      []string
}

func (f *fakeRuntime) record(call string) error {
	f.calls = append(f.calls, call)
	return f.err
}

func (f *fakeRuntime) EnsureService(context.Context, apigateway.ControllerRuntimeTask) error {
	return f.record("ensure")
}

func (f *fakeRuntime) FactoryResetService(context.Context, apigateway.ControllerRuntimeTask) error {
	f.calls = append(f.calls, "factory-reset")
	if f.resetErr != nil {
		return f.resetErr
	}
	return f.err
}

func (f *fakeRuntime) RestartService(context.Context, apigateway.ControllerRuntimeTask) error {
	return f.record("restart")
}

func (f *fakeRuntime) ApplySSHCredential(_ context.Context, _ apigateway.ControllerRuntimeTask, credential apigateway.ControllerSSHCredential) error {
	return f.record("ssh:" + credential.Password)
}

func (f *fakeRuntime) ValidateChallengeRuntime(context.Context, apigateway.ChallengeValidationRequest) (apigateway.ChallengeValidationResult, error) {
	return f.validation, f.record("validate")
}

func (f *fakeRuntime) RemoveService(context.Context, int, int) error { return f.record("remove") }
func (f *fakeRuntime) RemoveTeamServices(context.Context, int, ...string) error {
	return f.record("remove-team")
}
func (f *fakeRuntime) RemoveChallengeServices(context.Context, int) error {
	return f.record("remove-challenge")
}

type fakeAccess struct {
	status   apigateway.ControllerAccessStatus
	applyErr error
	teardown error
}

func (f *fakeAccess) Status() apigateway.ControllerAccessStatus { return f.status }
func (f *fakeAccess) Apply(context.Context, []apigateway.ControllerServiceAccessPolicy, time.Time) (apigateway.ControllerAccessStatus, error) {
	if f.applyErr != nil {
		return apigateway.ControllerAccessStatus{LastError: f.applyErr.Error()}, f.applyErr
	}
	return f.status, nil
}
func (f *fakeAccess) Teardown(context.Context) error { return f.teardown }

type fakeWireGuard struct {
	status apigateway.WireGuardGatewayStatus
	err    error
}

func (f fakeWireGuard) Reconcile(context.Context) (apigateway.WireGuardGatewayStatus, error) {
	return f.status, f.err
}

// lookupFailingStore fails the lookups the controller handlers make while
// delegating everything else to a working store.
type lookupFailingStore struct {
	apigateway.Store
	err error
}

func (s lookupFailingStore) ListControllerServiceAccessPolicies(context.Context) ([]apigateway.ControllerServiceAccessPolicy, error) {
	return nil, s.err
}

func (s lookupFailingStore) GetControllerServiceAccessPolicy(context.Context, int, int) (apigateway.ControllerServiceAccessPolicy, error) {
	return apigateway.ControllerServiceAccessPolicy{}, s.err
}

func (s lookupFailingStore) GetControllerRuntimeTask(context.Context, int, int) (apigateway.ControllerRuntimeTask, error) {
	return apigateway.ControllerRuntimeTask{}, s.err
}

const controllerTestToken = "controller-test-token"

func newHandlerTestServer(store apigateway.Store, runtime *fakeRuntime, access *fakeAccess, wireGuard controllerWireGuardReconciler) *http.ServeMux {
	server := &controllerServer{
		adminToken: controllerTestToken,
		store:      store,
		executor:   runtime,
		access:     access,
		wireGuard:  wireGuard,
		metrics:    newControllerServiceMetrics(),
		now:        time.Now,
	}
	mux := httpapi.NewBaseMux(httpapi.ServiceInfo{Name: "controller-service", Version: "test", Addr: ":0"})
	server.RegisterRoutes(mux)
	return mux
}

func controllerRequest(mux *http.ServeMux, method, path, token, body string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	recorder := httptest.NewRecorder()
	mux.ServeHTTP(recorder, request)
	return recorder
}

var controllerRoutePattern = regexp.MustCompile(`mux\.HandleFunc\("([A-Z]+) (/[^"]+)"`)
var controllerPathParam = regexp.MustCompile(`\{[a-z_]+\}`)

func controllerRoutes(t *testing.T) [][2]string {
	t.Helper()
	source, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatal(err)
	}
	var routes [][2]string
	for _, match := range controllerRoutePattern.FindAllStringSubmatch(string(source), -1) {
		routes = append(routes, [2]string{match[1], match[2]})
	}
	if len(routes) < 13 {
		t.Fatalf("route parse looks wrong: %v", routes)
	}
	return routes
}

// Every controller endpoint can drive Docker or the host firewall, so each one
// must reject callers without the internal token, before touching the runtime.
func TestEveryControllerRouteRequiresTheInternalToken(t *testing.T) {
	runtime := &fakeRuntime{}
	mux := newHandlerTestServer(apigateway.NewMemoryStore(101), runtime, &fakeAccess{}, noopControllerWireGuardReconciler{})
	for _, route := range controllerRoutes(t) {
		path := controllerPathParam.ReplaceAllString(route[1], "1")
		for _, token := range []string{"", "wrong-token", controllerTestToken + "x"} {
			response := controllerRequest(mux, route[0], path, token, `{"password":"p"}`)
			if response.Code != http.StatusForbidden {
				t.Errorf("%s %s token=%q: want 403, got %d", route[0], path, token, response.Code)
			}
		}
	}
	if len(runtime.calls) != 0 {
		t.Fatalf("unauthenticated requests reached the runtime: %v", runtime.calls)
	}
}

func TestControllerRoutesRejectMalformedIDs(t *testing.T) {
	runtime := &fakeRuntime{}
	mux := newHandlerTestServer(apigateway.NewMemoryStore(101), runtime, &fakeAccess{}, noopControllerWireGuardReconciler{})
	for _, route := range controllerRoutes(t) {
		params := controllerPathParam.FindAllString(route[1], -1)
		for index := range params {
			for _, bad := range []string{"x", "0", "-1"} {
				position := 0
				path := controllerPathParam.ReplaceAllStringFunc(route[1], func(string) string {
					value := "1"
					if position == index {
						value = bad
					}
					position++
					return value
				})
				response := controllerRequest(mux, route[0], path, controllerTestToken, `{"password":"p"}`)
				if response.Code != http.StatusBadRequest {
					t.Errorf("%s %s: want 400, got %d: %s", route[0], path, response.Code, response.Body.String())
				}
			}
		}
	}
	if len(runtime.calls) != 0 {
		t.Fatalf("malformed requests reached the runtime: %v", runtime.calls)
	}
}

func TestControllerRuntimeHandlersSucceed(t *testing.T) {
	runtime := &fakeRuntime{validation: apigateway.ChallengeValidationResult{ChallengeID: 1, Status: "valid"}}
	access := &fakeAccess{status: apigateway.ControllerAccessStatus{Mode: "dry-run", PoliciesTotal: 3}}
	mux := newHandlerTestServer(apigateway.NewMemoryStore(101), runtime, access, noopControllerWireGuardReconciler{})

	cases := []struct {
		method, path, body string
		want               int
	}{
		{http.MethodGet, "/internal/v1/deployments", "", http.StatusOK},
		{http.MethodPost, "/internal/v1/challenges/validate", `{"challenge_id":1,"baseline_image":"img"}`, http.StatusOK},
		{http.MethodGet, "/internal/v1/access/status", "", http.StatusOK},
		{http.MethodPost, "/internal/v1/access/reconcile", "", http.StatusOK},
		{http.MethodPost, "/internal/v1/access/teardown", "", http.StatusNoContent},
		{http.MethodPost, "/internal/v1/teams/101/services/1/access/reconcile", "", http.StatusOK},
		{http.MethodPost, "/internal/v1/teams/101/services/1/ssh-credential", `{"password":"Adp-secret"}`, http.StatusOK},
		{http.MethodPost, "/internal/v1/teams/101/services/1/reset/factory", "", http.StatusOK},
		{http.MethodPost, "/internal/v1/teams/101/services/1/restart", "", http.StatusOK},
		{http.MethodPost, "/internal/v1/teams/101/services/1/remove", "", http.StatusOK},
		{http.MethodPost, "/internal/v1/teams/101/remove", "", http.StatusOK},
		{http.MethodPost, "/internal/v1/challenges/1/remove", "", http.StatusOK},
	}
	for _, tc := range cases {
		response := controllerRequest(mux, tc.method, tc.path, controllerTestToken, tc.body)
		if response.Code != tc.want {
			t.Errorf("%s %s: want %d, got %d: %s", tc.method, tc.path, tc.want, response.Code, response.Body.String())
		}
	}
	want := "validate,ssh:Adp-secret,factory-reset,restart,remove,remove-team,remove-challenge"
	if got := strings.Join(runtime.calls, ","); got != want {
		t.Fatalf("runtime calls = %s, want %s", got, want)
	}
	if response := controllerRequest(mux, http.MethodPost, "/internal/v1/teams/101/services/1/reset/factory", controllerTestToken, ""); response.Header().Get("X-ADPlatform-Reset-Predelete") != "" {
		t.Fatal("a successful reset must not carry a predelete stage header")
	}
}

func TestControllerHandlersRejectInvalidBodies(t *testing.T) {
	runtime := &fakeRuntime{}
	mux := newHandlerTestServer(apigateway.NewMemoryStore(101), runtime, &fakeAccess{}, noopControllerWireGuardReconciler{})
	for _, tc := range []struct{ path, body string }{
		{"/internal/v1/challenges/validate", `{"challenge_id":0,"baseline_image":"img"}`},
		{"/internal/v1/challenges/validate", `{"challenge_id":1}`},
		{"/internal/v1/challenges/validate", `not json`},
		{"/internal/v1/teams/101/services/1/ssh-credential", `{"password":""}`},
		{"/internal/v1/teams/101/services/1/ssh-credential", `{"password":"p","extra":1}`},
	} {
		if response := controllerRequest(mux, http.MethodPost, tc.path, controllerTestToken, tc.body); response.Code != http.StatusBadRequest {
			t.Errorf("%s %s: want 400, got %d", tc.path, tc.body, response.Code)
		}
	}
	if len(runtime.calls) != 0 {
		t.Fatalf("invalid bodies reached the runtime: %v", runtime.calls)
	}
}

func TestControllerHandlersSurfaceRuntimeFailures(t *testing.T) {
	runtime := &fakeRuntime{err: errFakeRuntime}
	access := &fakeAccess{applyErr: errors.New("nft failed"), teardown: errors.New("teardown failed")}
	mux := newHandlerTestServer(apigateway.NewMemoryStore(101), runtime, access, noopControllerWireGuardReconciler{})
	for _, tc := range []struct{ method, path, body string }{
		{http.MethodPost, "/internal/v1/challenges/validate", `{"challenge_id":1,"baseline_image":"img"}`},
		{http.MethodPost, "/internal/v1/access/reconcile", ""},
		{http.MethodPost, "/internal/v1/access/teardown", ""},
		{http.MethodPost, "/internal/v1/teams/101/services/1/access/reconcile", ""},
		{http.MethodPost, "/internal/v1/teams/101/services/1/ssh-credential", `{"password":"p"}`},
		{http.MethodPost, "/internal/v1/teams/101/services/1/reset/factory", ""},
		{http.MethodPost, "/internal/v1/teams/101/services/1/restart", ""},
		{http.MethodPost, "/internal/v1/teams/101/services/1/remove", ""},
		{http.MethodPost, "/internal/v1/teams/101/remove", ""},
		{http.MethodPost, "/internal/v1/challenges/1/remove", ""},
	} {
		if response := controllerRequest(mux, tc.method, tc.path, controllerTestToken, tc.body); response.Code < 500 {
			t.Errorf("%s %s: want 5xx, got %d: %s", tc.method, tc.path, response.Code, response.Body.String())
		}
	}
}

func TestControllerFactoryResetReportsPredeleteStage(t *testing.T) {
	for _, tc := range []struct {
		changed bool
		want    string
	}{{false, "clean"}, {true, "runtime-changed"}} {
		runtime := &fakeRuntime{resetErr: resetPreflightError{err: errFakeRuntime, runtimeMayHaveChanged: tc.changed}}
		mux := newHandlerTestServer(apigateway.NewMemoryStore(101), runtime, &fakeAccess{}, noopControllerWireGuardReconciler{})
		response := controllerRequest(mux, http.MethodPost, "/internal/v1/teams/101/services/1/reset/factory", controllerTestToken, "")
		if response.Code != http.StatusInternalServerError || response.Header().Get("X-ADPlatform-Reset-Predelete") != tc.want {
			t.Fatalf("changed=%v: want 500 with stage %q, got %d %q", tc.changed, tc.want, response.Code, response.Header().Get("X-ADPlatform-Reset-Predelete"))
		}
	}
	// A rejected task lookup happens before anything runs: the stage stays clean.
	mux := newHandlerTestServer(apigateway.NewMemoryStore(101), &fakeRuntime{}, &fakeAccess{}, noopControllerWireGuardReconciler{})
	response := controllerRequest(mux, http.MethodPost, "/internal/v1/teams/999/services/1/reset/factory", controllerTestToken, "")
	if response.Code < 400 || response.Header().Get("X-ADPlatform-Reset-Predelete") != "clean" {
		t.Fatalf("unknown task: want failure with clean stage, got %d %q", response.Code, response.Header().Get("X-ADPlatform-Reset-Predelete"))
	}
}

func TestControllerHandlersSurfaceStoreFailures(t *testing.T) {
	for _, tc := range []struct {
		err  error
		want int
	}{
		{errors.New("db down"), http.StatusInternalServerError},
		{apigateway.ErrChallengeNotFound, http.StatusNotFound},
	} {
		runtime := &fakeRuntime{}
		store := lookupFailingStore{Store: apigateway.NewMemoryStore(101), err: tc.err}
		mux := newHandlerTestServer(store, runtime, &fakeAccess{}, noopControllerWireGuardReconciler{})
		paths := []string{
			"/internal/v1/teams/101/services/1/access/reconcile",
			"/internal/v1/teams/101/services/1/ssh-credential",
			"/internal/v1/teams/101/services/1/reset/factory",
			"/internal/v1/teams/101/services/1/restart",
		}
		for _, path := range paths {
			if response := controllerRequest(mux, http.MethodPost, path, controllerTestToken, `{"password":"p"}`); response.Code != tc.want {
				t.Errorf("%s with %v: want %d, got %d", path, tc.err, tc.want, response.Code)
			}
		}
		if len(runtime.calls) != 0 {
			t.Fatalf("failed task lookups reached the runtime: %v", runtime.calls)
		}
	}
	mux := newHandlerTestServer(lookupFailingStore{Store: apigateway.NewMemoryStore(101), err: errors.New("db down")}, &fakeRuntime{}, &fakeAccess{}, noopControllerWireGuardReconciler{})
	for _, tc := range []struct{ method, path string }{
		{http.MethodPost, "/internal/v1/access/reconcile"},
	} {
		if response := controllerRequest(mux, tc.method, tc.path, controllerTestToken, ""); response.Code != http.StatusInternalServerError {
			t.Errorf("%s %s: want 500, got %d", tc.method, tc.path, response.Code)
		}
	}
	// The per-service reconcile lists every policy after its own lookup.
	listFailing := &listPoliciesFailingStore{Store: apigateway.NewMemoryStore(101)}
	mux = newHandlerTestServer(listFailing, &fakeRuntime{}, &fakeAccess{}, noopControllerWireGuardReconciler{})
	if response := controllerRequest(mux, http.MethodPost, "/internal/v1/teams/101/services/1/access/reconcile", controllerTestToken, ""); response.Code != http.StatusInternalServerError {
		t.Fatalf("policy listing failure: want 500, got %d", response.Code)
	}
}

type listPoliciesFailingStore struct{ apigateway.Store }

func (listPoliciesFailingStore) ListControllerServiceAccessPolicies(context.Context) ([]apigateway.ControllerServiceAccessPolicy, error) {
	return nil, errors.New("list failed")
}

func TestControllerAccessReconcileRequiresWireGuardConvergence(t *testing.T) {
	access := &fakeAccess{status: apigateway.ControllerAccessStatus{Mode: "host", State: "applied", PoliciesTotal: 2}}
	for name, wireGuard := range map[string]controllerWireGuardReconciler{
		"reconcile error":   fakeWireGuard{err: errors.New("wg down")},
		"not applied state": fakeWireGuard{status: apigateway.WireGuardGatewayStatus{State: "pending"}},
	} {
		t.Run(name, func(t *testing.T) {
			if !requiresWireGuardConverge(access.status) {
				t.Skip("access status does not require WireGuard convergence in this configuration")
			}
			mux := newHandlerTestServer(apigateway.NewMemoryStore(101), &fakeRuntime{}, access, wireGuard)
			if response := controllerRequest(mux, http.MethodPost, "/internal/v1/access/reconcile", controllerTestToken, ""); response.Code != http.StatusBadGateway {
				t.Fatalf("want 502 when WireGuard did not converge, got %d: %s", response.Code, response.Body.String())
			}
		})
	}
}

func TestControllerReconcileDeploymentsHandler(t *testing.T) {
	mux := newHandlerTestServer(apigateway.NewMemoryStore(101), &fakeRuntime{}, &fakeAccess{}, noopControllerWireGuardReconciler{})
	if response := controllerRequest(mux, http.MethodPost, "/internal/v1/deployments/reconcile", controllerTestToken, ""); response.Code != http.StatusOK {
		t.Fatalf("reconcile: want 200, got %d: %s", response.Code, response.Body.String())
	}
	failing := newHandlerTestServer(apigateway.NewMemoryStore(101), &fakeRuntime{err: errFakeRuntime}, &fakeAccess{applyErr: errors.New("nft failed")}, noopControllerWireGuardReconciler{})
	if response := controllerRequest(failing, http.MethodPost, "/internal/v1/deployments/reconcile", controllerTestToken, ""); response.Code < 400 {
		t.Fatalf("reconcile with failing runtime: want an error, got %d: %s", response.Code, response.Body.String())
	}
}
