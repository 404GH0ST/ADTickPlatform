package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os/exec"
	"strings"
	"time"

	"adplatform/internal/platform/config"
	"adplatform/internal/platform/gamenet"
	"adplatform/internal/platform/httpapi"
	"adplatform/internal/services/apigateway"
)

type checkerRunnerServer struct {
	adminToken string
	executor   checkerExecutor
}

type checkerExecutor interface {
	ValidateChecker(ctx context.Context, request apigateway.CheckerValidationRequest) (apigateway.CheckerValidationResult, error)
	ExecuteChecker(ctx context.Context, request apigateway.CheckerExecutionRequest) (apigateway.CheckerExecutionResult, error)
}

type dryRunCheckerExecutor struct{}

type dockerCheckerExecutor struct {
	binary        string
	network       string
	networkLayout string
	security      checkerDockerSecurity
	timeout       time.Duration
}

type checkerDockerSecurity struct {
	capDrop     []string
	capAdd      []string
	securityOpt []string
	pidsLimit   string
	memory      string
	cpus        string
}

func (s checkerDockerSecurity) dockerArgs() []string {
	args := make([]string, 0)
	for _, capability := range s.capDrop {
		args = append(args, "--cap-drop", capability)
	}
	for _, capability := range s.capAdd {
		args = append(args, "--cap-add", capability)
	}
	for _, option := range s.securityOpt {
		args = append(args, "--security-opt", option)
	}
	if s.pidsLimit != "" {
		args = append(args, "--pids-limit", s.pidsLimit)
	}
	if s.memory != "" {
		args = append(args, "--memory", s.memory)
	}
	if s.cpus != "" {
		args = append(args, "--cpus", s.cpus)
	}
	return args
}

func checkerCSVConfig(key, fallback string) []string {
	raw := config.String(key, fallback)
	trimmedRaw := strings.TrimSpace(raw)
	if configDisabled(trimmedRaw) {
		return nil
	}
	parts := strings.Split(trimmedRaw, ",")
	values := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			values = append(values, trimmed)
		}
	}
	return values
}

func checkerStringConfig(key, fallback string) string {
	value := strings.TrimSpace(config.String(key, fallback))
	if configDisabled(value) {
		return ""
	}
	return value
}

func configDisabled(value string) bool {
	return value == "" || strings.EqualFold(value, "none") || strings.EqualFold(value, "disabled")
}

func defaultCheckerSecurity() checkerDockerSecurity {
	return checkerDockerSecurity{
		capDrop:     []string{"ALL"},
		securityOpt: []string{"no-new-privileges:true"},
		pidsLimit:   "128",
		memory:      "256m",
		cpus:        "0.5",
	}
}

func newCheckerRunnerServer(adminToken string, executor checkerExecutor) *checkerRunnerServer {
	return &checkerRunnerServer{
		adminToken: strings.TrimSpace(adminToken),
		executor:   executor,
	}
}

func (s *checkerRunnerServer) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /internal/v1/checkers/validate", s.handleValidateChecker)
	mux.HandleFunc("POST /internal/v1/checkers/execute", s.handleExecuteChecker)
}

func (s *checkerRunnerServer) handleValidateChecker(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdminAuth(w, r) {
		return
	}
	var request apigateway.CheckerValidationRequest
	if err := httpapi.DecodeJSON(r, &request); err != nil || request.ChallengeID <= 0 || strings.TrimSpace(request.CheckerImage) == "" {
		httpapi.WriteProblem(w, http.StatusBadRequest, httpapi.ProblemDetails{
			Title:  "Invalid request",
			Detail: "checker validation request is invalid.",
		})
		return
	}
	result, err := s.executor.ValidateChecker(r.Context(), request)
	if err != nil {
		httpapi.WriteProblem(w, http.StatusInternalServerError, httpapi.ProblemDetails{
			Title:  "Checker validation failed",
			Detail: err.Error(),
		})
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, result)
}

func (s *checkerRunnerServer) handleExecuteChecker(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdminAuth(w, r) {
		return
	}
	var request apigateway.CheckerExecutionRequest
	if err := httpapi.DecodeJSON(r, &request); err != nil || request.TeamID <= 0 || request.ChallengeID <= 0 || strings.TrimSpace(request.CheckerImage) == "" || !isValidCheckerPhase(request.Phase) || strings.TrimSpace(request.Target) == "" {
		httpapi.WriteProblem(w, http.StatusBadRequest, httpapi.ProblemDetails{
			Title:  "Invalid request",
			Detail: "checker execution request is invalid.",
		})
		return
	}
	result, err := s.executor.ExecuteChecker(r.Context(), request)
	if err != nil {
		httpapi.WriteProblem(w, http.StatusInternalServerError, httpapi.ProblemDetails{
			Title:  "Checker execution failed",
			Detail: err.Error(),
		})
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, result)
}

func (s *checkerRunnerServer) requireAdminAuth(w http.ResponseWriter, r *http.Request) bool {
	token, ok := httpapi.BearerToken(r)
	if !ok || token != s.adminToken {
		httpapi.WriteProblem(w, http.StatusForbidden, httpapi.ProblemDetails{
			Title:  "Forbidden",
			Detail: "please authenticate before accessing checker-runner endpoints.",
		})
		return false
	}
	return true
}

func newCheckerExecutor() checkerExecutor {
	switch strings.ToLower(config.String("CHECKER_RUNNER_MODE", "dry-run")) {
	case "docker":
		network := strings.TrimSpace(config.String("CHECKER_RUNNER_DOCKER_NETWORK", ""))
		if network == "" {
			network = strings.TrimSpace(config.String("CONTROLLER_DOCKER_NETWORK", "adplatform_game"))
		}
		return &dockerCheckerExecutor{
			binary:        strings.TrimSpace(config.String("CHECKER_RUNNER_DOCKER_BIN", "docker")),
			network:       network,
			networkLayout: gamenet.NormalizeLayout(config.String("AD_PLATFORM_NETWORK_LAYOUT", "per-service")),
			security: checkerDockerSecurity{
				capDrop:     checkerCSVConfig("CHECKER_RUNNER_CAP_DROP", "ALL"),
				capAdd:      checkerCSVConfig("CHECKER_RUNNER_CAP_ADD", ""),
				securityOpt: checkerCSVConfig("CHECKER_RUNNER_SECURITY_OPT", "no-new-privileges:true"),
				pidsLimit:   checkerStringConfig("CHECKER_RUNNER_PIDS_LIMIT", "128"),
				memory:      checkerStringConfig("CHECKER_RUNNER_MEMORY", "256m"),
				cpus:        checkerStringConfig("CHECKER_RUNNER_CPUS", "0.5"),
			},
			timeout: config.Duration("CHECKER_RUNNER_TIMEOUT", 15*time.Second),
		}
	default:
		return dryRunCheckerExecutor{}
	}
}

func (dryRunCheckerExecutor) ValidateChecker(_ context.Context, request apigateway.CheckerValidationRequest) (apigateway.CheckerValidationResult, error) {
	return apigateway.CheckerValidationResult{
		ChallengeID:            request.ChallengeID,
		Name:                   request.Name,
		CheckerImage:           request.CheckerImage,
		Status:                 "valid",
		ContractOK:             true,
		ServiceStateContractOK: true,
		CheckedAt:              time.Now().UTC().Format(time.RFC3339),
		Message:                "checker contract validation assumed in dry-run mode.",
	}, nil
}

func (dryRunCheckerExecutor) ExecuteChecker(_ context.Context, request apigateway.CheckerExecutionRequest) (apigateway.CheckerExecutionResult, error) {
	result := apigateway.CheckerExecutionResult{
		ChallengeID: request.ChallengeID,
		TeamID:      request.TeamID,
		Phase:       request.Phase,
		Status:      "success",
		ExitCode:    0,
		CheckedAt:   time.Now().UTC().Format(time.RFC3339),
		Message:     "checker execution assumed in dry-run mode.",
		Output:      "dry-run checker execution",
	}
	if strings.EqualFold(request.Phase, "check") {
		result.ServiceState = "ok"
		result.StateMessage = "dry-run checker reported the service healthy."
	}
	return result, nil
}

func (e *dockerCheckerExecutor) ValidateChecker(ctx context.Context, request apigateway.CheckerValidationRequest) (apigateway.CheckerValidationResult, error) {
	runCtx, cancel := context.WithTimeout(ctx, e.timeout)
	defer cancel()

	result := apigateway.CheckerValidationResult{
		ChallengeID:  request.ChallengeID,
		Name:         request.Name,
		CheckerImage: request.CheckerImage,
		CheckedAt:    time.Now().UTC().Format(time.RFC3339),
	}
	if strings.TrimSpace(request.CheckerImage) == "" {
		result.Status = "invalid"
		result.Message = "checker image is required for checker validation."
		return result, nil
	}

	output, exitCode, err := e.execDocker(runCtx, buildDockerCheckerValidationArgs(e.security, request)...)
	trimmedOutput, serviceStateContractOK := parseCheckerValidationOutput(output)
	if err != nil {
		result.Status = "invalid"
		result.Message = commandFailureMessage(err, []byte(trimmedOutput), exitCode)
		return result, nil
	}

	result.Status = "valid"
	result.ContractOK = true
	result.ServiceStateContractOK = serviceStateContractOK
	if !serviceStateContractOK {
		result.Status = "invalid"
		result.ContractOK = false
		result.Message = "checker validate output did not advertise canonical service-state support."
		return result, nil
	}
	result.Message = "checker image satisfies runner contract."
	return result, nil
}

func (e *dockerCheckerExecutor) ExecuteChecker(ctx context.Context, request apigateway.CheckerExecutionRequest) (apigateway.CheckerExecutionResult, error) {
	timeout := e.timeout
	if request.TimeoutSeconds > 0 {
		timeout = time.Duration(request.TimeoutSeconds) * time.Second
	}
	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	targetHost, _ := apigateway.ParseEndpoint(request.Target)
	networkPlan, err := gamenet.PlanDockerNetwork(e.network, e.networkLayout, firstNonEmpty(request.TargetIP, request.TargetHost, targetHost))
	if err != nil {
		return apigateway.CheckerExecutionResult{}, err
	}
	output, exitCode, err := e.execDocker(runCtx, buildDockerCheckerExecuteArgs(networkPlan.Name, e.security, request)...)
	trimmedOutput, serviceState, stateMessage := parseCheckerServiceStateOutput(output)
	result := apigateway.CheckerExecutionResult{
		ChallengeID:  request.ChallengeID,
		TeamID:       request.TeamID,
		Phase:        request.Phase,
		CheckedAt:    time.Now().UTC().Format(time.RFC3339),
		Output:       trimmedOutput,
		ExitCode:     exitCode,
		ServiceState: serviceState,
		StateMessage: stateMessage,
	}
	if err != nil {
		result.Status = "failed"
		result.Message = commandFailureMessage(err, []byte(trimmedOutput), exitCode)
		return result, nil
	}
	result.Status = "success"
	result.Message = "checker phase completed successfully."
	return result, nil
}

func (e *dockerCheckerExecutor) execDocker(ctx context.Context, args ...string) ([]byte, int, error) {
	cmd := exec.CommandContext(ctx, e.binary, args...) // #nosec G204,G702 -- docker binary is host operator configuration; args are passed without shell expansion.
	output, err := cmd.CombinedOutput()
	if err == nil {
		return output, 0, nil
	}

	exitCode := -1
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		exitCode = exitErr.ExitCode()
	}
	return output, exitCode, fmt.Errorf("%s %s failed: %w", e.binary, strings.Join(args, " "), err)
}

func buildDockerCheckerValidationArgs(security checkerDockerSecurity, request apigateway.CheckerValidationRequest) []string {
	args := []string{
		"run",
		"--rm",
	}
	args = append(args, security.dockerArgs()...)
	args = append(args,
		"--entrypoint",
		"/bin/sh",
		request.CheckerImage,
		"-lc",
		checkerEntrypointValidationScript(),
	)
	return args
}

func buildDockerCheckerExecuteArgs(network string, security checkerDockerSecurity, request apigateway.CheckerExecutionRequest) []string {
	args := []string{
		"run",
		"--rm",
	}
	args = append(args, security.dockerArgs()...)
	if strings.TrimSpace(network) != "" {
		args = append(args, "--network", network)
	}
	args = append(args,
		"-e", fmt.Sprintf("AD_PHASE=%s", request.Phase),
		"-e", fmt.Sprintf("AD_TEAM_ID=%d", request.TeamID),
		"-e", fmt.Sprintf("AD_TEAM_NAME=%s", request.TeamName),
		"-e", fmt.Sprintf("AD_CHALLENGE_ID=%d", request.ChallengeID),
		"-e", fmt.Sprintf("AD_CHALLENGE_NAME=%s", request.ChallengeName),
		"-e", fmt.Sprintf("AD_TARGET=%s", request.Target),
		"-e", fmt.Sprintf("AD_TARGET_HOST=%s", request.TargetHost),
		"-e", fmt.Sprintf("AD_TARGET_IP=%s", request.TargetIP),
		"-e", fmt.Sprintf("AD_TARGET_PORT=%d", request.TargetPort),
		"-e", fmt.Sprintf("AD_TICK_ID=%d", request.TickID),
		"-e", fmt.Sprintf("AD_FLAG=%s", request.Flag),
		"-e", fmt.Sprintf("AD_METADATA=%s", request.Metadata),
		"--entrypoint",
		"/bin/sh",
		request.CheckerImage,
		"-lc",
		checkerEntrypointExecuteScript(),
	)
	return args
}

func checkerEntrypointValidationScript() string {
	return `set -eu
checker_exec=""
checker_arg0=""
if command -v checker >/dev/null 2>&1; then
  checker_exec="checker"
elif [ -x /checker ]; then
  checker_exec="/checker"
elif [ -x /app/checker ]; then
  checker_exec="/app/checker"
elif [ -x /checker.sh ]; then
  checker_exec="/checker.sh"
elif [ -f /checker.sh ]; then
  checker_exec="/bin/sh"
  checker_arg0="/checker.sh"
else
  echo 'missing standard checker entrypoint inside image' >&2
  exit 1
fi
if [ -n "$checker_arg0" ]; then
  if "$checker_exec" "$checker_arg0" validate; then
    exit 0
  fi
  if "$checker_exec" "$checker_arg0" --help >/dev/null 2>&1; then
    exit 0
  fi
else
  if "$checker_exec" validate; then
    exit 0
  fi
  if "$checker_exec" --help >/dev/null 2>&1; then
    exit 0
  fi
fi
echo 'checker entrypoint does not support validate or --help' >&2
exit 1`
}

func checkerEntrypointExecuteScript() string {
	return `set -eu
case "$AD_PHASE" in
  put|get|check) ;;
  *)
    echo "unsupported checker phase: $AD_PHASE" >&2
    exit 2
    ;;
esac
checker_exec=""
checker_arg0=""
if command -v checker >/dev/null 2>&1; then
  checker_exec="checker"
elif [ -x /checker ]; then
  checker_exec="/checker"
elif [ -x /app/checker ]; then
  checker_exec="/app/checker"
elif [ -x /checker.sh ]; then
  checker_exec="/checker.sh"
elif [ -f /checker.sh ]; then
  checker_exec="/bin/sh"
  checker_arg0="/checker.sh"
else
  echo 'missing standard checker entrypoint inside image' >&2
  exit 1
fi
if [ -n "$checker_arg0" ]; then
  exec "$checker_exec" "$checker_arg0" "$AD_PHASE"
fi
exec "$checker_exec" "$AD_PHASE"`
}

func isValidCheckerPhase(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "put", "get", "check":
		return true
	default:
		return false
	}
}

func commandFailureMessage(err error, output []byte, exitCode int) string {
	trimmed := strings.TrimSpace(string(output))
	if trimmed != "" {
		return trimmed
	}
	if exitCode >= 0 {
		return fmt.Sprintf("checker command exited with status %d", exitCode)
	}
	return err.Error()
}

type checkerReportedServiceState struct {
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
}

type checkerCapabilities struct {
	ServiceState bool `json:"service_state"`
}

func parseCheckerServiceStateOutput(output []byte) (string, string, string) {
	lines := strings.Split(string(output), "\n")
	filtered := make([]string, 0, len(lines))
	reportedStatus := ""
	reportedMessage := ""

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "ADPLATFORM_SERVICE_STATE=") {
			filtered = append(filtered, line)
			continue
		}

		raw := strings.TrimSpace(strings.TrimPrefix(trimmed, "ADPLATFORM_SERVICE_STATE="))
		if raw == "" {
			continue
		}

		var payload checkerReportedServiceState
		if strings.HasPrefix(raw, "{") {
			if err := json.Unmarshal([]byte(raw), &payload); err != nil {
				filtered = append(filtered, line)
				continue
			}
		} else {
			payload.Status = raw
		}

		normalized := apigateway.NormalizeServiceStateStatus(payload.Status)
		if normalized == "" {
			filtered = append(filtered, line)
			continue
		}

		reportedStatus = normalized
		reportedMessage = strings.TrimSpace(payload.Message)
	}

	return strings.TrimSpace(strings.Join(filtered, "\n")), reportedStatus, reportedMessage
}

func parseCheckerValidationOutput(output []byte) (string, bool) {
	lines := strings.Split(string(output), "\n")
	filtered := make([]string, 0, len(lines))
	serviceStateContractOK := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "ADPLATFORM_CHECKER_CAPABILITIES=") {
			filtered = append(filtered, line)
			continue
		}

		raw := strings.TrimSpace(strings.TrimPrefix(trimmed, "ADPLATFORM_CHECKER_CAPABILITIES="))
		if raw == "" {
			continue
		}

		var payload checkerCapabilities
		if err := json.Unmarshal([]byte(raw), &payload); err != nil {
			filtered = append(filtered, line)
			continue
		}
		serviceStateContractOK = payload.ServiceState
	}

	return strings.TrimSpace(strings.Join(filtered, "\n")), serviceStateContractOK
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
