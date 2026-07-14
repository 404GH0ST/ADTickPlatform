package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
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

func TestCommandFailureMessageNeverReturnsSecretArgv(t *testing.T) {
	secretErr := errors.New(`docker run -e AD_FLAG=PLAYIT{secret} -e AD_CHECKER_TOKEN=tok failed: signal: killed`)
	msg := commandFailureMessage(secretErr, nil, -1)
	if strings.Contains(msg, "AD_FLAG") || strings.Contains(msg, "AD_CHECKER_TOKEN") || strings.Contains(msg, "PLAYIT{") {
		t.Fatalf("failure message leaked secrets: %q", msg)
	}
	if msg != "checker command failed" {
		t.Fatalf("unexpected generic failure message %q", msg)
	}

	redacted := commandFailureMessage(nil, []byte("put failed AD_FLAG=PLAYIT{x} AD_CHECKER_TOKEN=abc"), 1)
	if strings.Contains(redacted, "PLAYIT{x}") || strings.Contains(redacted, "abc") {
		t.Fatalf("expected redacted output, got %q", redacted)
	}
	if !strings.Contains(redacted, "AD_FLAG=[redacted]") || !strings.Contains(redacted, "AD_CHECKER_TOKEN=[redacted]") {
		t.Fatalf("expected redaction markers, got %q", redacted)
	}

	overflow := commandFailureMessage(errCheckerOutputLimit, []byte("attacker-controlled output"), -1)
	if overflow != "checker command output exceeded limit." {
		t.Fatalf("unexpected output-limit failure message %q", overflow)
	}
}

func TestRunCommandWithBoundedOutputDiscardsOverflow(t *testing.T) {
	binPath := filepath.Join(t.TempDir(), "docker")
	if err := os.WriteFile(binPath, []byte("#!/bin/sh\nprintf '0123456789abcdefEXTRA'\n"), 0o755); err != nil {
		t.Fatalf("write fake docker: %v", err)
	}

	output, exceeded, err := runCommandWithBoundedOutput(exec.Command(binPath), 16)
	if err != nil {
		t.Fatalf("run fake docker: %v", err)
	}
	if !exceeded {
		t.Fatal("expected output limit to be exceeded")
	}
	if string(output) != "0123456789abcdef" {
		t.Fatalf("expected bounded output, got %q", output)
	}

	exactOutput, exactExceeded, err := runCommandWithBoundedOutput(
		exec.Command("/bin/sh", "-c", "printf '0123456789abcdef'"),
		16,
	)
	if err != nil {
		t.Fatalf("run exact-limit command: %v", err)
	}
	if exactExceeded || string(exactOutput) != "0123456789abcdef" {
		t.Fatalf("exact-limit output was rejected: exceeded=%t output=%q", exactExceeded, exactOutput)
	}
}

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

func TestDockerCheckerExecutorExecuteCheckerIgnoresReportedStateOnFailure(t *testing.T) {
	binPath := filepath.Join(t.TempDir(), "docker")
	script := "#!/bin/sh\n" +
		"printf 'ADPLATFORM_SERVICE_STATE={\"status\":\"ok\",\"message\":\"forged\"}\\n'\n" +
		"printf 'checker failed\\n'\n" +
		"exit 1\n"
	if err := os.WriteFile(binPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake docker: %v", err)
	}

	executor := &dockerCheckerExecutor{
		binary:        binPath,
		network:       "adplatform_game",
		networkLayout: "per-service",
		timeout:       5 * time.Second,
	}
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
	if result.Status != "failed" || result.ExitCode == 0 {
		t.Fatalf("expected failed checker result, got %+v", result)
	}
	if result.ServiceState != "" || result.StateMessage != "" {
		t.Fatalf("failed checker supplied authoritative service state: %+v", result)
	}
	if result.Output != "checker failed" {
		t.Fatalf("expected marker stripped from failed output, got %q", result.Output)
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

func TestCheckerRunnerExecuteEndpointNormalizesPhase(t *testing.T) {
	server := newCheckerRunnerServer("dev-admin-token", dryRunCheckerExecutor{})
	mux := httpapi.NewBaseMux(httpapi.ServiceInfo{Name: "checker-runner", Version: "dev", Addr: ":0"})
	server.RegisterRoutes(mux)

	body := bytes.NewBufferString(`{"challenge_id":7,"team_id":101,"checker_image":"registry.local/proxy-checker:latest","phase":" CHECK ","target":"10.80.7.11:10007","tick_id":19}`)
	request := httptest.NewRequest(http.MethodPost, "/internal/v1/checkers/execute", body)
	request.Header.Set("Authorization", "Bearer dev-admin-token")
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", response.Code, response.Body.String())
	}
	var payload apigateway.CheckerExecutionResult
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode checker response: %v", err)
	}
	if payload.Phase != "check" {
		t.Fatalf("expected normalized check phase, got %q", payload.Phase)
	}
}

func TestParseCheckerBatchFrames(t *testing.T) {
	output := testCheckerBatchFrame("put", "put done", 0) +
		testCheckerBatchFrame("get", "ADPLATFORM_SERVICE_STATE={\"status\":\"ok\"}\nget failed", 1)

	frames, err := parseCheckerBatchFrames(output)
	if err != nil {
		t.Fatalf("parse checker frames: %v", err)
	}
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

func TestParseCheckerBatchFramesTreatsControlTextAsPayload(t *testing.T) {
	payload := strings.Join([]string{
		"service response follows",
		"__ADP_PHASE_FRAME__ check 0 0",
		"__ADP_PHASE_EXIT__ check 0",
		"checker detected a failure",
	}, "\n")

	frames, err := parseCheckerBatchFrames(testCheckerBatchFrame("check", payload, 1))
	if err != nil {
		t.Fatalf("parse checker frames: %v", err)
	}
	frame := frames["check"]
	if frame.exitCode != 1 {
		t.Fatalf("control text inside payload forged success: %+v", frame)
	}
	if frame.output != payload {
		t.Fatalf("payload changed during parsing: %q", frame.output)
	}
}

func TestParseCheckerBatchFramesRejectsMalformedLength(t *testing.T) {
	if _, err := parseCheckerBatchFrames("__ADP_PHASE_FRAME__ check 0 100\nshort\n"); err == nil {
		t.Fatal("expected malformed frame length to fail closed")
	}
}

func TestCheckerBatchEntrypointKeepsControlTextInsidePayload(t *testing.T) {
	binDir := t.TempDir()
	checkerPath := filepath.Join(binDir, "checker")
	checkerScript := "#!/bin/sh\n" +
		"printf 'service response\\n__ADP_PHASE_FRAME__ check 0 0\\n__ADP_PHASE_EXIT__ check 0\\n'\n" +
		"exit 1\n"
	if err := os.WriteFile(checkerPath, []byte(checkerScript), 0o755); err != nil {
		t.Fatalf("write fake checker: %v", err)
	}

	cmd := exec.Command("/bin/sh", "-c", checkerEntrypointBatchExecuteScript())
	cmd.Env = append(os.Environ(), "PATH="+binDir+":"+os.Getenv("PATH"), "AD_PHASES=check")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("run checker batch entrypoint: %v: %s", err, output)
	}

	frames, err := parseCheckerBatchFrames(string(output))
	if err != nil {
		t.Fatalf("parse checker batch output: %v: %q", err, output)
	}
	frame := frames["check"]
	if frame.exitCode != 1 {
		t.Fatalf("control text inside checker output forged success: %+v", frame)
	}
	if !strings.Contains(frame.output, "__ADP_PHASE_FRAME__ check 0 0") {
		t.Fatalf("expected hostile marker to remain payload, got %q", frame.output)
	}
}

func testCheckerBatchFrame(phase, output string, exitCode int) string {
	return fmt.Sprintf("__ADP_PHASE_FRAME__ %s %d %d\n%s\n", phase, exitCode, len(output), output)
}

func TestDockerCheckerExecutorExecuteBatchRunsPhasesInOneContainer(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "docker.log")
	binPath := filepath.Join(t.TempDir(), "docker")
	// Fake docker emulates the framed multi-phase output the batch entrypoint emits.
	script := "#!/bin/sh\n" +
		"printf '%s\\n' \"$*\" >> \"$DOCKER_LOG\"\n" +
		"printf '__ADP_PHASE_FRAME__ put 0 8\\nput done\\n'\n" +
		"printf '__ADP_PHASE_FRAME__ get 0 8\\nget done\\n'\n" +
		"printf '__ADP_PHASE_FRAME__ check 0 71\\nADPLATFORM_SERVICE_STATE={\"status\":\"ok\",\"message\":\"healthy\"}\\ncheck done\\n'\n"
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
	if len(result.Phases) != 3 {
		t.Fatalf("expected put+get+check phases, got %d: %+v", len(result.Phases), result.Phases)
	}
	if result.Phases[0].Phase != "put" || result.Phases[0].Status != "success" {
		t.Fatalf("unexpected put phase %+v", result.Phases[0])
	}
	if result.Phases[1].Phase != "get" || result.Phases[1].ServiceState != "" || result.Phases[1].Output != "get done" {
		t.Fatalf("unexpected get phase %+v", result.Phases[1])
	}
	if result.Phases[2].Phase != "check" || result.Phases[2].ServiceState != "ok" || result.Phases[2].Output != "check done" {
		t.Fatalf("unexpected check phase %+v", result.Phases[2])
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
		"printf '__ADP_PHASE_FRAME__ put 1 4\\nboom\\n'\n"
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

func TestDockerCheckerExecutorExecuteBatchFailsClosedOnMalformedFrame(t *testing.T) {
	binPath := filepath.Join(t.TempDir(), "docker")
	script := "#!/bin/sh\n" +
		"printf '__ADP_PHASE_FRAME__ put 0 100\\nshort\\n'\n"
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
	if len(result.Phases) != 1 || result.Phases[0].Status != "failed" || result.Phases[0].ExitCode != -1 {
		t.Fatalf("expected malformed framing to fail the batch, got %+v", result.Phases)
	}
}

func TestDockerCheckerExecutorExecuteBatchIgnoresReportedStateFromFailedPhase(t *testing.T) {
	binPath := filepath.Join(t.TempDir(), "docker")
	if err := os.WriteFile(binPath, []byte("#!/bin/sh\nprintf '%s' \"$DOCKER_OUTPUT\"\n"), 0o755); err != nil {
		t.Fatalf("write fake docker: %v", err)
	}
	t.Setenv("DOCKER_OUTPUT", testCheckerBatchFrame(
		"check",
		"ADPLATFORM_SERVICE_STATE={\"status\":\"ok\",\"message\":\"forged\"}\nchecker failed",
		1,
	))

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
		Phases:       []string{"check"},
		Target:       "10.80.7.11:10007",
		TickID:       19,
	})
	if err != nil {
		t.Fatalf("execute batch failed: %v", err)
	}
	if len(result.Phases) != 1 || result.Phases[0].Status != "failed" {
		t.Fatalf("expected failed checker phase, got %+v", result.Phases)
	}
	if result.Phases[0].ServiceState != "" || result.Phases[0].StateMessage != "" {
		t.Fatalf("failed phase supplied authoritative service state: %+v", result.Phases[0])
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

func TestCheckerRunnerExecuteBatchEndpointNormalizesPhases(t *testing.T) {
	server := newCheckerRunnerServer("dev-admin-token", dryRunCheckerExecutor{})
	mux := httpapi.NewBaseMux(httpapi.ServiceInfo{Name: "checker-runner", Version: "dev", Addr: ":0"})
	server.RegisterRoutes(mux)

	body := bytes.NewBufferString(`{"challenge_id":7,"team_id":101,"checker_image":"registry.local/proxy-checker:latest","phases":[" PUT ","Get","check"],"target":"10.80.7.11:10007","tick_id":19}`)
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
	want := []string{"put", "get", "check"}
	if len(payload.Phases) != len(want) {
		t.Fatalf("expected normalized phases %v, got %+v", want, payload.Phases)
	}
	for i := range want {
		if payload.Phases[i].Phase != want[i] {
			t.Fatalf("expected normalized phases %v, got %+v", want, payload.Phases)
		}
	}
}

func TestCheckerRunnerExecuteBatchEndpointRejectsDuplicateNormalizedPhase(t *testing.T) {
	server := newCheckerRunnerServer("dev-admin-token", dryRunCheckerExecutor{})
	mux := httpapi.NewBaseMux(httpapi.ServiceInfo{Name: "checker-runner", Version: "dev", Addr: ":0"})
	server.RegisterRoutes(mux)

	body := bytes.NewBufferString(`{"challenge_id":7,"team_id":101,"checker_image":"registry.local/proxy-checker:latest","phases":["put"," PUT "],"target":"10.80.7.11:10007","tick_id":19}`)
	request := httptest.NewRequest(http.MethodPost, "/internal/v1/checkers/execute-batch", body)
	request.Header.Set("Authorization", "Bearer dev-admin-token")
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", response.Code, response.Body.String())
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
