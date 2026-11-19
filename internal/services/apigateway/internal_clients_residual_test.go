package apigateway

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// truncatedBodyServer declares a longer body than it sends, so reading the
// response body fails.
func truncatedBodyServer(t *testing.T, status int) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Length", "100")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(`{"ti`))
	}))
	t.Cleanup(server.Close)
	return server
}

func closedServerURL(t *testing.T) string {
	t.Helper()
	server := httptest.NewServer(http.NotFoundHandler())
	server.Close()
	return server.URL
}

func TestHTTPControllerClientCallsEveryEndpoint(t *testing.T) {
	ctx := context.Background()
	if _, ok := NewHTTPControllerClient("", "x").(noopControllerClient); !ok {
		t.Fatal("a blank base URL yields the noop client")
	}
	server, requests := newRecordingServer(t, http.StatusOK, `{"state":"ready"}`)
	client := NewHTTPControllerClient(server.URL, " controller-token ").(*httpControllerClient)
	calls := map[string]func() error{
		"POST /internal/v1/teams/7/services/3/reset/factory":    func() error { return client.FactoryResetService(ctx, 7, 3) },
		"POST /internal/v1/teams/7/services/3/restart":          func() error { return client.RestartService(ctx, 7, 3) },
		"POST /internal/v1/teams/7/services/3/access/reconcile": func() error { return client.ReconcileServiceAccess(ctx, 7, 3) },
		"POST /internal/v1/teams/7/services/3/ssh-credential":   func() error { return client.ApplySSHCredential(ctx, 7, 3, ControllerSSHCredential{Password: "p"}) },
		"POST /internal/v1/access/teardown":                     func() error { return client.TeardownAccessPolicies(ctx) },
		"POST /internal/v1/teams/7/services/3/remove":           func() error { return client.RemoveService(ctx, 7, 3) },
		"POST /internal/v1/teams/7/remove":                      func() error { return client.RemoveTeamServices(ctx, 7) },
		"POST /internal/v1/challenges/3/remove":                 func() error { return client.RemoveChallengeServices(ctx, 3) },
		"POST /internal/v1/deployments/reconcile":               func() error { _, err := client.ReconcileDeployments(ctx); return err },
		"POST /internal/v1/challenges/validate": func() error {
			_, err := client.ValidateChallengeRuntime(ctx, ChallengeValidationRequest{ChallengeID: 3})
			return err
		},
		"GET /internal/v1/access/status":     func() error { _, err := client.AccessStatus(ctx); return err },
		"POST /internal/v1/access/reconcile": func() error { _, err := client.ReconcileAccessPolicies(ctx); return err },
	}
	for name, call := range calls {
		*requests = nil
		if err := call(); err != nil {
			t.Errorf("%s: %v", name, err)
			continue
		}
		got := (*requests)[0]
		if got.method+" "+got.path != name || got.auth != "Bearer controller-token" {
			t.Errorf("%s: sent %s %s auth %q", name, got.method, got.path, got.auth)
		}
	}
}

func TestHTTPControllerClientFailures(t *testing.T) {
	ctx := context.Background()
	for _, tc := range upstreamErrorCases {
		server, _ := newRecordingServer(t, http.StatusBadGateway, tc.body)
		client := NewHTTPControllerClient(server.URL, "").(*httpControllerClient)
		if err := client.RestartService(ctx, 1, 1); err == nil || !strings.Contains(err.Error()+" status 502", tc.want) {
			t.Errorf("post %s: %v", tc.name, err)
		}
		if _, err := client.AccessStatus(ctx); err == nil || !strings.Contains(err.Error()+" status 502", tc.want) {
			t.Errorf("json %s: %v", tc.name, err)
		}
	}

	notFound, _ := newRecordingServer(t, http.StatusNotFound, `{"title":"gone"}`)
	if err := NewHTTPControllerClient(notFound.URL, "").RestartService(ctx, 1, 1); !errors.Is(err, ErrChallengeNotFound) {
		t.Errorf("404 must map to challenge not found: %v", err)
	}

	resetHeader := func(status int, predelete string) error {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("X-ADPlatform-Reset-Predelete", predelete)
			w.WriteHeader(status)
			_, _ = w.Write([]byte(`{"title":"reset","detail":"refused"}`))
		}))
		defer server.Close()
		return NewHTTPControllerClient(server.URL, "").FactoryResetService(ctx, 1, 1)
	}
	if err := resetHeader(http.StatusNotFound, "clean"); !errors.Is(err, errResetPreflight) || !errors.Is(err, ErrChallengeNotFound) {
		t.Errorf("clean 404 reset: %v", err)
	}
	if err := resetHeader(http.StatusConflict, "clean"); !errors.Is(err, errResetPreflight) {
		t.Errorf("clean reset rejection: %v", err)
	}
	if err := resetHeader(http.StatusConflict, "runtime-changed"); !errors.Is(err, errResetBeforeVolume) {
		t.Errorf("runtime-changed reset: %v", err)
	}
	var problem *controllerProblemError
	if err := resetHeader(http.StatusConflict, ""); !errors.As(err, &problem) || problem.StatusCode() != http.StatusConflict {
		t.Errorf("plain reset failure: %v", err)
	}

	badJSON, _ := newRecordingServer(t, http.StatusOK, `not json`)
	if _, err := NewHTTPControllerClient(badJSON.URL, "").AccessStatus(ctx); err == nil {
		t.Error("an undecodable success body must fail")
	}
	for _, status := range []int{http.StatusOK, http.StatusBadGateway} {
		truncated := NewHTTPControllerClient(truncatedBodyServer(t, status).URL, "")
		if status != http.StatusOK {
			if err := truncated.RestartService(ctx, 1, 1); err == nil {
				t.Errorf("a truncated %d problem must fail", status)
			}
		}
		if _, err := truncated.AccessStatus(ctx); err == nil {
			t.Errorf("a truncated %d body must fail", status)
		}
	}

	down := &httpControllerClient{baseURL: closedServerURL(t), client: http.DefaultClient}
	if err := down.RestartService(ctx, 1, 1); err == nil {
		t.Error("an unreachable controller must fail posts")
	}
	if _, err := down.AccessStatus(ctx); err == nil {
		t.Error("an unreachable controller must fail reads")
	}
	malformed := &httpControllerClient{baseURL: "http://bad host", client: http.DefaultClient}
	if err := malformed.RestartService(ctx, 1, 1); err == nil {
		t.Error("an unbuildable request must fail posts")
	}
	if _, err := malformed.AccessStatus(ctx); err == nil {
		t.Error("an unbuildable request must fail reads")
	}
	if err := malformed.postJSON(ctx, "/x", make(chan int)); err == nil {
		t.Error("an unencodable body must fail posts")
	}
	if _, err := requestControllerJSON[ControllerAccessStatus](ctx, malformed, http.MethodPost, "/x", make(chan int)); err == nil {
		t.Error("an unencodable body must fail reads")
	}
}

func TestControllerProblemErrorText(t *testing.T) {
	cases := []struct {
		err                  controllerProblemError
		message, title, body string
	}{
		{controllerProblemError{statusCode: 409, title: "T", detail: "D"}, "D", "T", "D"},
		{controllerProblemError{statusCode: 409, title: "T"}, "T", "T", "T"},
		{controllerProblemError{statusCode: 409}, "controller request failed with status 409", "Controller request failed", "Controller request failed"},
	}
	for _, tc := range cases {
		if tc.err.Error() != tc.message || tc.err.Title() != tc.title || tc.err.Detail() != tc.body {
			t.Errorf("%+v: %q %q %q", tc.err, tc.err.Error(), tc.err.Title(), tc.err.Detail())
		}
	}
}

func TestNoopControllerClientIsInert(t *testing.T) {
	ctx := context.Background()
	client := noopControllerClient{}
	if _, err := client.ReconcileDeployments(ctx); !errors.Is(err, errControllerDisabled) {
		t.Errorf("noop reconcile: %v", err)
	}
	for _, err := range []error{
		client.FactoryResetService(ctx, 1, 1), client.RestartService(ctx, 1, 1), client.ReconcileServiceAccess(ctx, 1, 1),
		client.ApplySSHCredential(ctx, 1, 1, ControllerSSHCredential{}), client.TeardownAccessPolicies(ctx),
		client.RemoveService(ctx, 1, 1), client.RemoveTeamServices(ctx, 1), client.RemoveChallengeServices(ctx, 1),
	} {
		if err != nil {
			t.Errorf("noop action: %v", err)
		}
	}
	if result, err := client.ValidateChallengeRuntime(ctx, ChallengeValidationRequest{ChallengeID: 4, Name: "n"}); err != nil || result.Status != "unavailable" || result.ChallengeID != 4 {
		t.Errorf("noop validation: %+v %v", result, err)
	}
	for _, read := range []func(context.Context) (ControllerAccessStatus, error){client.AccessStatus, client.ReconcileAccessPolicies} {
		if status, err := read(ctx); err != nil || status.State != "disabled" {
			t.Errorf("noop access: %+v %v", status, err)
		}
	}
}

func TestHTTPWireGuardClient(t *testing.T) {
	ctx := context.Background()
	if _, ok := NewHTTPWireGuardClient("", "x").(noopWireGuardClient); !ok {
		t.Fatal("a blank base URL yields the noop client")
	}
	noop := noopWireGuardClient{}
	if status, err := noop.Status(ctx); err != nil || status.State != "disabled" {
		t.Errorf("noop status: %+v %v", status, err)
	}
	if _, err := noop.Reconcile(ctx); !errors.Is(err, errWireGuardGatewayDisabled) {
		t.Errorf("noop reconcile: %v", err)
	}
	if err := noop.Teardown(ctx); err != nil {
		t.Errorf("noop teardown: %v", err)
	}

	server, requests := newRecordingServer(t, http.StatusOK, `{"state":"ready","mode":"wg"}`)
	client := NewHTTPWireGuardClient(server.URL, " wg-token ")
	if status, err := client.Status(ctx); err != nil || status.State != "ready" {
		t.Errorf("status: %+v %v", status, err)
	}
	if _, err := client.Reconcile(ctx); err != nil {
		t.Errorf("reconcile: %v", err)
	}
	if err := client.Teardown(ctx); err != nil {
		t.Errorf("teardown: %v", err)
	}
	for i, want := range []string{"GET /internal/v1/wireguard/status", "POST /internal/v1/wireguard/reconcile", "POST /internal/v1/wireguard/teardown"} {
		got := (*requests)[i]
		if got.method+" "+got.path != want || got.auth != "Bearer wg-token" || (got.method == http.MethodPost) != (got.ctype == "application/json") {
			t.Errorf("request %d: %+v", i, got)
		}
	}

	noContent, _ := newRecordingServer(t, http.StatusNoContent, "")
	if status, err := NewHTTPWireGuardClient(noContent.URL, "").Reconcile(ctx); err != nil || status.State != "" {
		t.Errorf("204: %+v %v", status, err)
	}
	for _, tc := range upstreamErrorCases {
		failing, _ := newRecordingServer(t, http.StatusBadGateway, tc.body)
		if _, err := NewHTTPWireGuardClient(failing.URL, "").Status(ctx); err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: %v", tc.name, err)
		}
	}
	badJSON, _ := newRecordingServer(t, http.StatusOK, `not json`)
	if _, err := NewHTTPWireGuardClient(badJSON.URL, "").Status(ctx); err == nil {
		t.Error("an undecodable success body must fail")
	}
	if _, err := NewHTTPWireGuardClient(truncatedBodyServer(t, http.StatusBadGateway).URL, "").Status(ctx); err == nil || !strings.Contains(err.Error(), "status 502") {
		t.Errorf("a truncated problem body: %v", err)
	}
	if _, err := (&httpWireGuardClient{baseURL: closedServerURL(t), client: http.DefaultClient}).Status(ctx); err == nil {
		t.Error("an unreachable gateway must fail")
	}
	if _, err := (&httpWireGuardClient{baseURL: "http://bad host", client: http.DefaultClient}).Status(ctx); err == nil {
		t.Error("an unbuildable request must fail")
	}
}
