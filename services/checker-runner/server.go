package main

import (
	"bufio"
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unicode/utf8"

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
	ExecuteCheckerBatch(ctx context.Context, request apigateway.CheckerBatchExecutionRequest) (apigateway.CheckerBatchExecutionResult, error)
}

type dryRunCheckerExecutor struct{}

type dockerCheckerExecutor struct {
	binary        string
	network       string
	networkLayout string
	security      checkerDockerSecurity
	timeout       time.Duration

	checkerNetworkMu sync.Mutex
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
	mux.HandleFunc("POST /internal/v1/checkers/execute-batch", s.handleExecuteCheckerBatch)
	mux.HandleFunc("POST /internal/v1/checkers/rotate", s.handleRotateChecker)
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
			Detail: "checker validation could not be completed.",
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
	decodeErr := httpapi.DecodeJSON(r, &request)
	phase, phaseOK := normalizeCheckerPhase(request.Phase)
	if decodeErr != nil || request.TeamID <= 0 || request.ChallengeID <= 0 || strings.TrimSpace(request.CheckerImage) == "" || !phaseOK || strings.TrimSpace(request.Target) == "" {
		httpapi.WriteProblem(w, http.StatusBadRequest, httpapi.ProblemDetails{
			Title:  "Invalid request",
			Detail: "checker execution request is invalid.",
		})
		return
	}
	request.Phase = phase
	result, err := s.executor.ExecuteChecker(r.Context(), request)
	if err != nil {
		httpapi.WriteProblem(w, http.StatusInternalServerError, httpapi.ProblemDetails{
			Title:  "Checker execution failed",
			Detail: "checker execution could not be completed.",
		})
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, result)
}

func (s *checkerRunnerServer) handleExecuteCheckerBatch(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdminAuth(w, r) {
		return
	}
	var request apigateway.CheckerBatchExecutionRequest
	decodeErr := httpapi.DecodeJSON(r, &request)
	phases, phasesOK := normalizeCheckerPhases(request.Phases)
	if decodeErr != nil || request.TeamID <= 0 || request.ChallengeID <= 0 || strings.TrimSpace(request.CheckerImage) == "" || !phasesOK || strings.TrimSpace(request.Target) == "" {
		httpapi.WriteProblem(w, http.StatusBadRequest, httpapi.ProblemDetails{
			Title:  "Invalid request",
			Detail: "checker batch execution request is invalid.",
		})
		return
	}
	request.Phases = phases
	result, err := s.executor.ExecuteCheckerBatch(r.Context(), request)
	if err != nil {
		httpapi.WriteProblem(w, http.StatusInternalServerError, httpapi.ProblemDetails{
			Title:  "Checker execution failed",
			Detail: "checker batch execution could not be completed.",
		})
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, result)
}

func (s *checkerRunnerServer) requireAdminAuth(w http.ResponseWriter, r *http.Request) bool {
	token, ok := httpapi.BearerToken(r)
	if !ok || subtle.ConstantTimeCompare([]byte(token), []byte(s.adminToken)) != 1 {
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

func (dryRunCheckerExecutor) ExecuteCheckerBatch(_ context.Context, request apigateway.CheckerBatchExecutionRequest) (apigateway.CheckerBatchExecutionResult, error) {
	result := apigateway.CheckerBatchExecutionResult{
		ChallengeID: request.ChallengeID,
		TeamID:      request.TeamID,
	}
	for _, phase := range request.Phases {
		pr := apigateway.CheckerPhaseResult{
			Phase:     phase,
			Status:    "success",
			ExitCode:  0,
			CheckedAt: time.Now().UTC().Format(time.RFC3339),
			Message:   "checker execution assumed in dry-run mode.",
			Output:    "dry-run checker execution",
		}
		if strings.EqualFold(phase, "check") {
			pr.ServiceState = "ok"
			pr.StateMessage = "dry-run checker reported the service healthy."
		}
		result.Phases = append(result.Phases, pr)
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
	if err != nil {
		result.Status = "invalid"
		result.Message = commandFailureMessage(err, exitCode)
		return result, nil
	}

	result.Status = "valid"
	result.ContractOK = true
	result.ServiceStateContractOK = parseCheckerValidationOutput(output)
	if !result.ServiceStateContractOK {
		result.Status = "invalid"
		result.ContractOK = false
		result.Message = apigateway.CheckerCapabilityFailure
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
	runNetwork, sourceIP := e.resolveRunNetwork(runCtx, networkPlan.Name, request.SourceIP)
	request.SourceIP = sourceIP
	name := dockerRunName("adchk", request.ChallengeID, request.TeamID, request.TickID)
	stateNonce := rand.Text()
	env := checkerRunEnv(request.Flag, request.Metadata, request.CheckerToken, stateNonce)
	output, exitCode, err := e.execDockerWithEnv(runCtx, env, buildDockerCheckerExecuteArgs(runNetwork, name, e.security, request)...)
	if retryOnServiceNetwork(runCtx, exitCode, request.SourceIP) {
		e.forceRemove(name)
		request.SourceIP = ""
		output, exitCode, err = e.execDockerWithEnv(runCtx, env, buildDockerCheckerExecuteArgs(networkPlan.Name, name, e.security, request)...)
	}
	if runCtx.Err() != nil {
		e.forceRemove(name)
	}
	result := apigateway.CheckerExecutionResult{
		ChallengeID: request.ChallengeID,
		TeamID:      request.TeamID,
		Phase:       request.Phase,
		CheckedAt:   time.Now().UTC().Format(time.RFC3339),
		ExitCode:    exitCode,
	}
	if err != nil {
		result.Status = "failed"
		result.Message = commandFailureMessage(err, exitCode)
		return result, nil
	}
	result.Status = "success"
	result.Message = "checker phase completed successfully."
	filtered, serviceState, stateMessage := parseCheckerServiceStateOutput(output, strings.EqualFold(request.Phase, "check"), stateNonce)
	result.Output = truncateOutput(filtered, 65536)
	result.ServiceState = serviceState
	result.StateMessage = stateMessage
	return result, nil
}

func (e *dockerCheckerExecutor) ExecuteCheckerBatch(ctx context.Context, request apigateway.CheckerBatchExecutionRequest) (apigateway.CheckerBatchExecutionResult, error) {
	result := apigateway.CheckerBatchExecutionResult{
		ChallengeID: request.ChallengeID,
		TeamID:      request.TeamID,
	}

	perPhase := e.timeout
	if request.TimeoutSeconds > 0 {
		perPhase = time.Duration(request.TimeoutSeconds) * time.Second
	}
	// One container runs every phase sequentially, so the deadline scales with
	// the number of phases.
	runCtx, cancel := context.WithTimeout(ctx, perPhase*time.Duration(len(request.Phases)))
	defer cancel()

	targetHost, _ := apigateway.ParseEndpoint(request.Target)
	networkPlan, err := gamenet.PlanDockerNetwork(e.network, e.networkLayout, firstNonEmpty(request.TargetIP, request.TargetHost, targetHost))
	if err != nil {
		return result, err
	}

	runNetwork, sourceIP := e.resolveRunNetwork(runCtx, networkPlan.Name, request.SourceIP)
	request.SourceIP = sourceIP
	name := dockerRunName("adchkb", request.ChallengeID, request.TeamID, request.TickID)
	stateNonce := rand.Text()
	env := checkerRunEnv(request.Flag, request.Metadata, request.CheckerToken, stateNonce)
	output, exitCode, execErr := e.execDockerWithEnv(runCtx, env, buildDockerCheckerBatchExecuteArgs(runNetwork, name, e.security, request)...)
	if retryOnServiceNetwork(runCtx, exitCode, request.SourceIP) {
		e.forceRemove(name)
		request.SourceIP = ""
		output, _, execErr = e.execDockerWithEnv(runCtx, env, buildDockerCheckerBatchExecuteArgs(networkPlan.Name, name, e.security, request)...)
	}
	if runCtx.Err() != nil {
		e.forceRemove(name)
	}
	checkedAt := time.Now().UTC().Format(time.RFC3339)
	if execErr != nil {
		result.Phases = append(result.Phases, apigateway.CheckerPhaseResult{
			Phase:     request.Phases[0],
			Status:    "failed",
			ExitCode:  -1,
			CheckedAt: checkedAt,
			Message:   commandFailureMessage(execErr, -1),
		})
		return result, nil
	}
	frames, frameErr := parseCheckerBatchFrames(string(output))
	if frameErr != nil {
		result.Phases = append(result.Phases, apigateway.CheckerPhaseResult{
			Phase:     request.Phases[0],
			Status:    "failed",
			ExitCode:  -1,
			CheckedAt: checkedAt,
			Message:   apigateway.CheckerBatchFramingFailure,
		})
		return result, nil
	}

	for _, phase := range request.Phases {
		frame, ok := frames[phase]
		if !ok {
			// Phase never ran: either a prior phase failed and the script halted, or
			// the container died (timeout / infra error) before reaching it. Only the
			// latter is a hard error we surface; halted phases are simply absent and
			// the caller marks them skipped.
			break
		}
		pr := apigateway.CheckerPhaseResult{
			Phase:     phase,
			ExitCode:  frame.exitCode,
			CheckedAt: checkedAt,
		}
		if frame.exitCode != 0 {
			pr.Status = "failed"
			pr.Message = apigateway.CheckerExitFailureMessage(frame.exitCode)
			result.Phases = append(result.Phases, pr)
			break
		}
		filtered, serviceState, stateMessage := parseCheckerServiceStateOutput([]byte(frame.output), strings.EqualFold(phase, "check"), stateNonce)
		pr.Status = "success"
		pr.Message = "checker phase completed successfully."
		pr.Output = truncateOutput(filtered, 65536)
		pr.ServiceState = serviceState
		pr.StateMessage = stateMessage
		result.Phases = append(result.Phases, pr)
	}

	return result, nil
}

func truncateOutput(s string, limit int) string {
	if len(s) <= limit {
		return s
	}
	res := s[:limit]
	// Avoid cutting in the middle of a multi-byte UTF-8 sequence.
	for len(res) > 0 && !utf8.ValidString(res) {
		res = res[:len(res)-1]
	}
	return res + "\n... [truncated]"
}

var dockerRunSeq atomic.Uint64

// dockerRunName builds a unique, docker-valid container name so a run killed by
// context cancellation can be force-removed afterwards (a killed `docker run`
// CLI never fires its own --rm, leaking the container otherwise).
func dockerRunName(prefix string, challengeID, teamID, tickID int) string {
	return fmt.Sprintf("%s-%d-%d-%d-%d", prefix, tickID, challengeID, teamID, dockerRunSeq.Add(1))
}

// forceRemove best-effort deletes a container by name with a fresh short-lived
// context, since the run's own context is already cancelled when this is called.
func (e *dockerCheckerExecutor) forceRemove(name string) {
	if strings.TrimSpace(name) == "" {
		return
	}
	rmCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = exec.CommandContext(rmCtx, e.binary, "rm", "-f", name).Run() // #nosec G204 -- docker binary and generated name are operator-controlled.
}

func (e *dockerCheckerExecutor) execDocker(ctx context.Context, args ...string) ([]byte, int, error) {
	return e.execDockerWithEnv(ctx, nil, args...)
}

func checkerRunEnv(flag, metadata, token, stateNonce string) []string {
	return []string{"AD_FLAG=" + flag, "AD_METADATA=" + metadata, "AD_CHECKER_TOKEN=" + token, "AD_CHECKER_STATE_NONCE=" + stateNonce}
}

func (e *dockerCheckerExecutor) execDockerWithEnv(ctx context.Context, env []string, args ...string) ([]byte, int, error) {
	cmd := exec.CommandContext(ctx, e.binary, args...) // #nosec G204,G702 -- docker binary is host operator configuration; args are passed without shell expansion.
	cmd.Env = append(cmd.Environ(), env...)
	output, outputExceeded, err := runCommandWithBoundedOutput(cmd, maxCheckerProcessOutputBytes)

	if outputExceeded {
		return nil, -1, errCheckerOutputLimit
	}
	if err == nil {
		return output, 0, nil
	}
	if ctx.Err() != nil {
		return nil, -1, ctx.Err()
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return nil, exitErr.ExitCode(), err
	}
	return nil, -1, err
}

// Capture attached output for the checker protocol without sending it to the Docker log driver.
const checkerDockerLogArg = "--log-driver=none"

func buildDockerCheckerValidationArgs(security checkerDockerSecurity, request apigateway.CheckerValidationRequest) []string {
	args := []string{
		"run",
		"--rm",
		checkerDockerLogArg,
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

func buildDockerCheckerExecuteArgs(network, name string, security checkerDockerSecurity, request apigateway.CheckerExecutionRequest) []string {
	args := []string{
		"run",
		"--rm",
		checkerDockerLogArg,
		// Checker images are built locally on the runner host; never reach out to a
		// registry on the hot per-tick path.
		"--pull=never",
	}
	if strings.TrimSpace(name) != "" {
		args = append(args, "--name", name)
	}
	args = append(args, security.dockerArgs()...)
	args = appendCheckerNetworkArgs(args, network, request.SourceIP)
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
		"-e", "AD_FLAG",
		"-e", "AD_METADATA",
		"-e", "AD_CHECKER_TOKEN",
		"-e", "AD_CHECKER_STATE_NONCE",
		"--entrypoint",
		"/bin/sh",
		request.CheckerImage,
		"-lc",
		checkerEntrypointExecuteScript(),
	)
	return args
}

func buildDockerCheckerBatchExecuteArgs(network, name string, security checkerDockerSecurity, request apigateway.CheckerBatchExecutionRequest) []string {
	args := []string{
		"run",
		"--rm",
		checkerDockerLogArg,
		// Checker images are built locally on the runner host; never reach out to a
		// registry on the hot per-tick path.
		"--pull=never",
	}
	if strings.TrimSpace(name) != "" {
		args = append(args, "--name", name)
	}
	args = append(args, security.dockerArgs()...)
	args = appendCheckerNetworkArgs(args, network, request.SourceIP)
	args = append(args,
		"-e", fmt.Sprintf("AD_PHASES=%s", strings.Join(request.Phases, " ")),
		"-e", fmt.Sprintf("AD_TEAM_ID=%d", request.TeamID),
		"-e", fmt.Sprintf("AD_TEAM_NAME=%s", request.TeamName),
		"-e", fmt.Sprintf("AD_CHALLENGE_ID=%d", request.ChallengeID),
		"-e", fmt.Sprintf("AD_CHALLENGE_NAME=%s", request.ChallengeName),
		"-e", fmt.Sprintf("AD_TARGET=%s", request.Target),
		"-e", fmt.Sprintf("AD_TARGET_HOST=%s", request.TargetHost),
		"-e", fmt.Sprintf("AD_TARGET_IP=%s", request.TargetIP),
		"-e", fmt.Sprintf("AD_TARGET_PORT=%d", request.TargetPort),
		"-e", fmt.Sprintf("AD_TICK_ID=%d", request.TickID),
		"-e", "AD_FLAG",
		"-e", "AD_METADATA",
		"-e", "AD_CHECKER_TOKEN",
		"-e", "AD_CHECKER_STATE_NONCE",
		"--entrypoint",
		"/bin/sh",
		request.CheckerImage,
		"-lc",
		checkerEntrypointBatchExecuteScript(),
	)
	return args
}

// checkerBatchFrame is one phase's captured block from a batch container run.
type checkerBatchFrame struct {
	output   string
	exitCode int
}

const checkerBatchFramePrefix = "__ADP_PHASE_FRAME__ "

// parseCheckerBatchFrames decodes length-prefixed phase records emitted by the
// batch entrypoint. The payload length is authoritative, so checker output that
// contains frame-like text remains data instead of being interpreted as control
// records.
func parseCheckerBatchFrames(output string) (map[string]checkerBatchFrame, error) {
	if output == "" {
		return nil, errors.New("checker batch output is empty")
	}
	frames := make(map[string]checkerBatchFrame)
	reader := bufio.NewReader(strings.NewReader(output))
	for {
		if _, err := reader.Peek(1); err == io.EOF {
			return frames, nil
		}
		phase, frame, err := readCheckerBatchFrame(reader)
		if err != nil {
			return nil, err
		}
		if _, exists := frames[phase]; exists {
			return nil, errors.New("checker batch frame phase is duplicated")
		}
		frames[phase] = frame
	}
}

func readCheckerBatchFrame(reader *bufio.Reader) (string, checkerBatchFrame, error) {
	header, err := reader.ReadSlice('\n')
	if err != nil {
		return "", checkerBatchFrame{}, err
	}
	line := strings.TrimSuffix(string(header), "\n")
	fields := strings.Fields(strings.TrimPrefix(line, checkerBatchFramePrefix))
	if !strings.HasPrefix(line, checkerBatchFramePrefix) || len(fields) != 3 || !isValidCheckerPhase(fields[0]) {
		return "", checkerBatchFrame{}, errors.New("invalid checker frame header")
	}
	code, codeErr := strconv.Atoi(fields[1])
	size, sizeErr := strconv.Atoi(fields[2])
	if codeErr != nil || code < 0 || code > 255 || sizeErr != nil || size < 0 || size >= maxCheckerProcessOutputBytes {
		return "", checkerBatchFrame{}, errors.New("invalid checker frame bounds")
	}
	payload := make([]byte, size+1)
	if _, err := io.ReadFull(reader, payload); err != nil || payload[size] != '\n' {
		return "", checkerBatchFrame{}, errors.New("incomplete checker frame payload")
	}
	return strings.ToLower(strings.TrimSpace(fields[0])), checkerBatchFrame{output: string(payload[:size]), exitCode: code}, nil
}

// checkerEntrypointBatchExecuteScript resolves the checker entrypoint once and
// runs every phase in AD_PHASES sequentially inside the single container,
// chaining each successful phase's output into the next as AD_METADATA and
// halting on the first failure. Per-phase output is length-prefixed so output
// controlled by a target service cannot forge phase metadata.
func checkerEntrypointBatchExecuteScript() string {
	return checkerBatchEntrypoint + checkerBatchPhases
}

const checkerBatchEntrypoint = `set -u
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
run_phase() {
  if [ -n "$checker_arg0" ]; then
    "$checker_exec" "$checker_arg0" "$1"
  else
    "$checker_exec" "$1"
  fi
}
`

const checkerBatchPhases = `meta="${AD_METADATA:-}"
for phase in $AD_PHASES; do
  case "$phase" in
    put|get|check) ;;
    *)
      echo "unsupported checker phase: $phase" >&2
      exit 2
      ;;
  esac
  out=$(AD_PHASE="$phase" AD_METADATA="$meta" run_phase "$phase" </dev/null 2>&1)
  code=$?
  byte_count=$(printf '%s' "$out" | wc -c)
  printf '__ADP_PHASE_FRAME__ %s %s %s\n' "$phase" "$code" "$byte_count"
  printf '%s\n' "$out"
  if [ "$code" -ne 0 ]; then
    exit 0
  fi
  meta=$(printf '%s\n' "$out" | grep -v '^ADPLATFORM_SERVICE_STATE=' || true)
done
exit 0`

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
	_, ok := normalizeCheckerPhase(value)
	return ok
}

func normalizeCheckerPhase(value string) (string, bool) {
	phase := strings.ToLower(strings.TrimSpace(value))
	switch phase {
	case "put", "get", "check":
		return phase, true
	default:
		return "", false
	}
}

func normalizeCheckerPhases(phases []string) ([]string, bool) {
	if len(phases) == 0 {
		return nil, false
	}
	normalized := make([]string, 0, len(phases))
	seen := make(map[string]struct{}, len(phases))
	for _, phase := range phases {
		value, ok := normalizeCheckerPhase(phase)
		if !ok {
			return nil, false
		}
		if _, duplicate := seen[value]; duplicate {
			return nil, false
		}
		seen[value] = struct{}{}
		normalized = append(normalized, value)
	}
	return normalized, true
}

func commandFailureMessage(err error, exitCode int) string {
	switch {
	case errors.Is(err, errCheckerOutputLimit):
		return apigateway.CheckerOutputLimitFailure
	case errors.Is(err, context.DeadlineExceeded):
		return apigateway.CheckerTimeoutFailure
	case errors.Is(err, context.Canceled):
		return apigateway.CheckerCanceledFailure
	default:
		return apigateway.CheckerExitFailureMessage(exitCode)
	}
}

type checkerReportedServiceState struct {
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
	Nonce   string `json:"nonce,omitempty"`
}

type checkerCapabilities struct {
	ServiceState bool `json:"service_state"`
	StateNonce   bool `json:"state_nonce"`
}

func parseCheckerServiceStateOutput(output []byte, allowReportedState bool, stateNonce string) (string, string, string) {
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
		if !allowReportedState {
			continue
		}
		if stateNonce == "" || payload.Nonce != stateNonce {
			continue
		}
		// Only the first authenticated marker can set the service state.
		if reportedStatus == "" {
			reportedStatus = normalized
			reportedMessage = strings.TrimSpace(payload.Message)
		}
	}

	return strings.TrimSpace(strings.Join(filtered, "\n")), reportedStatus, reportedMessage
}

func parseCheckerValidationOutput(output []byte) bool {
	lines := strings.Split(string(output), "\n")
	serviceStateContractOK := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "ADPLATFORM_CHECKER_CAPABILITIES=") {
			continue
		}

		raw := strings.TrimSpace(strings.TrimPrefix(trimmed, "ADPLATFORM_CHECKER_CAPABILITIES="))
		if raw == "" {
			continue
		}

		var payload checkerCapabilities
		if err := json.Unmarshal([]byte(raw), &payload); err != nil {
			continue
		}
		serviceStateContractOK = payload.ServiceState && payload.StateNonce
	}

	return serviceStateContractOK
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
