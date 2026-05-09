package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"adplatform/internal/platform/httpapi"
	"adplatform/internal/services/apigateway"
)

func TestBuildDockerCheckerValidationArgs(t *testing.T) {
	args := buildDockerCheckerValidationArgs(apigateway.CheckerValidationRequest{
		ChallengeID:  7,
		Name:         "proxy",
		CheckerImage: "registry.local/proxy-checker:latest",
	})

	expected := []string{
		"run",
		"--rm",
		"--entrypoint",
		"/bin/sh",
		"registry.local/proxy-checker:latest",
		"-lc",
	}
	for _, fragment := range expected {
		if !contains(args, fragment) {
			t.Fatalf("expected fragment %q in args %v", fragment, args)
		}
	}
	if !strings.Contains(args[len(args)-1], "missing standard checker entrypoint inside image") {
		t.Fatalf("expected validation script, got %q", args[len(args)-1])
	}
}

func TestBuildDockerCheckerExecuteArgs(t *testing.T) {
	args := buildDockerCheckerExecuteArgs("adplatform_game_svc_007", apigateway.CheckerExecutionRequest{
		ChallengeID:   7,
		TeamID:        101,
		TeamName:      "Team Alpha",
		ChallengeName: "proxy",
		CheckerImage:  "registry.local/proxy-checker:latest",
		Phase:         "put",
		Target:        "10.80.7.11:10007",
		TargetHost:    "10.80.7.11",
		TargetIP:      "10.80.7.11",
		TargetPort:    10007,
		TickID:        19,
		Flag:          "FLAG{demo}",
		Metadata:      `{"slot":"demo"}`,
	})

	expected := []string{
		"run",
		"--rm",
		"--network",
		"adplatform_game_svc_007",
		"-e",
		"AD_PHASE=put",
		"AD_TEAM_ID=101",
		"AD_TEAM_NAME=Team Alpha",
		"AD_CHALLENGE_ID=7",
		"AD_CHALLENGE_NAME=proxy",
		"AD_TARGET=10.80.7.11:10007",
		"AD_TARGET_HOST=10.80.7.11",
		"AD_TARGET_IP=10.80.7.11",
		"AD_TARGET_PORT=10007",
		"AD_TICK_ID=19",
		"AD_FLAG=FLAG{demo}",
		`AD_METADATA={"slot":"demo"}`,
		"--entrypoint",
		"/bin/sh",
		"registry.local/proxy-checker:latest",
		"-lc",
	}
	for _, fragment := range expected {
		if !contains(args, fragment) {
			t.Fatalf("expected fragment %q in args %v", fragment, args)
		}
	}
	if !strings.Contains(args[len(args)-1], `exec "$checker_exec" "$AD_PHASE"`) {
		t.Fatalf("expected execute script, got %q", args[len(args)-1])
	}
}

func TestDockerCheckerExecutorValidateCheckerRunsProbe(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "docker.log")
	binPath := filepath.Join(t.TempDir(), "docker")
	script := "#!/bin/sh\n" +
		"printf '%s\\n' \"$*\" >> \"$DOCKER_LOG\"\n" +
		"printf 'ADPLATFORM_CHECKER_CAPABILITIES={\"service_state\":true}\\n'\n"
	if err := os.WriteFile(binPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake docker: %v", err)
	}

	executor := &dockerCheckerExecutor{
		binary:        binPath,
		network:       "adplatform_game",
		networkLayout: "per-service",
		timeout:       5 * time.Second,
	}
	t.Setenv("DOCKER_LOG", logPath)

	result, err := executor.ValidateChecker(context.Background(), apigateway.CheckerValidationRequest{
		ChallengeID:  7,
		Name:         "proxy",
		CheckerImage: "registry.local/proxy-checker:latest",
	})
	if err != nil {
		t.Fatalf("validate checker failed: %v", err)
	}
	if result.Status != "valid" || !result.ContractOK || !result.ServiceStateContractOK {
		t.Fatalf("unexpected validation result %+v", result)
	}

	logBytes, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read docker log: %v", err)
	}
	logOutput := string(logBytes)
	if !strings.Contains(logOutput, "run --rm --entrypoint /bin/sh registry.local/proxy-checker:latest -lc") {
		t.Fatalf("unexpected docker log %q", logOutput)
	}
	if !strings.Contains(logOutput, "checker entrypoint does not support validate or --help") {
		t.Fatalf("missing validation script in log %q", logOutput)
	}
}

func TestDockerCheckerExecutorValidateCheckerRejectsMissingServiceStateCapability(t *testing.T) {
	binPath := filepath.Join(t.TempDir(), "docker")
	script := "#!/bin/sh\n" +
		"printf 'ok\\n'\n"
	if err := os.WriteFile(binPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake docker: %v", err)
	}

	executor := &dockerCheckerExecutor{
		binary:        binPath,
		network:       "adplatform_game",
		networkLayout: "per-service",
		timeout:       5 * time.Second,
	}

	result, err := executor.ValidateChecker(context.Background(), apigateway.CheckerValidationRequest{
		ChallengeID:  7,
		Name:         "proxy",
		CheckerImage: "registry.local/proxy-checker:latest",
	})
	if err != nil {
		t.Fatalf("validate checker failed: %v", err)
	}
	if result.Status != "invalid" || result.ContractOK || result.ServiceStateContractOK {
		t.Fatalf("expected missing capability to invalidate checker, got %+v", result)
	}
}

func TestDockerCheckerExecutorExecuteCheckerRunsPhase(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "docker.log")
	binPath := filepath.Join(t.TempDir(), "docker")
	script := "#!/bin/sh\n" +
		"printf '%s\\n' \"$*\" >> \"$DOCKER_LOG\"\n" +
		"printf 'checker ok\\n'\n"
	if err := os.WriteFile(binPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake docker: %v", err)
	}

	executor := &dockerCheckerExecutor{
		binary:        binPath,
		network:       "adplatform_game",
		networkLayout: "per-service",
		timeout:       5 * time.Second,
	}
	t.Setenv("DOCKER_LOG", logPath)

	result, err := executor.ExecuteChecker(context.Background(), apigateway.CheckerExecutionRequest{
		ChallengeID:  7,
		TeamID:       101,
		CheckerImage: "registry.local/proxy-checker:latest",
		Phase:        "check",
		Target:       "10.80.7.11:10007",
		TickID:       19,
	})
	if err != nil {
		t.Fatalf("execute checker failed: %v", err)
	}
	if result.Status != "success" || result.ExitCode != 0 {
		t.Fatalf("unexpected execute result %+v", result)
	}
	if result.Output != "checker ok" {
		t.Fatalf("unexpected checker output %q", result.Output)
	}

	logBytes, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read docker log: %v", err)
	}
	logOutput := string(logBytes)
	if !strings.Contains(logOutput, "--network adplatform_game_svc_007") || !strings.Contains(logOutput, "AD_PHASE=check") || !strings.Contains(logOutput, `exec "$checker_exec" "$AD_PHASE"`) {
		t.Fatalf("unexpected docker log %q", logOutput)
	}
}

func TestDockerCheckerExecutorExecuteCheckerCapturesReportedServiceState(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "docker.log")
	binPath := filepath.Join(t.TempDir(), "docker")
	script := "#!/bin/sh\n" +
		"printf '%s\\n' \"$*\" >> \"$DOCKER_LOG\"\n" +
		"printf 'ADPLATFORM_SERVICE_STATE={\"status\":\"faulty\",\"message\":\"explicit checker status\"}\\n'\n" +
		"printf 'checker ok\\n'\n"
	if err := os.WriteFile(binPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake docker: %v", err)
	}

	executor := &dockerCheckerExecutor{
		binary:        binPath,
		network:       "adplatform_game",
		networkLayout: "per-service",
		timeout:       5 * time.Second,
	}
	t.Setenv("DOCKER_LOG", logPath)

	result, err := executor.ExecuteChecker(context.Background(), apigateway.CheckerExecutionRequest{
		ChallengeID:  7,
		TeamID:       101,
		CheckerImage: "registry.local/proxy-checker:latest",
		Phase:        "check",
		Target:       "10.80.7.11:10007",
		TickID:       19,
	})
	if err != nil {
		t.Fatalf("execute checker failed: %v", err)
	}
	if result.ServiceState != "faulty" {
		t.Fatalf("expected reported faulty service state, got %+v", result)
	}
	if result.StateMessage != "explicit checker status" {
		t.Fatalf("unexpected state message %+v", result)
	}
	if result.Output != "checker ok" {
		t.Fatalf("expected service-state marker stripped from output, got %q", result.Output)
	}
}

func TestCheckerRunnerValidateEndpoint(t *testing.T) {
	server := newCheckerRunnerServer("dev-admin-token", dryRunCheckerExecutor{})
	mux := httpapi.NewBaseMux(httpapi.ServiceInfo{Name: "checker-runner", Version: "dev", Addr: ":0"})
	server.RegisterRoutes(mux)

	body := bytes.NewBufferString(`{"challenge_id":7,"name":"proxy","checker_image":"registry.local/proxy-checker:latest"}`)
	request := httptest.NewRequest(http.MethodPost, "/internal/v1/checkers/validate", body)
	request.Header.Set("Authorization", "Bearer dev-admin-token")
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", response.Code)
	}

	var payload apigateway.CheckerValidationResult
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode validation response: %v", err)
	}
	if payload.Status != "valid" || !payload.ContractOK || !payload.ServiceStateContractOK {
		t.Fatalf("unexpected validation payload %+v", payload)
	}
}

func TestParseCheckerValidationOutputDetectsServiceStateCapability(t *testing.T) {
	output, ok := parseCheckerValidationOutput([]byte("ADPLATFORM_CHECKER_CAPABILITIES={\"service_state\":true}\nok\n"))
	if !ok {
		t.Fatal("expected service-state capability to be detected")
	}
	if output != "ok" {
		t.Fatalf("expected stripped output to be ok, got %q", output)
	}
}

func TestCheckerRunnerExecuteEndpointRejectsInvalidPhase(t *testing.T) {
	server := newCheckerRunnerServer("dev-admin-token", dryRunCheckerExecutor{})
	mux := httpapi.NewBaseMux(httpapi.ServiceInfo{Name: "checker-runner", Version: "dev", Addr: ":0"})
	server.RegisterRoutes(mux)

	body := bytes.NewBufferString(`{"challenge_id":7,"team_id":101,"checker_image":"registry.local/proxy-checker:latest","phase":"boom","target":"10.80.7.11:10007","tick_id":19}`)
	request := httptest.NewRequest(http.MethodPost, "/internal/v1/checkers/execute", body)
	request.Header.Set("Authorization", "Bearer dev-admin-token")
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", response.Code)
	}
}

func contains(values []string, needle string) bool {
	for _, value := range values {
		if value == needle {
			return true
		}
	}
	return false
}
