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
			unlockProofSecret: strings.TrimSpace(config.String("UNLOCK_PROOF_SECRET", config.String("TEAM_JWT_SECRET", config.String("TEAM_JWT_DEV_TOKEN", "dev-team-token")))),
			sshApplyMode:      strings.ToLower(strings.TrimSpace(config.String("CONTROLLER_SSH_PASSWORD_APPLY_MODE", "docker-exec"))),
			sshContractMode:   strings.ToLower(strings.TrimSpace(config.String("CONTROLLER_SSH_CONTRACT_MODE", "verify"))),
			checkerValidator:  newCheckerValidationClient(),
			timeout:           config.Duration("CONTROLLER_RUNTIME_TIMEOUT", 15*time.Second),
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
	sshApplyMode      string
	sshContractMode   string
	checkerValidator  checkerValidationClient
	timeout           time.Duration
}

func (e *dockerCLIExecutor) EnsureService(ctx context.Context, task apigateway.ControllerRuntimeTask) error {
	if task.RuntimeKind != "" && task.RuntimeKind != "docker" {
		return fmt.Errorf("unsupported runtime kind %q for %s", task.RuntimeKind, task.ContainerName)
	}

	runCtx, cancel := context.WithTimeout(ctx, e.timeout)
	defer cancel()

	exists, err := e.containerExists(runCtx, task.ContainerName)
	if err != nil {
		return err
	}
	if exists {
		return e.verifySSHContract(runCtx, task)
	}

	if err := e.runFreshContainer(runCtx, task); err != nil {
		return err
	}
	return e.verifySSHContract(runCtx, task)
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
			return err
		}
		if stateVolume != "" {
			if err := e.removeVolumeIfExists(ctx, stateVolume); err != nil {
				// We don't return error here to continue cleaning up other containers
				log.Printf("failed to remove volume %q: %v", stateVolume, err)
			}
		}
	}

	return nil
}

func (e *dockerCLIExecutor) verifySSHContract(ctx context.Context, task apigateway.ControllerRuntimeTask) error {
	if e.sshContractMode == "disabled" {
		return nil
	}
	_, err := e.execDocker(ctx, buildDockerSSHContractProbeArgs(task)...)
	return err
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
	if err := e.ensureNetwork(ctx, plan); err != nil {
		return err
	}
	_, err = e.execDocker(ctx, buildDockerRunArgs(plan.Name, e.stateMountPath, e.unlockProofSecret, task)...)
	return err
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
	cmd := exec.CommandContext(ctx, e.binary, args...)
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

func buildDockerRunArgs(network, stateMountPath, unlockProofSecret string, task apigateway.ControllerRuntimeTask) []string {
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
		"--restart", "unless-stopped",
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
		"-e", fmt.Sprintf("AD_PLATFORM_UNLOCK_PROOF=%s", unlockproof.Issue(unlockProofSecret, task.TeamID, task.ChallengeID)),
		"-e", fmt.Sprintf("PORT=%d", servicePort),
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
	return []string{
		"run",
		"--rm",
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
	}
}

func buildDockerCheckerValidationArgs(request apigateway.CheckerValidationRequest) []string {
	return []string{
		"run",
		"--rm",
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
	}
}
