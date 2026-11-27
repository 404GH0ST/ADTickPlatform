package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	"adplatform/internal/platform/config"
	"adplatform/internal/platform/gamenet"
	"adplatform/internal/services/apigateway"
)

type runtimeExecutor interface {
	EnsureService(ctx context.Context, task apigateway.ControllerRuntimeTask) error
	FactoryResetService(ctx context.Context, task apigateway.ControllerRuntimeTask) error
	RestartService(ctx context.Context, task apigateway.ControllerRuntimeTask) error
	ApplySSHCredential(ctx context.Context, task apigateway.ControllerRuntimeTask, credential apigateway.ControllerSSHCredential) error
	ValidateChallengeRuntime(ctx context.Context, request apigateway.ChallengeValidationRequest) (apigateway.ChallengeValidationResult, error)
	RemoveService(ctx context.Context, teamID, challengeID int) error
	RemoveTeamServices(ctx context.Context, teamID int, stateVolumes ...string) error
	RemoveChallengeServices(ctx context.Context, challengeID int) error
}

var errContainerOwnership = errors.New("container does not belong to requested service instance")

// These failures are known to occur before the state volume is removed. The
// API may reverse its provisional penalty; it marks the service degraded when
// the container itself may already have changed.
type resetPreflightError struct {
	err                   error
	runtimeMayHaveChanged bool
}

type volumeLookupError struct{ err error }

func (e volumeLookupError) Error() string { return e.err.Error() }
func (e volumeLookupError) Unwrap() error { return e.err }

func (e resetPreflightError) Error() string { return e.err.Error() }
func (e resetPreflightError) Unwrap() error { return e.err }

func newRuntimeExecutor() runtimeExecutor {
	switch strings.ToLower(config.String("CONTROLLER_RUNTIME_MODE", "dry-run")) {
	case "docker":
		return &dockerCLIExecutor{
			binary:           config.String("CONTROLLER_DOCKER_BIN", "docker"),
			network:          strings.TrimSpace(config.String("CONTROLLER_DOCKER_NETWORK", "adplatform_game")),
			networkLayout:    gamenet.NormalizeLayout(config.String("AD_PLATFORM_NETWORK_LAYOUT", "per-service")),
			stateMountPath:   strings.TrimSpace(config.String("CONTROLLER_STATE_MOUNT_PATH", "/opt/ad/state")),
			requireDiskQuota: config.Bool("CONTROLLER_REQUIRE_DISK_QUOTA", false),
			diskLimitMiB:     config.Int("CONTROLLER_SERVICE_DISK_LIMIT_MIB", 0),
			serviceSecurity: dockerRunSecurity{
				capDrop:     csvConfig("CONTROLLER_SERVICE_CAP_DROP", "ALL"),
				capAdd:      csvConfig("CONTROLLER_SERVICE_CAP_ADD", "CHOWN,DAC_OVERRIDE,FOWNER,SETGID,SETUID,NET_BIND_SERVICE,SYS_CHROOT,AUDIT_WRITE"),
				securityOpt: csvConfig("CONTROLLER_SERVICE_SECURITY_OPT", "no-new-privileges:true"),
				pidsLimit:   stringConfig("CONTROLLER_SERVICE_PIDS_LIMIT", "256"),
				memory:      stringConfig("CONTROLLER_SERVICE_MEMORY", "512m"),
				cpus:        stringConfig("CONTROLLER_SERVICE_CPUS", "1.0"),
			},
			sshApplyMode:         strings.ToLower(strings.TrimSpace(config.String("CONTROLLER_SSH_PASSWORD_APPLY_MODE", "docker-exec"))),
			sshContractMode:      strings.ToLower(strings.TrimSpace(config.String("CONTROLLER_SSH_CONTRACT_MODE", "verify"))),
			checkerValidator:     newCheckerValidationClient(),
			captureMode:          strings.ToLower(config.String("CONTROLLER_CAPTURE_MODE", "disabled")),
			captureVolume:        config.String("CONTROLLER_CAPTURE_BOOTSTRAP_VOLUME", "adplatform_prod_capture_bootstrap"),
			captureTool:          config.String("CONTROLLER_CAPTURE_POLICY_BINARY", "/usr/local/bin/adplatform-capture-policy"),
			captureObject:        config.String("CONTROLLER_CAPTURE_POLICY_OBJECT", "/usr/local/share/adplatform/capture.bpf.o"),
			captureProcRoot:      config.String("CONTROLLER_CAPTURE_PROC_ROOT", "/host/proc"),
			captureNetnsRoot:     config.String("CONTROLLER_CAPTURE_NETNS_ROOT", "/host/docker-netns"),
			captureNetnsHostRoot: config.String("CAPTURE_NETNS_HOST_PATH", "/var/run/docker/netns"),
			timeout:              config.Duration("CONTROLLER_RUNTIME_TIMEOUT", 15*time.Second),
		}
	default:
		return dryRunExecutor{}
	}
}

type dryRunExecutor struct{}

func (dryRunExecutor) EnsureService(_ context.Context, _ apigateway.ControllerRuntimeTask) error {
	return nil
}

func (dryRunExecutor) FactoryResetService(_ context.Context, _ apigateway.ControllerRuntimeTask) error {
	return nil
}

func (dryRunExecutor) RestartService(_ context.Context, _ apigateway.ControllerRuntimeTask) error {
	return nil
}

func (dryRunExecutor) ApplySSHCredential(_ context.Context, _ apigateway.ControllerRuntimeTask, _ apigateway.ControllerSSHCredential) error {
	return nil
}

func (dryRunExecutor) ValidateChallengeRuntime(_ context.Context, request apigateway.ChallengeValidationRequest) (apigateway.ChallengeValidationResult, error) {
	return apigateway.ChallengeValidationResult{
		ChallengeID:            request.ChallengeID,
		Name:                   request.Name,
		BaselineImage:          request.BaselineImage,
		CheckerImage:           request.CheckerImage,
		Status:                 "valid",
		BaselineSSHContractOK:  true,
		CheckerContractOK:      true,
		ServiceStateContractOK: true,
		CheckedAt:              time.Now().UTC().Format(time.RFC3339),
		Message:                "challenge package validation assumed in dry-run runtime mode.",
	}, nil
}

func (dryRunExecutor) RemoveService(_ context.Context, _, _ int) error {
	return nil
}

func (dryRunExecutor) RemoveTeamServices(_ context.Context, _ int, _ ...string) error {
	return nil
}

func (dryRunExecutor) RemoveChallengeServices(_ context.Context, _ int) error {
	return nil
}

type dockerCLIExecutor struct {
	binary           string
	network          string
	networkLayout    string
	stateMountPath   string
	requireDiskQuota bool
	diskLimitMiB     int
	serviceSecurity  dockerRunSecurity
	sshApplyMode     string
	sshContractMode  string
	checkerValidator checkerValidationClient
	timeout          time.Duration

	captureMode          string
	captureVolume        string
	captureTool          string
	captureObject        string
	captureProcRoot      string
	captureNetnsRoot     string
	captureNetnsHostRoot string
	// captureMu guards captureSeen only. Instance operations run commands inside
	// participant containers, so they hold a per-instance lock from captureLocks.
	captureMu    sync.Mutex
	captureLocks sync.Map
	// captureSweepInterval spaces guard re-verification sweeps; zero means 2s.
	captureSweepInterval time.Duration
	captureSeen          map[string]captureStamp
}

type dockerRunSecurity struct {
	capDrop     []string
	capAdd      []string
	securityOpt []string
	pidsLimit   string
	memory      string
	cpus        string
}

func (s dockerRunSecurity) dockerArgs() []string {
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

func csvConfig(key, fallback string) []string {
	raw := config.String(key, fallback)
	trimmedRaw := strings.TrimSpace(raw)
	if dockerConfigDisabled(trimmedRaw) {
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

func stringConfig(key, fallback string) string {
	value := strings.TrimSpace(config.String(key, fallback))
	if dockerConfigDisabled(value) {
		return ""
	}
	return value
}

func dockerConfigDisabled(value string) bool {
	return value == "" || strings.EqualFold(value, "none") || strings.EqualFold(value, "disabled")
}

func defaultProbeSecurity() dockerRunSecurity {
	return dockerRunSecurity{
		capDrop:     []string{"ALL"},
		securityOpt: []string{"no-new-privileges:true"},
		pidsLimit:   "128",
		memory:      "256m",
		cpus:        "0.5",
	}
}

func (e *dockerCLIExecutor) EnsureService(ctx context.Context, task apigateway.ControllerRuntimeTask) error {
	if err := e.validateCaptureConfig(); err != nil {
		return err
	}
	if e.captureEnabled() {
		defer e.lockCaptureInstance(task.ContainerName)()
	}

	if task.RuntimeKind != "" && task.RuntimeKind != "docker" {
		return fmt.Errorf("unsupported runtime kind %q for %s", task.RuntimeKind, task.ContainerName)
	}

	// Use an overall deadline (3x per-step timeout) so that slow image
	// inspects or network creation don't starve later steps.
	overallCtx, overallCancel := context.WithTimeout(ctx, e.timeout*3)
	defer overallCancel()
	if err := e.ensureStateVolume(overallCtx, task.StateVolume); err != nil {
		return err
	}

	exists, err := e.containerExists(overallCtx, task.ContainerName)
	if err != nil {
		return err
	}
	var containerID string
	if exists {
		containerID, err = e.inspectTaskContainer(overallCtx, task)
		if err != nil {
			return err
		}
		if err := e.verifyContainerDiskLimits(overallCtx, task.ContainerName); err != nil {
			return err
		}
	}

	if exists && e.captureEnabled() {
		info, err := e.inspectCapture(overallCtx, task.ContainerName)
		if err != nil {
			return err
		}
		if len(info.Config.Entrypoint) == 1 && info.Config.Entrypoint[0] == captureGate {
			// Existing guarded containers without the DNS restriction, or whose
			// baked checker token a redeploy has since rotated, must be
			// recreated: the DNS policy and the token both take effect only at
			// container creation, so a warm reconcile would leave stale ones.
			if err := e.verifyContainerDNS(overallCtx, task); err == nil && checkerTokenCurrent(info, task) {
				if err = e.reconcileCapturedService(overallCtx, task); err != nil {
					e.stopCapturedService(task.ContainerName)
					return err
				}
				return e.verifySSHContract(overallCtx, task)
			}
		}
	}
	if exists {
		if _, err := e.execDocker(overallCtx, "rm", "-f", containerID); err != nil {
			return err
		}
	}

	if err := e.runFreshContainer(overallCtx, task); err != nil {
		// Clean up any partially created container on failure.
		_ = e.removeTaskContainerIfExists(context.Background(), task)
		return err
	}
	return e.verifySSHContract(overallCtx, task)
}

func (e *dockerCLIExecutor) FactoryResetService(ctx context.Context, task apigateway.ControllerRuntimeTask) error {
	if err := e.validateCaptureConfig(); err != nil {
		return resetPreflightError{err: err}
	}
	if e.captureEnabled() {
		defer e.lockCaptureInstance(task.ContainerName)()
	}

	if task.RuntimeKind != "" && task.RuntimeKind != "docker" {
		return resetPreflightError{err: fmt.Errorf("unsupported runtime kind %q for %s", task.RuntimeKind, task.ContainerName)}
	}

	runCtx, cancel := context.WithTimeout(ctx, e.timeout)
	defer cancel()

	if err := e.removeTaskContainerIfExists(runCtx, task); err != nil {
		return resetPreflightError{err: err, runtimeMayHaveChanged: !errors.Is(err, errContainerOwnership)}
	}
	if err := e.removeVolumeIfExists(runCtx, task.StateVolume); err != nil {
		var lookup volumeLookupError
		if errors.As(err, &lookup) {
			return resetPreflightError{err: err, runtimeMayHaveChanged: true}
		}
		// Docker may report an error after a removal attempt. Only reverse the
		// penalty when a fresh inspection proves the volume still exists.
		probeCtx, probeCancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer probeCancel()
		if _, probeErr := e.execDocker(probeCtx, "volume", "inspect", task.StateVolume); probeErr == nil {
			return resetPreflightError{err: err, runtimeMayHaveChanged: true}
		}
		return err
	}
	if err := e.runFreshContainer(runCtx, task); err != nil {
		return err
	}
	return e.verifySSHContract(runCtx, task)
}

func (e *dockerCLIExecutor) RestartService(ctx context.Context, task apigateway.ControllerRuntimeTask) error {
	if err := e.validateCaptureConfig(); err != nil {
		return err
	}
	if e.captureEnabled() {
		defer e.lockCaptureInstance(task.ContainerName)()
	}

	if task.RuntimeKind != "" && task.RuntimeKind != "docker" {
		return fmt.Errorf("unsupported runtime kind %q for %s", task.RuntimeKind, task.ContainerName)
	}

	runCtx, cancel := context.WithTimeout(ctx, e.timeout)
	defer cancel()

	exists, err := e.containerExists(runCtx, task.ContainerName)
	if err != nil {
		return err
	}
	if !exists {
		if err := e.runFreshContainer(runCtx, task); err != nil {
			return err
		}
		return e.verifySSHContract(runCtx, task)
	}
	containerID, err := e.inspectTaskContainer(runCtx, task)
	if err != nil {
		return err
	}
	if err := e.verifyContainerDiskLimits(runCtx, task.ContainerName); err != nil {
		return err
	}
	if err := e.verifyContainerDNS(runCtx, task); err != nil {
		return err
	}
	if err := e.ensureStateVolume(runCtx, task.StateVolume); err != nil {
		return err
	}

	if e.captureEnabled() {
		info, err := e.inspectCapture(runCtx, task.ContainerName)
		if err != nil {
			return err
		}
		if len(info.Config.Entrypoint) != 1 || info.Config.Entrypoint[0] != captureGate {
			return fmt.Errorf("legacy instance requires deployment reconciliation before guarded restart")
		}
	}

	if _, err := e.execDocker(runCtx, "restart", containerID); err != nil {
		return err
	}
	if e.captureEnabled() {
		if err := e.reconcileCapturedService(runCtx, task); err != nil {
			return err
		}
	}
	return e.verifySSHContract(runCtx, task)
}

func (e *dockerCLIExecutor) ApplySSHCredential(ctx context.Context, task apigateway.ControllerRuntimeTask, credential apigateway.ControllerSSHCredential) error {
	if credential.StateVolume != "" && credential.StateVolume != task.StateVolume {
		return fmt.Errorf("service instance changed before applying the SSH credential")
	}
	if err := e.validateCaptureConfig(); err != nil {
		return err
	}
	if e.captureEnabled() {
		defer e.lockCaptureInstance(task.ContainerName)()
	}
	if task.RuntimeKind != "" && task.RuntimeKind != "docker" {
		return fmt.Errorf("unsupported runtime kind %q for %s", task.RuntimeKind, task.ContainerName)
	}
	if e.sshApplyMode == "disabled" {
		return fmt.Errorf("ssh credential runtime apply is disabled")
	}

	runCtx, cancel := context.WithTimeout(ctx, e.timeout)
	defer cancel()

	exists, err := e.containerExists(runCtx, task.ContainerName)
	if err != nil {
		return err
	}
	if !exists {
		if err := e.runFreshContainer(runCtx, task); err != nil {
			return err
		}
	} else {
		if _, err := e.inspectTaskContainer(runCtx, task); err != nil {
			return err
		}
		if err := e.verifyContainerDiskLimits(runCtx, task.ContainerName); err != nil {
			return err
		}
		if err := e.verifyContainerDNS(runCtx, task); err != nil {
			return err
		}
		if err := e.ensureStateVolume(runCtx, task.StateVolume); err != nil {
			return err
		}
	}
	if e.captureEnabled() {
		if err := e.reconcileCapturedService(runCtx, task); err != nil {
			e.stopCapturedService(task.ContainerName)
			return err
		}
	}
	if err := e.verifySSHContract(runCtx, task); err != nil {
		return err
	}
	// The container name can be reused between the contract probe and password
	// update. Execute against the inspected immutable ID.
	containerID, err := e.inspectTaskContainer(runCtx, task)
	if err != nil {
		return err
	}
	task.ContainerName = containerID

	_, err = e.execDockerWithEnv(runCtx, []string{"AD_PLATFORM_ROOT_PASSWORD=" + credential.Password}, buildDockerSSHCredentialArgs(task)...)
	return err
}

func (e *dockerCLIExecutor) ValidateChallengeRuntime(ctx context.Context, request apigateway.ChallengeValidationRequest) (apigateway.ChallengeValidationResult, error) {
	runCtx, cancel := context.WithTimeout(ctx, e.timeout)
	defer cancel()

	result := apigateway.ChallengeValidationResult{
		ChallengeID:   request.ChallengeID,
		Name:          request.Name,
		BaselineImage: request.BaselineImage,
		CheckerImage:  request.CheckerImage,
		CheckedAt:     time.Now().UTC().Format(time.RFC3339),
	}
	if strings.TrimSpace(request.BaselineImage) == "" {
		result.Status = "invalid"
		result.Message = "baseline image is required for runtime validation."
		return result, nil
	}
	if strings.TrimSpace(request.CheckerImage) == "" {
		result.Status = "invalid"
		result.Message = "checker image is required for runtime validation."
		return result, nil
	}

	if _, err := e.execDocker(runCtx, buildDockerBaselineValidationArgs(request)...); err != nil {
		result.Status = "invalid"
		result.Message = err.Error()
		return result, nil
	}
	result.BaselineSSHContractOK = true

	checkerRequest := apigateway.CheckerValidationRequest{
		ChallengeID:  request.ChallengeID,
		Name:         request.Name,
		CheckerImage: request.CheckerImage,
	}
	if e.checkerValidator != nil {
		checkerResult, err := e.checkerValidator.Validate(runCtx, checkerRequest)
		if err != nil {
			return result, errors.New("checker validation could not be completed")
		}
		if checkerResult.Status != "valid" || !checkerResult.ContractOK {
			result.Status = "invalid"
			switch checkerResult.Message {
			case apigateway.CheckerCapabilityFailure, apigateway.CheckerOutputLimitFailure,
				apigateway.CheckerTimeoutFailure, apigateway.CheckerCanceledFailure:
				result.Message = checkerResult.Message
			default:
				result.Message = "checker image failed runner validation."
			}
			return result, nil
		}
		if !checkerResult.ServiceStateContractOK {
			result.Status = "invalid"
			result.Message = "checker validation did not confirm canonical service-state support."
			return result, nil
		}
	} else if _, err := e.execDocker(runCtx, buildDockerCheckerValidationArgs(checkerRequest)...); err != nil {
		result.Status = "invalid"
		result.Message = "checker image failed runtime validation."
		return result, nil
	}
	result.CheckerContractOK = true
	result.ServiceStateContractOK = true
	result.Status = "valid"
	result.Message = "challenge package satisfies runtime validation."
	return result, nil
}

func (e *dockerCLIExecutor) RemoveService(ctx context.Context, teamID, challengeID int) error {
	runCtx, cancel := context.WithTimeout(ctx, e.timeout)
	defer cancel()

	return e.removeByFilter(runCtx, fmt.Sprintf("label=adplatform.team_id=%d,label=adplatform.challenge_id=%d", teamID, challengeID))
}

func (e *dockerCLIExecutor) RemoveTeamServices(ctx context.Context, teamID int, stateVolumes ...string) error {
	runCtx, cancel := context.WithTimeout(ctx, e.timeout)
	defer cancel()

	return e.removeByFilter(runCtx, fmt.Sprintf("label=adplatform.team_id=%d", teamID), stateVolumes...)
}

func (e *dockerCLIExecutor) RemoveChallengeServices(ctx context.Context, challengeID int) error {
	runCtx, cancel := context.WithTimeout(ctx, e.timeout)
	defer cancel()

	return e.removeByFilter(runCtx, fmt.Sprintf("label=adplatform.challenge_id=%d", challengeID))
}

func (e *dockerCLIExecutor) removeByFilter(ctx context.Context, filter string, stateVolumes ...string) error {
	output, err := e.execDocker(ctx, "ps", "-a", "--filter", filter, "--format", "{{.ID}} {{.Labels}}")
	if err != nil {
		return err
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	var cleanupErrors []error
	volumes := make(map[string]struct{}, len(stateVolumes))
	for _, name := range stateVolumes {
		volumes[name] = struct{}{}
	}
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		parts := strings.SplitN(line, " ", 2)
		containerID := parts[0]

		// Try to extract state volume from labels if present
		var stateVolume string
		if len(parts) > 1 {
			labels := parts[1]
			// Labels are comma separated in format: key=value,key=value
			for _, label := range strings.Split(labels, ",") {
				if strings.HasPrefix(label, "adplatform.state_volume=") {
					stateVolume = strings.TrimPrefix(label, "adplatform.state_volume=")
					break
				}
			}
		}

		if _, err := e.execDocker(ctx, "rm", "-f", containerID); err != nil {
			cleanupErrors = append(cleanupErrors, fmt.Errorf("remove container %q: %w", containerID, err))
			continue
		}
		if stateVolume != "" {
			volumes[stateVolume] = struct{}{}
		}
	}
	for name := range volumes {
		if err := e.removeVolumeIfExists(ctx, name); err != nil {
			cleanupErrors = append(cleanupErrors, fmt.Errorf("remove state volume %q: %w", name, err))
		}
	}
	return errors.Join(cleanupErrors...)
}

func (e *dockerCLIExecutor) verifySSHContract(ctx context.Context, task apigateway.ControllerRuntimeTask) error {
	if e.sshContractMode == "disabled" {
		return nil
	}
	args := buildDockerSSHContractProbeArgs(task)
	var lastErr error
	for {
		_, err := e.execDocker(ctx, args...)
		if err == nil {
			return nil
		}
		lastErr = err
		if !isTransientDockerExecState(err) {
			return err
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("container %s did not become ready for SSH contract verification: %w", task.ContainerName, lastErr)
		case <-time.After(500 * time.Millisecond):
		}
	}
}

func isTransientDockerExecState(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "is restarting") ||
		strings.Contains(message, "is not running") ||
		strings.Contains(message, "container is restarting") ||
		strings.Contains(message, "container is not running")
}

func (e *dockerCLIExecutor) containerExists(ctx context.Context, containerName string) (bool, error) {
	output, err := e.execDocker(ctx, "ps", "-a", "--filter", fmt.Sprintf("name=^/%s$", containerName), "--format", "{{.Names}}")
	if err != nil {
		return false, err
	}
	for _, line := range strings.Split(strings.TrimSpace(string(output)), "\n") {
		if strings.TrimSpace(line) == containerName {
			return true, nil
		}
	}
	return false, nil
}

func (e *dockerCLIExecutor) inspectTaskContainer(ctx context.Context, task apigateway.ControllerRuntimeTask) (string, error) {
	output, err := e.execDocker(ctx, "inspect", "--format", `{{.Id}} {{json .Config.Labels}}`, task.ContainerName)
	if err != nil {
		return "", err
	}
	id, labelsJSON, ok := strings.Cut(strings.TrimSpace(string(output)), " ")
	var labels map[string]string
	if !ok || id == "" || json.Unmarshal([]byte(labelsJSON), &labels) != nil ||
		labels["adplatform.team_id"] != strconv.Itoa(task.TeamID) ||
		labels["adplatform.challenge_id"] != strconv.Itoa(task.ChallengeID) ||
		labels["adplatform.state_volume"] != task.StateVolume {
		return "", fmt.Errorf("container %s: %w", task.ContainerName, errContainerOwnership)
	}
	return id, nil
}

func (e *dockerCLIExecutor) removeTaskContainerIfExists(ctx context.Context, task apigateway.ControllerRuntimeTask) error {
	exists, err := e.containerExists(ctx, task.ContainerName)
	if err != nil {
		return err
	}
	if !exists {
		return nil
	}
	id, err := e.inspectTaskContainer(ctx, task)
	if err != nil {
		return err
	}
	_, err = e.execDocker(ctx, "rm", "-f", id)
	return err
}

func (e *dockerCLIExecutor) runFreshContainer(ctx context.Context, task apigateway.ControllerRuntimeTask) error {
	plan, err := e.networkPlan(task)
	if err != nil {
		return err
	}
	if err := e.refreshServiceImage(ctx, task.BaselineImage); err != nil {
		return err
	}
	if err := e.verifyImageVolumes(ctx, task.BaselineImage); err != nil {
		return err
	}
	if err := e.ensureStateVolume(ctx, task.StateVolume); err != nil {
		return err
	}
	if err := e.ensureNetwork(ctx, plan); err != nil {
		return err
	}
	var guardedArgs []string
	if e.captureEnabled() {
		guardedArgs, err = e.capturedRunArgs(ctx, plan.Name, task)
		if err != nil {
			return err
		}
	}
	var runErr error
	for attempts := 0; attempts < 3; attempts++ {
		args := guardedArgs
		if !e.captureEnabled() {
			args = buildDockerRunArgs(plan.Name, e.stateMountPath, e.serviceSecurity, task)
		}
		if e.diskLimitMiB > 0 {
			// Add options before the image, including when the guard supplies
			// the original entrypoint and command as positional operands.
			args = append([]string{"run", "--storage-opt", "size=" + e.diskQuotaBytes()}, args[1:]...)
		}
		_, runErr = e.execDockerWithEnv(ctx, []string{"AD_CHECKER_TOKEN=" + task.CheckerToken}, args...)
		if runErr == nil {
			if e.captureEnabled() {
				return e.reconcileCapturedService(ctx, task)
			}
			return nil
		}
		if !strings.Contains(strings.ToLower(runErr.Error()), "conflict") {
			return runErr
		}
		// Explicitly remove the conflicting container before retrying,
		// otherwise every retry will fail with the same naming conflict.
		if err := e.removeTaskContainerIfExists(ctx, task); err != nil {
			return err
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("timeout waiting for container name release: %w", runErr)
		case <-time.After(500 * time.Millisecond):
		}
	}
	return runErr
}

func (e *dockerCLIExecutor) refreshServiceImage(ctx context.Context, image string) error {
	image = strings.TrimSpace(image)
	if image == "" {
		return nil
	}
	if strings.HasPrefix(image, "local/") {
		if _, inspectErr := e.execDocker(ctx, "image", "inspect", image); inspectErr == nil {
			return nil
		}
	}
	if _, err := e.execDocker(ctx, "pull", image); err == nil {
		return nil
	} else if _, inspectErr := e.execDocker(ctx, "image", "inspect", image); inspectErr == nil {
		log.Printf("warning: docker pull failed for %s; using existing local image: %v", image, err)
		return nil
	} else {
		return fmt.Errorf("docker pull failed: %w (local fallback inspect also failed: %v)", err, inspectErr)
	}
}

func (e *dockerCLIExecutor) networkPlan(task apigateway.ControllerRuntimeTask) (gamenet.DockerNetworkPlan, error) {
	serviceIP := strings.TrimSpace(task.SSHHost)
	if serviceIP == "" {
		if endpointHost, _ := apigateway.ParseEndpoint(task.Endpoint); endpointHost != "" {
			serviceIP = endpointHost
		}
	}
	return gamenet.PlanDockerNetwork(e.network, e.networkLayout, serviceIP)
}

func (e *dockerCLIExecutor) execDocker(ctx context.Context, args ...string) ([]byte, error) {
	return e.execDockerWithEnv(ctx, nil, args...)
}

func (e *dockerCLIExecutor) execDockerWithEnv(ctx context.Context, env []string, args ...string) ([]byte, error) {
	commandCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	cmd := exec.CommandContext(commandCtx, e.binary, args...) // #nosec G204,G702 -- docker binary is host operator configuration; args are passed without shell expansion.
	cmd.Env = append(cmd.Environ(), env...)
	output := &boundedDockerOutput{limit: maxDockerCommandOutput, cancel: cancel}
	cmd.Stdout, cmd.Stderr = output, output
	err := cmd.Run()
	buffer, overflow := output.result()
	if overflow {
		return nil, fmt.Errorf("docker command output exceeded %d bytes", maxDockerCommandOutput)
	}
	if err != nil {
		operation := "command"
		if len(args) > 0 && strings.TrimSpace(args[0]) != "" {
			operation = strings.TrimSpace(args[0])
		}
		detail := strings.TrimSpace(redactControllerDockerSecrets(string(buffer), args))
		detail = redactControllerDockerSecrets(detail, env)
		if detail == "" {
			return nil, fmt.Errorf("%s %s failed: %w", e.binary, operation, err)
		}
		return nil, fmt.Errorf("%s %s failed: %w: %s", e.binary, operation, err, detail)
	}
	return buffer, nil
}

const maxDockerCommandOutput = 4 << 20

type boundedDockerOutput struct {
	mu       sync.Mutex
	buffer   bytes.Buffer
	limit    int
	cancel   context.CancelFunc
	overflow bool
}

func (o *boundedDockerOutput) Write(p []byte) (int, error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	remaining := o.limit - o.buffer.Len()
	if len(p) > remaining {
		o.buffer.Write(p[:remaining])
		o.overflow = true
		o.cancel()
		return remaining, fmt.Errorf("docker command output limit reached")
	}
	return o.buffer.Write(p)
}

func (o *boundedDockerOutput) result() ([]byte, bool) {
	o.mu.Lock()
	defer o.mu.Unlock()
	return bytes.Clone(o.buffer.Bytes()), o.overflow
}

func redactControllerDockerSecrets(message string, args []string) string {
	for _, arg := range args {
		for _, prefix := range []string{
			"AD_CHECKER_TOKEN=",
			"AD_PLATFORM_ROOT_PASSWORD=",
		} {
			if !strings.HasPrefix(arg, prefix) {
				continue
			}
			secret := strings.TrimPrefix(arg, prefix)
			if secret != "" {
				message = strings.ReplaceAll(message, secret, "[redacted]")
			}
		}
	}
	return message
}

func (e *dockerCLIExecutor) removeVolumeIfExists(ctx context.Context, volumeName string) error {
	if strings.TrimSpace(volumeName) == "" {
		return nil
	}
	output, err := e.execDocker(ctx, "volume", "ls", "--filter", fmt.Sprintf("name=^%s$", volumeName), "--format", "{{.Name}}")
	if err != nil {
		return volumeLookupError{err: err}
	}
	for _, line := range strings.Split(strings.TrimSpace(string(output)), "\n") {
		if strings.TrimSpace(line) == volumeName {
			_, err := e.execDocker(ctx, "volume", "rm", "-f", volumeName)
			return err
		}
	}
	return nil
}

func (e *dockerCLIExecutor) ensureNetwork(ctx context.Context, plan gamenet.DockerNetworkPlan) error {
	if strings.TrimSpace(plan.Name) == "" {
		return nil
	}
	if _, err := e.execDocker(ctx, "network", "inspect", plan.Name); err == nil {
		return nil
	}
	args := []string{
		"network",
		"create",
		"--label", "adplatform.game_network=true",
		"--label", fmt.Sprintf("adplatform.network_layout=%s", plan.Layout),
	}
	if strings.TrimSpace(plan.Subnet) != "" {
		args = append(args, "--subnet", plan.Subnet)
	}
	args = append(args, plan.Name)
	_, err := e.execDocker(ctx, args...)
	if err != nil {
		// Another instance of the same challenge may have created it first.
		if _, inspectErr := e.execDocker(ctx, "network", "inspect", plan.Name); inspectErr == nil {
			return nil
		}
	}
	return err
}

func buildDockerRunArgs(network, stateMountPath string, security dockerRunSecurity, task apigateway.ControllerRuntimeTask) []string {
	serviceIP := strings.TrimSpace(task.SSHHost)
	servicePort := task.ServicePort
	if endpointHost, endpointPort := apigateway.ParseEndpoint(task.Endpoint); serviceIP == "" || servicePort == 0 {
		if serviceIP == "" {
			serviceIP = endpointHost
		}
		if servicePort == 0 {
			servicePort = endpointPort
		}
	}
	args := []string{
		"run",
		"-d",
		"--init",
		"--restart", "unless-stopped",
		"--log-driver", "local",
		"--log-opt", "max-size=10m",
		"--log-opt", "max-file=3",
	}
	args = append(args, security.dockerArgs()...)
	args = append(args,
		"--name", task.ContainerName,
		"--hostname", task.ContainerName,
		"--label", fmt.Sprintf("adplatform.team_id=%d", task.TeamID),
		"--label", fmt.Sprintf("adplatform.challenge_id=%d", task.ChallengeID),
		"--label", fmt.Sprintf("adplatform.deployment_job_id=%d", task.DeploymentJobID),
		"--label", fmt.Sprintf("adplatform.service_ip=%s", serviceIP),
		"--label", fmt.Sprintf("adplatform.service_port=%d", servicePort),
		"--label", fmt.Sprintf("adplatform.state_volume=%s", task.StateVolume),
		"-e", fmt.Sprintf("AD_PLATFORM_TEAM_ID=%d", task.TeamID),
		"-e", fmt.Sprintf("AD_PLATFORM_CHALLENGE_ID=%d", task.ChallengeID),
		"-e", fmt.Sprintf("AD_PLATFORM_SERVICE_IP=%s", serviceIP),
		"-e", fmt.Sprintf("AD_PLATFORM_SERVICE_PORT=%d", servicePort),
		"-e", fmt.Sprintf("AD_PLATFORM_ENDPOINT=%s", task.Endpoint),
		"-e", "AD_CHECKER_TOKEN",
		"-e", fmt.Sprintf("PORT=%d", servicePort),
	)
	if task.DisableExternalDNS {
		// Keep Docker's embedded resolver for local names while disabling its
		// external fallback through a resolver on container loopback.
		args = append(args, "--dns", "127.0.0.1")
	}
	if strings.TrimSpace(task.StateVolume) != "" && strings.TrimSpace(stateMountPath) != "" {
		args = append(args, "--mount", fmt.Sprintf("type=volume,src=%s,dst=%s", task.StateVolume, stateMountPath))
	}
	if strings.TrimSpace(network) != "" {
		args = append(args, "--network", network)
		if strings.TrimSpace(serviceIP) != "" {
			args = append(args, "--ip", serviceIP)
		}
	}
	args = append(args, task.BaselineImage)
	return args
}

func buildDockerSSHCredentialArgs(task apigateway.ControllerRuntimeTask) []string {
	return []string{
		"exec",
		"-e", "AD_PLATFORM_ROOT_PASSWORD",
		task.ContainerName,
		"/bin/sh",
		"-lc",
		`set -eu
if command -v chpasswd >/dev/null 2>&1; then
  printf 'root:%s\n' "$AD_PLATFORM_ROOT_PASSWORD" | chpasswd
elif command -v passwd >/dev/null 2>&1; then
  printf '%s\n%s\n' "$AD_PLATFORM_ROOT_PASSWORD" "$AD_PLATFORM_ROOT_PASSWORD" | passwd root >/dev/null
	else
	  echo 'no supported password setter found inside container' >&2
	  exit 1
	fi
	if [ -d /run/adplatform ]; then
	  rm -f /run/adplatform/ssh-password-expires-at
	fi`,
	}
}

func buildDockerSSHContractProbeArgs(task apigateway.ControllerRuntimeTask) []string {
	return []string{
		"exec",
		task.ContainerName,
		"/bin/sh",
		"-lc",
		`set -eu
if command -v sshd >/dev/null 2>&1 || [ -x /usr/sbin/sshd ] || command -v dropbear >/dev/null 2>&1 || [ -x /usr/sbin/dropbear ]; then
  :
else
  echo 'missing ssh daemon binary inside container' >&2
  exit 1
fi
if command -v chpasswd >/dev/null 2>&1 || command -v passwd >/dev/null 2>&1; then
  :
else
  echo 'missing supported password setter inside container' >&2
  exit 1
		fi`,
	}
}

func buildDockerBaselineValidationArgs(request apigateway.ChallengeValidationRequest) []string {
	args := []string{
		"run",
		"--rm",
	}
	args = append(args, defaultProbeSecurity().dockerArgs()...)
	args = append(args,
		"--entrypoint",
		"/bin/sh",
		request.BaselineImage,
		"-lc",
		`set -eu
if command -v sshd >/dev/null 2>&1 || [ -x /usr/sbin/sshd ] || command -v dropbear >/dev/null 2>&1 || [ -x /usr/sbin/dropbear ]; then
  :
else
  echo 'missing ssh daemon binary inside image' >&2
  exit 1
fi
if command -v chpasswd >/dev/null 2>&1 || command -v passwd >/dev/null 2>&1; then
  :
else
  echo 'missing supported password setter inside image' >&2
  exit 1
fi`,
	)
	return args
}

func buildDockerCheckerValidationArgs(request apigateway.CheckerValidationRequest) []string {
	args := []string{
		"run",
		"--rm",
		"--log-driver=none",
	}
	args = append(args, defaultProbeSecurity().dockerArgs()...)
	args = append(args,
		"--entrypoint",
		"/bin/sh",
		request.CheckerImage,
		"-lc",
		`set -eu
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
  if "$checker_exec" "$checker_arg0" validate >/dev/null 2>&1 || "$checker_exec" "$checker_arg0" --help >/dev/null 2>&1; then
    exit 0
  fi
else
  if "$checker_exec" validate >/dev/null 2>&1 || "$checker_exec" --help >/dev/null 2>&1; then
    exit 0
  fi
fi
echo 'checker entrypoint does not support validate or --help' >&2
exit 1`,
	)
	return args
}
