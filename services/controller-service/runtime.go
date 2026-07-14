package main

import (
	"context"
	"fmt"
	"log"
	"os/exec"
	"strings"
	"time"

	"adplatform/internal/platform/config"
	"adplatform/internal/platform/gamenet"
	"adplatform/internal/platform/unlockproof"
	"adplatform/internal/services/apigateway"
)

type runtimeExecutor interface {
	EnsureService(ctx context.Context, task apigateway.ControllerRuntimeTask) error
	FactoryResetService(ctx context.Context, task apigateway.ControllerRuntimeTask) error
	RestartService(ctx context.Context, task apigateway.ControllerRuntimeTask) error
	ApplySSHCredential(ctx context.Context, task apigateway.ControllerRuntimeTask, credential apigateway.ControllerSSHCredential) error
	ValidateChallengeRuntime(ctx context.Context, request apigateway.ChallengeValidationRequest) (apigateway.ChallengeValidationResult, error)
	RemoveService(ctx context.Context, teamID, challengeID int) error
	RemoveTeamServices(ctx context.Context, teamID int) error
	RemoveChallengeServices(ctx context.Context, challengeID int) error
}

func newRuntimeExecutor() runtimeExecutor {
	switch strings.ToLower(config.String("CONTROLLER_RUNTIME_MODE", "dry-run")) {
	case "docker":
		return &dockerCLIExecutor{
			binary:            config.String("CONTROLLER_DOCKER_BIN", "docker"),
			network:           strings.TrimSpace(config.String("CONTROLLER_DOCKER_NETWORK", "adplatform_game")),
			networkLayout:     gamenet.NormalizeLayout(config.String("AD_PLATFORM_NETWORK_LAYOUT", "per-service")),
			stateMountPath:    strings.TrimSpace(config.String("CONTROLLER_STATE_MOUNT_PATH", "/opt/ad/state")),
			unlockProofSecret: strings.TrimSpace(config.Secret("UNLOCK_PROOF_SECRET", "TEAM_JWT_SECRET")),
			serviceSecurity: dockerRunSecurity{
				capDrop:     csvConfig("CONTROLLER_SERVICE_CAP_DROP", "ALL"),
				capAdd:      csvConfig("CONTROLLER_SERVICE_CAP_ADD", "CHOWN,DAC_OVERRIDE,FOWNER,SETGID,SETUID,NET_BIND_SERVICE,SYS_CHROOT,AUDIT_WRITE"),
				securityOpt: csvConfig("CONTROLLER_SERVICE_SECURITY_OPT", "no-new-privileges:true"),
				pidsLimit:   stringConfig("CONTROLLER_SERVICE_PIDS_LIMIT", "256"),
				memory:      stringConfig("CONTROLLER_SERVICE_MEMORY", "512m"),
				cpus:        stringConfig("CONTROLLER_SERVICE_CPUS", "1.0"),
			},
			sshApplyMode:     strings.ToLower(strings.TrimSpace(config.String("CONTROLLER_SSH_PASSWORD_APPLY_MODE", "docker-exec"))),
			sshContractMode:  strings.ToLower(strings.TrimSpace(config.String("CONTROLLER_SSH_CONTRACT_MODE", "verify"))),
			checkerValidator: newCheckerValidationClient(),
			timeout:          config.Duration("CONTROLLER_RUNTIME_TIMEOUT", 15*time.Second),
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

func (dryRunExecutor) RemoveTeamServices(_ context.Context, _ int) error {
	return nil
}

func (dryRunExecutor) RemoveChallengeServices(_ context.Context, _ int) error {
	return nil
}

type dockerCLIExecutor struct {
	binary            string
	network           string
	networkLayout     string
	stateMountPath    string
	unlockProofSecret string
	serviceSecurity   dockerRunSecurity
	sshApplyMode      string
	sshContractMode   string
	checkerValidator  checkerValidationClient
	timeout           time.Duration
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
	if task.RuntimeKind != "" && task.RuntimeKind != "docker" {
		return fmt.Errorf("unsupported runtime kind %q for %s", task.RuntimeKind, task.ContainerName)
	}

	// Use an overall deadline (3x per-step timeout) so that slow image
	// inspects or network creation don't starve later steps.
	overallCtx, overallCancel := context.WithTimeout(ctx, e.timeout*3)
	defer overallCancel()

	exists, err := e.containerExists(overallCtx, task.ContainerName)
	if err != nil {
		return err
	}
	if exists {
		if err := e.removeContainerIfExists(overallCtx, task.ContainerName); err != nil {
			return err
		}
	}

	if err := e.runFreshContainer(overallCtx, task); err != nil {
		// Clean up any partially created container on failure.
		_ = e.removeContainerIfExists(context.Background(), task.ContainerName)
		return err
	}
	return e.verifySSHContract(overallCtx, task)
}

func (e *dockerCLIExecutor) FactoryResetService(ctx context.Context, task apigateway.ControllerRuntimeTask) error {
	if task.RuntimeKind != "" && task.RuntimeKind != "docker" {
		return fmt.Errorf("unsupported runtime kind %q for %s", task.RuntimeKind, task.ContainerName)
	}

	runCtx, cancel := context.WithTimeout(ctx, e.timeout)
	defer cancel()

	if err := e.removeContainerIfExists(runCtx, task.ContainerName); err != nil {
		return err
	}
	if err := e.removeVolumeIfExists(runCtx, task.StateVolume); err != nil {
		return err
	}
	if err := e.runFreshContainer(runCtx, task); err != nil {
		return err
	}
	return e.verifySSHContract(runCtx, task)
}

func (e *dockerCLIExecutor) RestartService(ctx context.Context, task apigateway.ControllerRuntimeTask) error {
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

	if _, err := e.execDocker(runCtx, "restart", task.ContainerName); err != nil {
		return err
	}
	return e.verifySSHContract(runCtx, task)
}

func (e *dockerCLIExecutor) ApplySSHCredential(ctx context.Context, task apigateway.ControllerRuntimeTask, credential apigateway.ControllerSSHCredential) error {
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
	}
	if err := e.verifySSHContract(runCtx, task); err != nil {
		return err
	}

	_, err = e.execDocker(runCtx, buildDockerSSHCredentialArgs(task, credential)...)
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
			return result, err
		}
		if checkerResult.Status != "valid" || !checkerResult.ContractOK {
			result.Status = "invalid"
			result.Message = checkerResult.Message
			return result, nil
		}
		if !checkerResult.ServiceStateContractOK {
			result.Status = "invalid"
			result.Message = "checker validation did not confirm canonical service-state support."
			return result, nil
		}
	} else if _, err := e.execDocker(runCtx, buildDockerCheckerValidationArgs(checkerRequest)...); err != nil {
		result.Status = "invalid"
		result.Message = err.Error()
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

func (e *dockerCLIExecutor) RemoveTeamServices(ctx context.Context, teamID int) error {
	runCtx, cancel := context.WithTimeout(ctx, e.timeout)
	defer cancel()

	return e.removeByFilter(runCtx, fmt.Sprintf("label=adplatform.team_id=%d", teamID))
}

func (e *dockerCLIExecutor) RemoveChallengeServices(ctx context.Context, challengeID int) error {
	runCtx, cancel := context.WithTimeout(ctx, e.timeout)
	defer cancel()

	return e.removeByFilter(runCtx, fmt.Sprintf("label=adplatform.challenge_id=%d", challengeID))
}

func (e *dockerCLIExecutor) removeByFilter(ctx context.Context, filter string) error {
	output, err := e.execDocker(ctx, "ps", "-a", "--filter", filter, "--format", "{{.ID}} {{.Labels}}")
	if err != nil {
		return err
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	var lastErr error
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
			log.Printf("failed to remove container %q: %v", containerID, err)
			lastErr = err
			continue
		}
		if stateVolume != "" {
			if err := e.removeVolumeIfExists(ctx, stateVolume); err != nil {
				// We don't return error here to continue cleaning up other containers
				log.Printf("failed to remove volume %q: %v", stateVolume, err)
			}
		}
	}

	return lastErr
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

func (e *dockerCLIExecutor) removeContainerIfExists(ctx context.Context, containerName string) error {
	exists, err := e.containerExists(ctx, containerName)
	if err != nil {
		return err
	}
	if !exists {
		return nil
	}
	_, err = e.execDocker(ctx, "rm", "-f", containerName)
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
	if err := e.ensureNetwork(ctx, plan); err != nil {
		return err
	}
	var runErr error
	for attempts := 0; attempts < 3; attempts++ {
		_, runErr = e.execDocker(ctx, buildDockerRunArgs(plan.Name, e.stateMountPath, e.unlockProofSecret, e.serviceSecurity, task)...)
		if runErr == nil {
			return nil
		}
		if !strings.Contains(strings.ToLower(runErr.Error()), "conflict") {
			return runErr
		}
		// Explicitly remove the conflicting container before retrying,
		// otherwise every retry will fail with the same naming conflict.
		_ = e.removeContainerIfExists(ctx, task.ContainerName)
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
	cmd := exec.CommandContext(ctx, e.binary, args...) // #nosec G204,G702 -- docker binary is host operator configuration; args are passed without shell expansion.
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("%s %s failed: %w: %s", e.binary, strings.Join(args, " "), err, strings.TrimSpace(string(output)))
	}
	return output, nil
}

func (e *dockerCLIExecutor) removeVolumeIfExists(ctx context.Context, volumeName string) error {
	if strings.TrimSpace(volumeName) == "" {
		return nil
	}
	output, err := e.execDocker(ctx, "volume", "ls", "--filter", fmt.Sprintf("name=^%s$", volumeName), "--format", "{{.Name}}")
	if err != nil {
		return err
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
	return err
}

func buildDockerRunArgs(network, stateMountPath, unlockProofSecret string, security dockerRunSecurity, task apigateway.ControllerRuntimeTask) []string {
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
		"-e", fmt.Sprintf("AD_PLATFORM_UNLOCK_PROOF=%s", unlockproof.Issue(unlockProofSecret, task.TeamID, task.ChallengeID, task.UnlockProofEpoch)),
		"-e", fmt.Sprintf("AD_CHECKER_TOKEN=%s", task.CheckerToken),
		"-e", fmt.Sprintf("PORT=%d", servicePort),
	)
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

func buildDockerSSHCredentialArgs(task apigateway.ControllerRuntimeTask, credential apigateway.ControllerSSHCredential) []string {
	return []string{
		"exec",
		"-e", fmt.Sprintf("AD_PLATFORM_ROOT_PASSWORD=%s", credential.Password),
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
