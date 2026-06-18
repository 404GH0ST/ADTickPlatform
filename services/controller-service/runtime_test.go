package main

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"adplatform/internal/platform/unlockproof"
	"adplatform/internal/services/apigateway"
)

type stubCheckerValidationClient struct {
	result apigateway.CheckerValidationResult
	err    error
	called bool
}

func testServiceSecurity() dockerRunSecurity {
	return dockerRunSecurity{
		capDrop:   []string{"ALL"},
		capAdd:    []string{"CHOWN", "DAC_OVERRIDE", "FOWNER", "SETGID", "SETUID", "NET_BIND_SERVICE", "SYS_CHROOT", "AUDIT_WRITE"},
		pidsLimit: "256",
		memory:    "512m",
		cpus:      "1.0",
	}
}

func (c *stubCheckerValidationClient) Validate(_ context.Context, _ apigateway.CheckerValidationRequest) (apigateway.CheckerValidationResult, error) {
	c.called = true
	return c.result, c.err
}

func TestBuildDockerRunArgsWithNetworkAndIP(t *testing.T) {
	task := apigateway.ControllerRuntimeTask{
		DeploymentJobID: 9,
		TeamID:          101,
		ChallengeID:     3,
		ContainerName:   "svc-storage-team-101",
		StateVolume:     "svc-storage-team-101-state",
		BaselineImage:   "registry.local/storage:baseline",
		CheckerToken:    "checker-secret-101-3",
		Endpoint:        "10.80.3.11:10003",
		SSHHost:         "10.80.3.11",
	}

	args := buildDockerRunArgs("adplatform_game_svc_003", "/opt/ad/state", "dev-unlock-secret", testServiceSecurity(), task)

	if len(args) == 0 || args[0] != "run" {
		t.Fatalf("expected docker run args, got %v", args)
	}
	if !slices.Contains(args, "--restart") || !slices.Contains(args, "unless-stopped") {
		t.Fatalf("expected restart policy args, got %v", args)
	}
	if !slices.Contains(args, "--network") || !slices.Contains(args, "adplatform_game_svc_003") {
		t.Fatalf("expected network args, got %v", args)
	}
	if !slices.Contains(args, "--ip") || !slices.Contains(args, "10.80.3.11") {
		t.Fatalf("expected ip args, got %v", args)
	}
	if !slices.Contains(args, "--mount") || !slices.Contains(args, "type=volume,src=svc-storage-team-101-state,dst=/opt/ad/state") {
		t.Fatalf("expected state volume mount args, got %v", args)
	}
	if !slices.Contains(args, "-e") || !slices.Contains(args, "AD_PLATFORM_UNLOCK_PROOF="+unlockproof.Issue("dev-unlock-secret", 101, 3)) {
		t.Fatalf("expected unlock proof env args, got %v", args)
	}
	if !slices.Contains(args, "AD_PLATFORM_SERVICE_IP=10.80.3.11") || !slices.Contains(args, "AD_PLATFORM_SERVICE_PORT=10003") || !slices.Contains(args, "AD_CHECKER_TOKEN=checker-secret-101-3") || !slices.Contains(args, "PORT=10003") {
		t.Fatalf("expected service ip/port env args, got %v", args)
	}
	if args[len(args)-1] != task.BaselineImage {
		t.Fatalf("expected image %s, got %s", task.BaselineImage, args[len(args)-1])
	}
	assertArgSequence(t, args, []string{
		"run", "-d", "--restart", "unless-stopped",
		"--cap-drop", "ALL",
		"--cap-add", "CHOWN",
		"--cap-add", "DAC_OVERRIDE",
		"--cap-add", "FOWNER",
		"--cap-add", "SETGID",
		"--cap-add", "SETUID",
		"--cap-add", "NET_BIND_SERVICE",
		"--cap-add", "SYS_CHROOT",
		"--cap-add", "AUDIT_WRITE",
		"--pids-limit", "256",
		"--memory", "512m",
		"--cpus", "1.0",
		"--name", task.ContainerName,
	})
}

func TestBuildDockerRunArgsWithoutNetwork(t *testing.T) {
	task := apigateway.ControllerRuntimeTask{
		DeploymentJobID: 4,
		TeamID:          102,
		ChallengeID:     2,
		ContainerName:   "svc-chat-team-102",
		BaselineImage:   "registry.local/chat:baseline",
		Endpoint:        "10.80.2.12:10002",
		SSHHost:         "10.80.2.12",
	}

	args := buildDockerRunArgs("", "", "dev-unlock-secret", testServiceSecurity(), task)

	if slices.Contains(args, "--network") || slices.Contains(args, "--ip") || slices.Contains(args, "--mount") {
		t.Fatalf("did not expect network, ip, or mount args, got %v", args)
	}
}

func TestDockerFactoryResetRemovesVolumeAndRecreatesContainer(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "docker.log")
	binPath := filepath.Join(t.TempDir(), "docker")
	script := "#!/bin/sh\n" +
		"printf '%s\\n' \"$*\" >> \"$DOCKER_LOG\"\n" +
		"case \"$1\" in\n" +
		"  ps)\n" +
		"    printf '%s\\n' \"$DOCKER_CONTAINER_NAME\"\n" +
		"    ;;\n" +
		"  network)\n" +
		"    if [ \"$2\" = \"inspect\" ]; then\n" +
		"      exit 1\n" +
		"    fi\n" +
		"    ;;\n" +
		"  volume)\n" +
		"    if [ \"$2\" = \"ls\" ]; then\n" +
		"      printf '%s\\n' \"$DOCKER_VOLUME_NAME\"\n" +
		"    fi\n" +
		"    ;;\n" +
		"esac\n"
	if err := os.WriteFile(binPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake docker: %v", err)
	}

	task := apigateway.ControllerRuntimeTask{
		TeamID:        101,
		ChallengeID:   3,
		ContainerName: "svc-storage-team-101",
		StateVolume:   "svc-storage-team-101-state",
		BaselineImage: "registry.local/storage:baseline",
		CheckerToken:  "checker-secret-101-3",
		Endpoint:      "10.80.3.11:10003",
		SSHHost:       "10.80.3.11",
	}
	executor := &dockerCLIExecutor{
		binary:            binPath,
		network:           "adplatform_game",
		networkLayout:     "per-service",
		stateMountPath:    "/opt/ad/state",
		unlockProofSecret: "dev-unlock-secret",
		serviceSecurity:   testServiceSecurity(),
		timeout:           5 * time.Second,
	}
	t.Setenv("DOCKER_LOG", logPath)
	t.Setenv("DOCKER_CONTAINER_NAME", task.ContainerName)
	t.Setenv("DOCKER_VOLUME_NAME", task.StateVolume)

	if err := executor.FactoryResetService(context.Background(), task); err != nil {
		t.Fatalf("factory reset failed: %v", err)
	}

	logBytes, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read fake docker log: %v", err)
	}
	logOutput := string(logBytes)
	expected := []string{
		"ps -a --filter name=^/svc-storage-team-101$ --format {{.Names}}",
		"rm -f svc-storage-team-101",
		"volume ls --filter name=^svc-storage-team-101-state$ --format {{.Name}}",
		"volume rm -f svc-storage-team-101-state",
		"network inspect adplatform_game_svc_003",
		"network create --label adplatform.game_network=true --label adplatform.network_layout=per-service --subnet 10.80.3.0/24 adplatform_game_svc_003",
		"run -d --restart unless-stopped --cap-drop ALL --cap-add CHOWN --cap-add DAC_OVERRIDE --cap-add FOWNER --cap-add SETGID --cap-add SETUID --cap-add NET_BIND_SERVICE --cap-add SYS_CHROOT --cap-add AUDIT_WRITE --pids-limit 256 --memory 512m --cpus 1.0 --name svc-storage-team-101 --hostname svc-storage-team-101",
		"-e AD_PLATFORM_UNLOCK_PROOF=" + unlockproof.Issue("dev-unlock-secret", 101, 3),
		"-e AD_CHECKER_TOKEN=checker-secret-101-3",
		"--mount type=volume,src=svc-storage-team-101-state,dst=/opt/ad/state",
		"--network adplatform_game_svc_003 --ip 10.80.3.11 registry.local/storage:baseline",
	}
	for _, fragment := range expected {
		if !strings.Contains(logOutput, fragment) {
			t.Fatalf("expected log fragment %q in %q", fragment, logOutput)
		}
	}
}

func TestBuildDockerSSHCredentialArgs(t *testing.T) {
	task := apigateway.ControllerRuntimeTask{
		TeamID:        101,
		ChallengeID:   1,
		ContainerName: "svc-banking-team-101",
	}
	credential := apigateway.ControllerSSHCredential{
		Password: "one-time-secret",
	}

	args := buildDockerSSHCredentialArgs(task, credential)

	expected := []string{
		"exec",
		"-e",
		"AD_PLATFORM_ROOT_PASSWORD=one-time-secret",
		"svc-banking-team-101",
		"/bin/sh",
		"-lc",
	}
	for _, fragment := range expected {
		if !slices.Contains(args, fragment) {
			t.Fatalf("expected fragment %q in args %v", fragment, args)
		}
	}
	if !strings.Contains(args[len(args)-1], "chpasswd") || !strings.Contains(args[len(args)-1], "passwd root") {
		t.Fatalf("expected password apply script, got %q", args[len(args)-1])
	}
}

func TestBuildDockerSSHContractProbeArgs(t *testing.T) {
	task := apigateway.ControllerRuntimeTask{
		TeamID:        101,
		ChallengeID:   1,
		ContainerName: "svc-banking-team-101",
	}

	args := buildDockerSSHContractProbeArgs(task)

	expected := []string{
		"exec",
		"svc-banking-team-101",
		"/bin/sh",
		"-lc",
	}
	for _, fragment := range expected {
		if !slices.Contains(args, fragment) {
			t.Fatalf("expected fragment %q in args %v", fragment, args)
		}
	}
	if !strings.Contains(args[len(args)-1], "sshd") || !strings.Contains(args[len(args)-1], "chpasswd") {
		t.Fatalf("expected ssh contract probe script, got %q", args[len(args)-1])
	}
}

func TestBuildDockerBaselineValidationArgs(t *testing.T) {
	args := buildDockerBaselineValidationArgs(apigateway.ChallengeValidationRequest{
		ChallengeID:   7,
		Name:          "proxy",
		BaselineImage: "registry.local/proxy:baseline",
		CheckerImage:  "registry.local/proxy-checker:latest",
	})

	expected := []string{
		"run",
		"--rm",
		"--entrypoint",
		"/bin/sh",
		"registry.local/proxy:baseline",
		"-lc",
	}
	for _, fragment := range expected {
		if !slices.Contains(args, fragment) {
			t.Fatalf("expected fragment %q in args %v", fragment, args)
		}
	}
	if !strings.Contains(args[len(args)-1], "missing ssh daemon binary inside image") || !strings.Contains(args[len(args)-1], "missing supported password setter inside image") {
		t.Fatalf("expected challenge validation script, got %q", args[len(args)-1])
	}
	assertArgSequence(t, args, []string{"run", "--rm", "--cap-drop", "ALL", "--security-opt", "no-new-privileges:true", "--pids-limit", "128", "--memory", "256m", "--cpus", "0.5", "--entrypoint"})
}

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
		if !slices.Contains(args, fragment) {
			t.Fatalf("expected fragment %q in args %v", fragment, args)
		}
	}
	if !strings.Contains(args[len(args)-1], "missing standard checker entrypoint inside image") {
		t.Fatalf("expected checker validation script, got %q", args[len(args)-1])
	}
	assertArgSequence(t, args, []string{"run", "--rm", "--cap-drop", "ALL", "--security-opt", "no-new-privileges:true", "--pids-limit", "128", "--memory", "256m", "--cpus", "0.5", "--entrypoint"})
}

func assertArgSequence(t *testing.T, args []string, expected []string) {
	t.Helper()
	if len(expected) > len(args) {
		t.Fatalf("expected sequence %v in args %v", expected, args)
	}
	for i := range expected {
		if args[i] != expected[i] {
			t.Fatalf("expected args prefix %v, got %v", expected, args[:len(expected)])
		}
	}
}

func TestDockerEnsureServiceVerifiesSSHContractAfterRun(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "docker.log")
	binPath := filepath.Join(t.TempDir(), "docker")
	script := "#!/bin/sh\n" +
		"printf '%s\\n' \"$*\" >> \"$DOCKER_LOG\"\n" +
		"case \"$1\" in\n" +
		"  ps)\n" +
		"    exit 0\n" +
		"    ;;\n" +
		"  network)\n" +
		"    if [ \"$2\" = \"inspect\" ]; then\n" +
		"      exit 1\n" +
		"    fi\n" +
		"    ;;\n" +
		"esac\n"
	if err := os.WriteFile(binPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake docker: %v", err)
	}

	task := apigateway.ControllerRuntimeTask{
		TeamID:        101,
		ChallengeID:   1,
		ContainerName: "svc-banking-team-101",
		BaselineImage: "registry.local/banking:baseline",
		Endpoint:      "10.80.1.11:10001",
		SSHHost:       "10.80.1.11",
	}
	executor := &dockerCLIExecutor{
		binary:          binPath,
		network:         "adplatform_game",
		networkLayout:   "per-service",
		sshContractMode: "verify",
		timeout:         5 * time.Second,
	}
	t.Setenv("DOCKER_LOG", logPath)

	if err := executor.EnsureService(context.Background(), task); err != nil {
		t.Fatalf("ensure service failed: %v", err)
	}

	logBytes, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read fake docker log: %v", err)
	}
	logOutput := string(logBytes)
	expected := []string{
		"ps -a --filter name=^/svc-banking-team-101$ --format {{.Names}}",
		"network inspect adplatform_game_svc_001",
		"network create --label adplatform.game_network=true --label adplatform.network_layout=per-service --subnet 10.80.1.0/24 adplatform_game_svc_001",
		"--name svc-banking-team-101 --hostname svc-banking-team-101",
		"exec svc-banking-team-101 /bin/sh -lc",
	}
	for _, fragment := range expected {
		if !strings.Contains(logOutput, fragment) {
			t.Fatalf("expected log fragment %q in %q", fragment, logOutput)
		}
	}
}

func TestDockerEnsureServiceRecreatesExistingContainer(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "docker.log")
	binPath := filepath.Join(t.TempDir(), "docker")
	script := "#!/bin/sh\n" +
		"printf '%s\\n' \"$*\" >> \"$DOCKER_LOG\"\n" +
		"case \"$1\" in\n" +
		"  ps)\n" +
		"    printf '%s\\n' svc-banking-team-101\n" +
		"    ;;\n" +
		"  network)\n" +
		"    if [ \"$2\" = \"inspect\" ]; then\n" +
		"      exit 1\n" +
		"    fi\n" +
		"    ;;\n" +
		"esac\n"
	if err := os.WriteFile(binPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake docker: %v", err)
	}

	task := apigateway.ControllerRuntimeTask{
		TeamID:        101,
		ChallengeID:   1,
		ContainerName: "svc-banking-team-101",
		BaselineImage: "registry.local/banking:baseline",
		Endpoint:      "10.80.1.11:10001",
		SSHHost:       "10.80.1.11",
	}
	executor := &dockerCLIExecutor{
		binary:          binPath,
		network:         "adplatform_game",
		networkLayout:   "per-service",
		sshContractMode: "verify",
		timeout:         5 * time.Second,
	}
	t.Setenv("DOCKER_LOG", logPath)

	if err := executor.EnsureService(context.Background(), task); err != nil {
		t.Fatalf("ensure service failed: %v", err)
	}

	logBytes, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read fake docker log: %v", err)
	}
	logOutput := string(logBytes)
	for _, fragment := range []string{
		"ps -a --filter name=^/svc-banking-team-101$ --format {{.Names}}",
		"rm -f svc-banking-team-101",
		"pull registry.local/banking:baseline",
		"run -d",
		"exec svc-banking-team-101 /bin/sh -lc",
	} {
		if !strings.Contains(logOutput, fragment) {
			t.Fatalf("expected log fragment %q in %q", fragment, logOutput)
		}
	}
}

func TestDockerEnsureServiceFallsBackToLocalImageWhenPullFails(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "docker.log")
	binPath := filepath.Join(t.TempDir(), "docker")
	script := "#!/bin/sh\n" +
		"printf '%s\\n' \"$*\" >> \"$DOCKER_LOG\"\n" +
		"case \"$1\" in\n" +
		"  ps)\n" +
		"    exit 0\n" +
		"    ;;\n" +
		"  pull)\n" +
		"    echo 'pull unavailable' >&2\n" +
		"    exit 1\n" +
		"    ;;\n" +
		"  image)\n" +
		"    if [ \"$2\" = \"inspect\" ]; then\n" +
		"      exit 0\n" +
		"    fi\n" +
		"    ;;\n" +
		"  network)\n" +
		"    if [ \"$2\" = \"inspect\" ]; then\n" +
		"      exit 1\n" +
		"    fi\n" +
		"    ;;\n" +
		"esac\n"
	if err := os.WriteFile(binPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake docker: %v", err)
	}

	task := apigateway.ControllerRuntimeTask{
		TeamID:        101,
		ChallengeID:   1,
		ContainerName: "svc-banking-team-101",
		BaselineImage: "local/banking:baseline",
		Endpoint:      "10.80.1.11:10001",
		SSHHost:       "10.80.1.11",
	}
	executor := &dockerCLIExecutor{
		binary:          binPath,
		network:         "adplatform_game",
		networkLayout:   "per-service",
		sshContractMode: "verify",
		timeout:         5 * time.Second,
	}
	t.Setenv("DOCKER_LOG", logPath)

	if err := executor.EnsureService(context.Background(), task); err != nil {
		t.Fatalf("ensure service should use existing local image after pull failure: %v", err)
	}

	logBytes, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read fake docker log: %v", err)
	}
	logOutput := string(logBytes)
	for _, fragment := range []string{
		"pull local/banking:baseline",
		"image inspect local/banking:baseline",
		"run -d",
	} {
		if !strings.Contains(logOutput, fragment) {
			t.Fatalf("expected log fragment %q in %q", fragment, logOutput)
		}
	}
}

func TestDockerEnsureServiceRetriesSSHContractWhileContainerRestarts(t *testing.T) {
	tempDir := t.TempDir()
	logPath := filepath.Join(tempDir, "docker.log")
	countPath := filepath.Join(tempDir, "exec.count")
	binPath := filepath.Join(tempDir, "docker")
	script := "#!/bin/sh\n" +
		"printf '%s\\n' \"$*\" >> \"$DOCKER_LOG\"\n" +
		"case \"$1\" in\n" +
		"  ps)\n" +
		"    exit 0\n" +
		"    ;;\n" +
		"  network)\n" +
		"    if [ \"$2\" = \"inspect\" ]; then\n" +
		"      exit 1\n" +
		"    fi\n" +
		"    ;;\n" +
		"  exec)\n" +
		"    count=0\n" +
		"    if [ -f \"$EXEC_COUNT\" ]; then count=$(cat \"$EXEC_COUNT\"); fi\n" +
		"    count=$((count + 1))\n" +
		"    printf '%s' \"$count\" > \"$EXEC_COUNT\"\n" +
		"    if [ \"$count\" -eq 1 ]; then\n" +
		"      echo 'Error response from daemon: Container abc is restarting, wait until the container is running' >&2\n" +
		"      exit 1\n" +
		"    fi\n" +
		"    exit 0\n" +
		"    ;;\n" +
		"esac\n"
	if err := os.WriteFile(binPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake docker: %v", err)
	}

	task := apigateway.ControllerRuntimeTask{
		TeamID:        101,
		ChallengeID:   1,
		ContainerName: "svc-banking-team-101",
		BaselineImage: "registry.local/banking:baseline",
		Endpoint:      "10.80.1.11:10001",
		SSHHost:       "10.80.1.11",
	}
	executor := &dockerCLIExecutor{
		binary:          binPath,
		network:         "adplatform_game",
		networkLayout:   "per-service",
		sshContractMode: "verify",
		timeout:         3 * time.Second,
	}
	t.Setenv("DOCKER_LOG", logPath)
	t.Setenv("EXEC_COUNT", countPath)

	if err := executor.EnsureService(context.Background(), task); err != nil {
		t.Fatalf("ensure service failed after transient restart: %v", err)
	}
	countBytes, err := os.ReadFile(countPath)
	if err != nil {
		t.Fatalf("read exec count: %v", err)
	}
	if strings.TrimSpace(string(countBytes)) != "2" {
		t.Fatalf("expected ssh probe to retry once, got count %q", string(countBytes))
	}
}

func TestDockerEnsureServiceTimesOutWhenContainerKeepsRestarting(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "docker.log")
	binPath := filepath.Join(t.TempDir(), "docker")
	script := "#!/bin/sh\n" +
		"printf '%s\\n' \"$*\" >> \"$DOCKER_LOG\"\n" +
		"case \"$1\" in\n" +
		"  ps)\n" +
		"    exit 0\n" +
		"    ;;\n" +
		"  network)\n" +
		"    if [ \"$2\" = \"inspect\" ]; then\n" +
		"      exit 1\n" +
		"    fi\n" +
		"    ;;\n" +
		"  exec)\n" +
		"    echo 'Error response from daemon: Container abc is restarting, wait until the container is running' >&2\n" +
		"    exit 1\n" +
		"    ;;\n" +
		"esac\n"
	if err := os.WriteFile(binPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake docker: %v", err)
	}

	task := apigateway.ControllerRuntimeTask{
		TeamID:        101,
		ChallengeID:   1,
		ContainerName: "svc-banking-team-101",
		BaselineImage: "registry.local/banking:baseline",
		Endpoint:      "10.80.1.11:10001",
		SSHHost:       "10.80.1.11",
	}
	executor := &dockerCLIExecutor{
		binary:          binPath,
		network:         "adplatform_game",
		networkLayout:   "per-service",
		sshContractMode: "verify",
		timeout:         time.Second,
	}
	t.Setenv("DOCKER_LOG", logPath)

	err := executor.EnsureService(context.Background(), task)
	if err == nil {
		t.Fatal("expected timeout while container keeps restarting")
	}
	if !strings.Contains(err.Error(), "did not become ready for SSH contract verification") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDockerApplySSHCredentialVerifiesContractBeforePasswordSet(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "docker.log")
	binPath := filepath.Join(t.TempDir(), "docker")
	script := "#!/bin/sh\n" +
		"printf '%s\\n' \"$*\" >> \"$DOCKER_LOG\"\n" +
		"case \"$1\" in\n" +
		"  ps)\n" +
		"    printf '%s\\n' \"$DOCKER_CONTAINER_NAME\"\n" +
		"    ;;\n" +
		"esac\n"
	if err := os.WriteFile(binPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake docker: %v", err)
	}

	task := apigateway.ControllerRuntimeTask{
		TeamID:        101,
		ChallengeID:   1,
		ContainerName: "svc-banking-team-101",
		BaselineImage: "registry.local/banking:baseline",
		Endpoint:      "10.80.1.11:10001",
		SSHHost:       "10.80.1.11",
	}
	executor := &dockerCLIExecutor{
		binary:          binPath,
		network:         "adplatform_game",
		networkLayout:   "per-service",
		sshApplyMode:    "docker-exec",
		sshContractMode: "verify",
		timeout:         5 * time.Second,
	}
	t.Setenv("DOCKER_LOG", logPath)
	t.Setenv("DOCKER_CONTAINER_NAME", task.ContainerName)

	if err := executor.ApplySSHCredential(context.Background(), task, apigateway.ControllerSSHCredential{
		Password: "one-time-secret",
	}); err != nil {
		t.Fatalf("apply ssh credential failed: %v", err)
	}

	logBytes, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read fake docker log: %v", err)
	}
	logOutput := string(logBytes)
	expected := []string{
		"ps -a --filter name=^/svc-banking-team-101$ --format {{.Names}}",
		"exec svc-banking-team-101 /bin/sh -lc",
		"exec -e AD_PLATFORM_ROOT_PASSWORD=one-time-secret svc-banking-team-101 /bin/sh -lc",
	}
	for _, fragment := range expected {
		if !strings.Contains(logOutput, fragment) {
			t.Fatalf("expected log fragment %q in %q", fragment, logOutput)
		}
	}
}

func TestDockerValidateChallengeRuntimeRunsEphemeralImageProbe(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "docker.log")
	binPath := filepath.Join(t.TempDir(), "docker")
	script := "#!/bin/sh\n" +
		"printf '%s\\n' \"$*\" >> \"$DOCKER_LOG\"\n" +
		"printf 'ADPLATFORM_CHECKER_CAPABILITIES={\"service_state\":true}\\n'\n"
	if err := os.WriteFile(binPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake docker: %v", err)
	}

	executor := &dockerCLIExecutor{
		binary:  binPath,
		timeout: 5 * time.Second,
	}
	t.Setenv("DOCKER_LOG", logPath)

	result, err := executor.ValidateChallengeRuntime(context.Background(), apigateway.ChallengeValidationRequest{
		ChallengeID:   7,
		Name:          "proxy",
		BaselineImage: "registry.local/proxy:baseline",
		CheckerImage:  "registry.local/proxy-checker:latest",
	})
	if err != nil {
		t.Fatalf("validate challenge runtime failed: %v", err)
	}
	if result.Status != "valid" || !result.BaselineSSHContractOK || !result.CheckerContractOK || !result.ServiceStateContractOK {
		t.Fatalf("unexpected validation result %+v", result)
	}

	logBytes, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read fake docker log: %v", err)
	}
	logOutput := string(logBytes)
	expected := []string{
		"--entrypoint /bin/sh registry.local/proxy:baseline -lc",
		"missing ssh daemon binary inside image",
		"--entrypoint /bin/sh registry.local/proxy-checker:latest -lc",
		"missing standard checker entrypoint inside image",
	}
	for _, fragment := range expected {
		if !strings.Contains(logOutput, fragment) {
			t.Fatalf("expected log fragment %q in %q", fragment, logOutput)
		}
	}
}

func TestDockerValidateChallengeRuntimeRejectsMissingCheckerImage(t *testing.T) {
	executor := &dockerCLIExecutor{timeout: 5 * time.Second}

	result, err := executor.ValidateChallengeRuntime(context.Background(), apigateway.ChallengeValidationRequest{
		ChallengeID:   7,
		Name:          "proxy",
		BaselineImage: "registry.local/proxy:baseline",
	})
	if err != nil {
		t.Fatalf("validate challenge runtime failed: %v", err)
	}
	if result.Status != "invalid" || result.Message != "checker image is required for runtime validation." {
		t.Fatalf("unexpected validation result %+v", result)
	}
}

func TestDockerValidateChallengeRuntimeUsesCheckerRunnerClientWhenConfigured(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "docker.log")
	binPath := filepath.Join(t.TempDir(), "docker")
	script := "#!/bin/sh\n" +
		"printf '%s\\n' \"$*\" >> \"$DOCKER_LOG\"\n"
	if err := os.WriteFile(binPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake docker: %v", err)
	}

	checkerClient := &stubCheckerValidationClient{
		result: apigateway.CheckerValidationResult{
			Status:                 "valid",
			ContractOK:             true,
			ServiceStateContractOK: true,
			Message:                "validated by checker-runner",
		},
	}
	executor := &dockerCLIExecutor{
		binary:           binPath,
		checkerValidator: checkerClient,
		timeout:          5 * time.Second,
	}
	t.Setenv("DOCKER_LOG", logPath)

	result, err := executor.ValidateChallengeRuntime(context.Background(), apigateway.ChallengeValidationRequest{
		ChallengeID:   7,
		Name:          "proxy",
		BaselineImage: "registry.local/proxy:baseline",
		CheckerImage:  "registry.local/proxy-checker:latest",
	})
	if err != nil {
		t.Fatalf("validate challenge runtime failed: %v", err)
	}
	if result.Status != "valid" || !checkerClient.called || !result.CheckerContractOK || !result.ServiceStateContractOK {
		t.Fatalf("unexpected validation result %+v", result)
	}

	logBytes, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read docker log: %v", err)
	}
	logOutput := string(logBytes)
	if !strings.Contains(logOutput, "--entrypoint /bin/sh registry.local/proxy:baseline -lc") {
		t.Fatalf("expected baseline validation docker call, got %q", logOutput)
	}
	if strings.Contains(logOutput, "registry.local/proxy-checker:latest") {
		t.Fatalf("expected checker image validation to come from checker-runner client, got %q", logOutput)
	}
}
