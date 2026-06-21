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

func TestCheckerRunnerRoutesRequireAdminAuth(t *testing.T) {
	server := newCheckerRunnerServer("dev-admin-token", dryRunCheckerExecutor{})
	mux := httpapi.NewBaseMux(httpapi.ServiceInfo{Name: "checker-runner", Version: "dev", Addr: ":0"})
	server.RegisterRoutes(mux)
	cases := []struct {
		method string
		path   string
		body   string
	}{
		{http.MethodPost, "/internal/v1/checkers/validate", `{}`},
		{http.MethodPost, "/internal/v1/checkers/execute", `{}`},
		{http.MethodPost, "/internal/v1/checkers/execute-batch", `{}`},
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

func TestBuildDockerCheckerValidationArgs(t *testing.T) {
	args := buildDockerCheckerValidationArgs(defaultCheckerSecurity(), apigateway.CheckerValidationRequest{
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
	assertArgPrefix(t, args, []string{"run", "--rm", "--cap-drop", "ALL", "--security-opt", "no-new-privileges:true", "--pids-limit", "128", "--memory", "256m", "--cpus", "0.5", "--entrypoint"})
}

func TestBuildDockerCheckerExecuteArgs(t *testing.T) {
	args := buildDockerCheckerExecuteArgs("adplatform_game_svc_007", "adchk-19-7-101-1", defaultCheckerSecurity(), apigateway.CheckerExecutionRequest{
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
		CheckerToken:  "checker-secret-101-7",
	})

	expected := []string{
		"run",
		"--rm",
		"--name",
		"adchk-19-7-101-1",
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
		"AD_CHECKER_TOKEN=checker-secret-101-7",
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
	if !strings.Contains(logOutput, "--entrypoint /bin/sh registry.local/proxy-checker:latest -lc") {
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

func TestParseCheckerBatchFrames(t *testing.T) {
	output := strings.Join([]string{
		"__ADP_PHASE_BEGIN__ put",
		"put done",
		"__ADP_PHASE_EXIT__ put 0",
		"__ADP_PHASE_BEGIN__ get",
		`ADPLATFORM_SERVICE_STATE={"status":"ok"}`,
		"get failed",
		"__ADP_PHASE_EXIT__ get 1",
	}, "\n")

	frames := parseCheckerBatchFrames(output)
	if len(frames) != 2 {
		t.Fatalf("expected 2 frames, got %d: %v", len(frames), frames)
	}
	if frames["put"].exitCode != 0 || frames["put"].output != "put done" {
		t.Fatalf("unexpected put frame %+v", frames["put"])
	}
	if frames["get"].exitCode != 1 {
		t.Fatalf("expected get exit 1, got %+v", frames["get"])
	}
	if !strings.Contains(frames["get"].output, "get failed") {
		t.Fatalf("expected get output retained, got %q", frames["get"].output)
	}
}

func TestDockerCheckerExecutorExecuteBatchRunsPhasesInOneContainer(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "docker.log")
	binPath := filepath.Join(t.TempDir(), "docker")
	// Fake docker emulates the framed multi-phase output the batch entrypoint emits.
	script := "#!/bin/sh\n" +
		"printf '%s\\n' \"$*\" >> \"$DOCKER_LOG\"\n" +
		"printf '__ADP_PHASE_BEGIN__ put\\nput done\\n__ADP_PHASE_EXIT__ put 0\\n'\n" +
		"printf '__ADP_PHASE_BEGIN__ get\\nADPLATFORM_SERVICE_STATE={\"status\":\"ok\",\"message\":\"healthy\"}\\nget done\\n__ADP_PHASE_EXIT__ get 0\\n'\n"
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

	result, err := executor.ExecuteCheckerBatch(context.Background(), apigateway.CheckerBatchExecutionRequest{
		ChallengeID:  7,
		TeamID:       101,
		CheckerImage: "registry.local/proxy-checker:latest",
		Phases:       []string{"put", "get", "check"},
		Target:       "10.80.7.11:10007",
		TickID:       19,
	})
	if err != nil {
		t.Fatalf("execute batch failed: %v", err)
	}
	if len(result.Phases) != 2 {
		t.Fatalf("expected put+get phases, got %d: %+v", len(result.Phases), result.Phases)
	}
	if result.Phases[0].Phase != "put" || result.Phases[0].Status != "success" {
		t.Fatalf("unexpected put phase %+v", result.Phases[0])
	}
	if result.Phases[1].Phase != "get" || result.Phases[1].ServiceState != "ok" || result.Phases[1].Output != "get done" {
		t.Fatalf("unexpected get phase %+v", result.Phases[1])
	}

	logBytes, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read docker log: %v", err)
	}
	logOutput := string(logBytes)
	// One container invocation only, carrying every phase via AD_PHASES.
	if strings.Count(logOutput, "--network") != 1 {
		t.Fatalf("expected exactly one docker run, got log %q", logOutput)
	}
	if !strings.Contains(logOutput, "AD_PHASES=put get check") {
		t.Fatalf("expected AD_PHASES env, got %q", logOutput)
	}
}

func TestDockerCheckerExecutorExecuteBatchHaltsOnFailure(t *testing.T) {
	binPath := filepath.Join(t.TempDir(), "docker")
	script := "#!/bin/sh\n" +
		"printf '__ADP_PHASE_BEGIN__ put\\nboom\\n__ADP_PHASE_EXIT__ put 1\\n'\n"
	if err := os.WriteFile(binPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake docker: %v", err)
	}

	executor := &dockerCheckerExecutor{
		binary:        binPath,
		network:       "adplatform_game",
		networkLayout: "per-service",
		timeout:       5 * time.Second,
	}

	result, err := executor.ExecuteCheckerBatch(context.Background(), apigateway.CheckerBatchExecutionRequest{
		ChallengeID:  7,
		TeamID:       101,
		CheckerImage: "registry.local/proxy-checker:latest",
		Phases:       []string{"put", "get", "check"},
		Target:       "10.80.7.11:10007",
		TickID:       19,
	})
	if err != nil {
		t.Fatalf("execute batch failed: %v", err)
	}
	// Only the failed put is reported; get/check are absent so the caller marks
	// them skipped.
	if len(result.Phases) != 1 || result.Phases[0].Phase != "put" || result.Phases[0].Status != "failed" {
		t.Fatalf("expected single failed put phase, got %+v", result.Phases)
	}
}

func TestDockerCheckerExecutorForceRemovesTimedOutContainer(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "docker.log")
	binPath := filepath.Join(t.TempDir(), "docker")
	// `run` hangs so the per-call context times out; `rm` returns immediately.
	script := "#!/bin/sh\n" +
		"printf '%s\\n' \"$*\" >> \"$DOCKER_LOG\"\n" +
		"case \"$1\" in run) sleep 5 ;; *) exit 0 ;; esac\n"
	if err := os.WriteFile(binPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake docker: %v", err)
	}

	executor := &dockerCheckerExecutor{
		binary:        binPath,
		network:       "adplatform_game",
		networkLayout: "per-service",
		timeout:       100 * time.Millisecond,
	}
	t.Setenv("DOCKER_LOG", logPath)

	result, err := executor.ExecuteCheckerBatch(context.Background(), apigateway.CheckerBatchExecutionRequest{
		ChallengeID:  7,
		TeamID:       101,
		CheckerImage: "registry.local/proxy-checker:latest",
		Phases:       []string{"put", "get", "check"},
		Target:       "10.80.7.11:10007",
		TickID:       19,
	})
	if err != nil {
		t.Fatalf("execute batch failed: %v", err)
	}
	if len(result.Phases) != 1 || result.Phases[0].Status != "failed" {
		t.Fatalf("expected timed-out batch to report a failed phase, got %+v", result.Phases)
	}

	logBytes, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read docker log: %v", err)
	}
	if !strings.Contains(string(logBytes), "rm -f adchkb-19-7-101-") {
		t.Fatalf("expected force-remove of leaked container, got log %q", string(logBytes))
	}
}

func TestCheckerRunnerExecuteBatchEndpoint(t *testing.T) {
	server := newCheckerRunnerServer("dev-admin-token", dryRunCheckerExecutor{})
	mux := httpapi.NewBaseMux(httpapi.ServiceInfo{Name: "checker-runner", Version: "dev", Addr: ":0"})
	server.RegisterRoutes(mux)

	body := bytes.NewBufferString(`{"challenge_id":7,"team_id":101,"checker_image":"registry.local/proxy-checker:latest","phases":["put","get","check"],"target":"10.80.7.11:10007","tick_id":19}`)
	request := httptest.NewRequest(http.MethodPost, "/internal/v1/checkers/execute-batch", body)
	request.Header.Set("Authorization", "Bearer dev-admin-token")
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", response.Code, response.Body.String())
	}
	var payload apigateway.CheckerBatchExecutionResult
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode batch response: %v", err)
	}
	if len(payload.Phases) != 3 {
		t.Fatalf("expected 3 phases, got %+v", payload.Phases)
	}
}

func TestCheckerRunnerExecuteBatchEndpointRejectsInvalidPhase(t *testing.T) {
	server := newCheckerRunnerServer("dev-admin-token", dryRunCheckerExecutor{})
	mux := httpapi.NewBaseMux(httpapi.ServiceInfo{Name: "checker-runner", Version: "dev", Addr: ":0"})
	server.RegisterRoutes(mux)

	body := bytes.NewBufferString(`{"challenge_id":7,"team_id":101,"checker_image":"registry.local/proxy-checker:latest","phases":["put","boom"],"target":"10.80.7.11:10007","tick_id":19}`)
	request := httptest.NewRequest(http.MethodPost, "/internal/v1/checkers/execute-batch", body)
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

func assertArgPrefix(t *testing.T, args []string, expected []string) {
	t.Helper()
	if len(expected) > len(args) {
		t.Fatalf("expected prefix %v in args %v", expected, args)
	}
	for i := range expected {
		if args[i] != expected[i] {
			t.Fatalf("expected args prefix %v, got %v", expected, args[:len(expected)])
		}
	}
}
