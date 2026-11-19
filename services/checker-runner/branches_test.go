package main

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"adplatform/internal/platform/httpapi"
	"adplatform/internal/services/apigateway"
)

// fakeDocker writes an executable stand-in for the docker CLI.
func fakeDocker(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "docker")
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"+body), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestCheckerConfigHelpers(t *testing.T) {
	t.Setenv("CHK_CSV", " a, ,b ")
	if got := checkerCSVConfig("CHK_CSV", ""); strings.Join(got, ",") != "a,b" {
		t.Fatalf("csv: %v", got)
	}
	for _, disabled := range []string{"", "none", "DISABLED"} {
		t.Setenv("CHK_CSV", disabled)
		t.Setenv("CHK_STR", disabled)
		if got := checkerCSVConfig("CHK_CSV", ""); got != nil {
			t.Fatalf("%q must disable the list, got %v", disabled, got)
		}
		if got := checkerStringConfig("CHK_STR", ""); got != "" {
			t.Fatalf("%q must disable the value, got %q", disabled, got)
		}
	}
	t.Setenv("CHK_STR", " 64m ")
	if checkerStringConfig("CHK_STR", "") != "64m" {
		t.Fatal("string config must be trimmed")
	}
	args := checkerDockerSecurity{capDrop: []string{"ALL"}, capAdd: []string{"NET_RAW"}}.dockerArgs()
	if strings.Join(args, " ") != "--cap-drop ALL --cap-add NET_RAW" {
		t.Fatalf("security args: %v", args)
	}
}

func TestNewCheckerExecutorModes(t *testing.T) {
	t.Setenv("CHECKER_RUNNER_MODE", "docker")
	t.Setenv("CHECKER_RUNNER_DOCKER_NETWORK", "")
	t.Setenv("CONTROLLER_DOCKER_NETWORK", "fallback_net")
	executor, ok := newCheckerExecutor().(*dockerCheckerExecutor)
	if !ok || executor.network != "fallback_net" || executor.binary == "" {
		t.Fatalf("docker mode: %+v", executor)
	}
	t.Setenv("CHECKER_RUNNER_DOCKER_NETWORK", "checker_net")
	if executor := newCheckerExecutor().(*dockerCheckerExecutor); executor.network != "checker_net" {
		t.Fatalf("explicit network ignored: %q", executor.network)
	}
	t.Setenv("CHECKER_RUNNER_MODE", "dry-run")
	if _, ok := newCheckerExecutor().(dryRunCheckerExecutor); !ok {
		t.Fatal("dry-run mode must use the dry-run executor")
	}
}

type failingCheckerExecutor struct{}

func (failingCheckerExecutor) ValidateChecker(context.Context, apigateway.CheckerValidationRequest) (apigateway.CheckerValidationResult, error) {
	return apigateway.CheckerValidationResult{}, errors.New("validate exploded")
}

func (failingCheckerExecutor) ExecuteChecker(context.Context, apigateway.CheckerExecutionRequest) (apigateway.CheckerExecutionResult, error) {
	return apigateway.CheckerExecutionResult{}, errors.New("execute exploded")
}

func (failingCheckerExecutor) ExecuteCheckerBatch(context.Context, apigateway.CheckerBatchExecutionRequest) (apigateway.CheckerBatchExecutionResult, error) {
	return apigateway.CheckerBatchExecutionResult{}, errors.New("batch exploded")
}

func TestCheckerRunnerHandlerFailures(t *testing.T) {
	mux := httpapi.NewBaseMux(httpapi.ServiceInfo{Name: "checker-runner", Version: "test", Addr: ":0"})
	newCheckerRunnerServer("runner-token", failingCheckerExecutor{}).RegisterRoutes(mux)
	call := func(path, body string) int {
		request := httptest.NewRequest(http.MethodPost, path, bytes.NewBufferString(body))
		request.Header.Set("Authorization", "Bearer runner-token")
		recorder := httptest.NewRecorder()
		mux.ServeHTTP(recorder, request)
		return recorder.Code
	}
	if code := call("/internal/v1/checkers/validate", `not json`); code != http.StatusBadRequest {
		t.Fatalf("bad validate body: %d", code)
	}
	if code := call("/internal/v1/checkers/validate", `{"challenge_id":1,"checker_image":"img"}`); code != http.StatusInternalServerError {
		t.Fatalf("validate failure: %d", code)
	}
	if code := call("/internal/v1/checkers/execute", `{"challenge_id":1,"team_id":1,"checker_image":"img","phase":"put","target":"10.80.1.2:1"}`); code != http.StatusInternalServerError {
		t.Fatalf("execute failure: %d", code)
	}
	if code := call("/internal/v1/checkers/execute-batch", `{"challenge_id":1,"team_id":1,"checker_image":"img","phases":["put"],"target":"10.80.1.2:1"}`); code != http.StatusInternalServerError {
		t.Fatalf("batch failure: %d", code)
	}
}

func TestDockerValidateCheckerBranches(t *testing.T) {
	executor := &dockerCheckerExecutor{binary: fakeDocker(t, "echo nope; exit 4\n"), timeout: 5 * time.Second}
	result, err := executor.ValidateChecker(context.Background(), apigateway.CheckerValidationRequest{ChallengeID: 1, CheckerImage: "  "})
	if err != nil || result.Status != "invalid" || !strings.Contains(result.Message, "required") {
		t.Fatalf("blank image: %+v %v", result, err)
	}
	result, err = executor.ValidateChecker(context.Background(), apigateway.CheckerValidationRequest{ChallengeID: 1, CheckerImage: "img"})
	if err != nil || result.Status != "invalid" || result.Message != "nope" {
		t.Fatalf("failing probe: %+v %v", result, err)
	}
}

func TestDockerExecuteCheckerBranches(t *testing.T) {
	executor := &dockerCheckerExecutor{binary: fakeDocker(t, "case \"$1\" in run) exec sleep 5 ;; *) exit 0 ;; esac\n"), network: "adplatform_game", networkLayout: "per-service", timeout: 5 * time.Second}
	request := apigateway.CheckerExecutionRequest{ChallengeID: 1, TeamID: 2, CheckerImage: "img", Phase: "put", Target: "10.80.1.2:1001", TimeoutSeconds: 1}
	started := time.Now()
	result, err := executor.ExecuteChecker(context.Background(), request)
	if err != nil || result.Status != "failed" || time.Since(started) > 4*time.Second {
		t.Fatalf("per-request timeout must apply: %+v %v after %s", result, err, time.Since(started))
	}
	request.Target = "not-an-ip:1"
	if _, err := executor.ExecuteChecker(context.Background(), request); err == nil {
		t.Fatal("an unroutable target must be rejected")
	}
	if _, err := executor.ExecuteCheckerBatch(context.Background(), apigateway.CheckerBatchExecutionRequest{ChallengeID: 1, TeamID: 2, CheckerImage: "img", Phases: []string{"put"}, Target: "not-an-ip:1"}); err == nil {
		t.Fatal("an unroutable batch target must be rejected")
	}
	executor.forceRemove("  ")
}

func TestDockerExecuteBatchBranches(t *testing.T) {
	frame := testCheckerBatchFrame("put", "stored", 0)
	executor := &dockerCheckerExecutor{binary: fakeDocker(t, "printf '%s' '"+frame+"'\n"), network: "adplatform_game", networkLayout: "per-service", timeout: 5 * time.Second}
	result, err := executor.ExecuteCheckerBatch(context.Background(), apigateway.CheckerBatchExecutionRequest{ChallengeID: 1, TeamID: 2, CheckerImage: "img", Phases: []string{"put", "get"}, Target: "10.80.1.2:1001", TimeoutSeconds: 2})
	if err != nil || len(result.Phases) != 1 || result.Phases[0].Phase != "put" {
		t.Fatalf("a missing later frame must stop the batch after put: %+v %v", result, err)
	}

	flood := &dockerCheckerExecutor{binary: fakeDocker(t, "head -c 2000000 /dev/zero\n"), network: "adplatform_game", networkLayout: "per-service", timeout: 5 * time.Second}
	if _, _, err := flood.execDocker(context.Background(), "run"); !errors.Is(err, errCheckerOutputLimit) {
		t.Fatalf("oversized output must hit the limit, got %v", err)
	}
	if _, _, err := flood.execDocker(context.Background()); !errors.Is(err, errCheckerOutputLimit) {
		t.Fatalf("argument-less exec must still report the limit, got %v", err)
	}
}

func TestRunCommandWithBoundedOutputRejectsNonPositiveLimit(t *testing.T) {
	if _, _, err := runCommandWithBoundedOutput(exec.Command("true"), 0); err == nil {
		t.Fatal("a zero limit must be rejected")
	}
}

func TestParseCheckerBatchFramesRejectsForgedFraming(t *testing.T) {
	for name, output := range map[string]string{
		"empty":             "",
		"no header newline": "__ADP_PHASE_FRAME__ put 0 1",
		"wrong prefix":      "FRAME put 0 1\nx\n",
		"bad phase":         "__ADP_PHASE_FRAME__ destroy 0 1\nx\n",
		"too few fields":    "__ADP_PHASE_FRAME__ put 0\nx\n",
		"duplicate phase":   testCheckerBatchFrame("put", "a", 0) + testCheckerBatchFrame("put", "b", 0),
		"bad exit code":     "__ADP_PHASE_FRAME__ put 999 1\nx\n",
		"negative exit":     "__ADP_PHASE_FRAME__ put -1 1\nx\n",
		"missing delimiter": "__ADP_PHASE_FRAME__ put 0 1\nxy",
		"length overflow":   "__ADP_PHASE_FRAME__ put 0 50\nx\n",
	} {
		if frames, err := parseCheckerBatchFrames(output); err == nil {
			t.Errorf("%s: forged framing was accepted: %+v", name, frames)
		}
	}
}

func TestTruncateOutputKeepsValidUTF8(t *testing.T) {
	if got := truncateOutput("short", 10); got != "short" {
		t.Fatal(got)
	}
	got := truncateOutput("ab€cd", 3) // the euro sign spans bytes 2-4
	if got != "ab\n... [truncated]" {
		t.Fatalf("truncation split a rune: %q", got)
	}
}

func TestCommandFailureMessageBranches(t *testing.T) {
	cases := []struct {
		err      error
		output   string
		exitCode int
		want     string
	}{
		{errCheckerOutputLimit, "x", 1, "output exceeded limit"},
		{errors.New("x"), "", 3, "exited with status 3"},
		{context.DeadlineExceeded, "", -1, "timed out"},
		{context.Canceled, "", -1, "canceled"},
		{errors.New("x"), "", -1, "checker command failed"},
		{nil, "  AD_FLAG=secret tail", -1, "AD_FLAG=[redacted] tail"},
	}
	for _, tc := range cases {
		if got := commandFailureMessage(tc.err, []byte(tc.output), tc.exitCode); !strings.Contains(got, tc.want) {
			t.Errorf("commandFailureMessage(%v, %q, %d) = %q, want %q", tc.err, tc.output, tc.exitCode, got, tc.want)
		}
	}
	if redactCheckerSecretMaterial("") != "" {
		t.Fatal("empty message")
	}
}

func TestParseCheckerOutputBranches(t *testing.T) {
	output := strings.Join([]string{
		"line one",
		"ADPLATFORM_SERVICE_STATE=",
		"ADPLATFORM_SERVICE_STATE={broken",
		"ADPLATFORM_SERVICE_STATE=mystery",
		"ADPLATFORM_SERVICE_STATE=ok",
		`ADPLATFORM_SERVICE_STATE={"status":"faulty","message":"later lines cannot override"}`,
	}, "\n")
	filtered, status, message := parseCheckerServiceStateOutput([]byte(output), true)
	if status != "ok" || message != "" {
		t.Fatalf("first valid verdict must win, got %q %q", status, message)
	}
	if !strings.Contains(filtered, "{broken") || !strings.Contains(filtered, "mystery") || strings.Contains(filtered, "=ok") {
		t.Fatalf("unparseable markers stay in output, valid ones are removed: %q", filtered)
	}
	if _, status, _ := parseCheckerServiceStateOutput([]byte("ADPLATFORM_SERVICE_STATE=ok"), false); status != "" {
		t.Fatal("reported state must be ignored when not allowed")
	}
	filtered, ok := parseCheckerValidationOutput([]byte("hello\nADPLATFORM_CHECKER_CAPABILITIES=\nADPLATFORM_CHECKER_CAPABILITIES={bad\n"))
	if ok || !strings.Contains(filtered, "{bad") {
		t.Fatalf("invalid capability lines: %q %v", filtered, ok)
	}
	if firstNonEmpty(" ", "") != "" {
		t.Fatal("firstNonEmpty of blanks")
	}
}

func TestNormalizeCheckerPhasesRejectsEmpty(t *testing.T) {
	if phases, ok := normalizeCheckerPhases(nil); ok || phases != nil {
		t.Fatalf("an empty phase list must be rejected, got %v %v", phases, ok)
	}
}
