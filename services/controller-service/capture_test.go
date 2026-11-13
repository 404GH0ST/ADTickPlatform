package main

import (
	"context"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"adplatform/internal/services/apigateway"
)

func TestCaptureConfigRejectsUnsafeCapabilities(t *testing.T) {
	e := &dockerCLIExecutor{serviceSecurity: testServiceSecurity()}
	e.serviceSecurity.capAdd = append(e.serviceSecurity.capAdd, "NET_RAW")
	if e.validateCaptureConfig() == nil {
		t.Fatal("unguarded NET_RAW accepted")
	}
	e.serviceSecurity = testServiceSecurity()
	e.serviceSecurity.capDrop = nil
	if e.validateCaptureConfig() == nil {
		t.Fatal("Docker default raw access accepted in disabled mode")
	}
	e.serviceSecurity = testServiceSecurity()
	e.serviceSecurity.capAdd = append(e.serviceSecurity.capAdd, "ALL")
	if e.validateCaptureConfig() == nil {
		t.Fatal("ALL bypassed disabled capture")
	}
	e.serviceSecurity = testServiceSecurity()
	e.captureMode = "guarded"
	e.captureVolume = "volume"
	e.captureTool = "policy"
	e.captureObject = "object"
	if err := e.validateCaptureConfig(); err != nil {
		t.Fatal(err)
	}
	for _, cap := range []string{"NET_ADMIN", "SYS_ADMIN", "BPF", "PERFMON", "SYS_PTRACE", "ALL"} {
		e.serviceSecurity = testServiceSecurity()
		e.serviceSecurity.capAdd = append(e.serviceSecurity.capAdd, cap)
		if e.validateCaptureConfig() == nil {
			t.Fatalf("accepted %s", cap)
		}
	}
	e.serviceSecurity = testServiceSecurity()
	e.serviceSecurity.securityOpt = append(e.serviceSecurity.securityOpt, "seccomp=unconfined")
	if e.validateCaptureConfig() == nil {
		t.Fatal("accepted disabled seccomp")
	}
}

// Explicit opt-in. Run only in the disposable test control container created by
// scripts/smoke-live-capture.sh. Every resource has its own random owner label.
func TestCaptureDockerLifecycle(t *testing.T) {
	if os.Getenv("CAPTURE_LIFECYCLE_TEST") != "1" {
		t.Skip("requires isolated privileged Docker test environment")
	}
	prefix := os.Getenv("CAPTURE_TEST_PREFIX")
	if !strings.HasPrefix(prefix, "adcap-test-") {
		t.Fatal("disposable resource prefix required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	e := &dockerCLIExecutor{binary: "docker", network: prefix, networkLayout: "per-service", stateMountPath: "/opt/ad/state", serviceSecurity: testServiceSecurity(), sshContractMode: "verify", sshApplyMode: "docker-exec", timeout: 20 * time.Second, captureMode: "guarded", captureVolume: prefix + "-bootstrap", captureTool: "/usr/local/bin/adplatform-capture-policy", captureObject: "/usr/local/share/adplatform/capture.bpf.o", captureProcRoot: "/host/proc", captureNetnsRoot: "/host/docker-netns", captureNetnsHostRoot: "/var/run/docker/netns"}
	task := apigateway.ControllerRuntimeTask{TeamID: 9001, ChallengeID: 251, ContainerName: prefix + "-service", StateVolume: prefix + "-state", BaselineImage: os.Getenv("CAPTURE_TEST_IMAGE"), SSHHost: os.Getenv("CAPTURE_TEST_IP"), Endpoint: os.Getenv("CAPTURE_TEST_IP") + ":8080", CheckerToken: "disposable-checker-token", RuntimeKind: "docker"}
	docker := func(args ...string) string {
		t.Helper()
		out, err := e.execDocker(ctx, args...)
		if err != nil {
			t.Fatal(err)
		}
		return strings.TrimSpace(string(out))
	}
	t.Cleanup(func() {
		e.execDocker(context.Background(), "rm", "-f", task.ContainerName)
		e.execDocker(context.Background(), "volume", "rm", task.StateVolume)
		plan, _ := e.networkPlan(task)
		e.execDocker(context.Background(), "network", "rm", plan.Name)
	})
	if err := e.EnsureService(ctx, task); err != nil {
		t.Fatal(err)
	}
	t.Log("fresh deployment released only after exact send/receive guard verification")
	docker("exec", task.ContainerName, "sh", "-c", "test ! -e /opt/ad/state/unsafe-start && test ! -e /opt/ad/state/unsafe-healthcheck")
	// Empty trusted storage must not trigger Docker image-to-volume copy-up.
	originalVolume := e.captureVolume
	emptyVolume := prefix + "-empty-bootstrap"
	badTask := task
	badTask.ContainerName = prefix + "-bad-service"
	badTask.StateVolume = prefix + "-bad-state"
	docker("volume", "create", "--label", "adplatform.capture-bootstrap=true", emptyVolume)
	t.Cleanup(func() {
		e.execDocker(context.Background(), "rm", "-f", badTask.ContainerName)
		e.execDocker(context.Background(), "volume", "rm", emptyVolume, badTask.StateVolume)
	})
	e.captureVolume = emptyVolume
	if err := e.EnsureService(ctx, badTask); err == nil {
		t.Fatal("empty bootstrap volume allowed image-controlled code")
	}
	docker("run", "--rm", "--network", "none", "--cap-drop", "ALL", "--no-healthcheck", "--entrypoint", "/bin/sh", "--mount", "type=volume,src="+badTask.StateVolume+",dst=/state,readonly,volume-nocopy", task.BaselineImage, "-c", "test ! -e /state/unsafe-start && test ! -e /state/unsafe-healthcheck")
	e.captureVolume = originalVolume
	t.Log("image-supplied startup replacement and healthcheck cannot bypass the gate")

	info, err := e.inspectCapture(ctx, task.ContainerName)
	if err != nil {
		t.Fatal(err)
	}
	group, ns, err := e.capturePaths(info, task)
	if err != nil {
		t.Fatal(err)
	}
	alias := info.Mounts[0]
	alias.Name = e.captureVolume
	alias.Destination = "/unsafe-alias"
	alias.RW = true
	info.Mounts = append(info.Mounts, alias)
	if _, _, err = e.capturePaths(info, task); err == nil {
		t.Fatal("accepted writable bootstrap alias")
	}
	info.Mounts = info.Mounts[:len(info.Mounts)-1]
	if !strings.HasPrefix(group, "/sys/fs/cgroup/") {
		t.Fatalf("bad cgroup: %s", group)
	}
	for _, args := range [][]string{{"check", group, e.captureObject}, {"rx-check", ns, "eth0", task.SSHHost, e.captureObject}} {
		if err = e.capturePolicy(ctx, args...); err != nil {
			t.Fatal(err)
		}
	}
	if e.capturePolicy(ctx, "rx-check", ns, "eth0", "192.0.2.99", e.captureObject) == nil {
		t.Fatal("accepted wrong receive IP")
	}
	if _, err = e.execDocker(ctx, "exec", task.ContainerName, "sh", "-c", "echo tamper > "+captureGate); err == nil {
		t.Fatal("root modified immutable gate")
	}
	docker("exec", task.ContainerName, "python3", "-c", `import socket,errno
s=socket.socket(socket.AF_PACKET,socket.SOCK_RAW);s.bind(('eth0',0))
try:s.send(b'X'*64)
except PermissionError:pass
else:raise AssertionError('packet injection allowed')`)
	docker("exec", task.ContainerName, "sh", "-c", "echo patched >/root/capture-patch; echo saved >/opt/ad/state/capture-state")
	if err = e.RestartService(ctx, task); err != nil {
		t.Fatal(err)
	}
	if docker("exec", task.ContainerName, "cat", "/root/capture-patch") != "patched" {
		t.Fatal("restart lost patch")
	}
	if err = e.EnsureService(ctx, task); err != nil {
		t.Fatal(err)
	}
	if docker("exec", task.ContainerName, "cat", "/root/capture-patch") != "patched" {
		t.Fatal("reconcile lost patch")
	}
	t.Log("restart and controller reconciliation preserve patches and guards")
	// Simulate Docker restart without the controller: patched application code
	// cannot run until the immutable gate gets a fresh verified release.
	docker("exec", task.ContainerName, "sh", "-c", "sed -i '2i echo booted >> /opt/ad/state/boots' /app/entrypoint.sh")
	docker("restart", task.ContainerName)
	time.Sleep(300 * time.Millisecond)
	if _, err = e.execDocker(ctx, "exec", task.ContainerName, captureGate, "release", strings.Repeat("0", 64)); err == nil {
		t.Fatal("invalid boot nonce released application")
	}

	if _, err = e.execDocker(ctx, "exec", task.ContainerName, "test", "-e", "/opt/ad/state/boots"); err == nil {
		t.Fatal("application ran before guard recovery")
	}
	if err = e.sweepCapture(ctx); err != nil {
		t.Fatal(err)
	}
	docker("exec", task.ContainerName, "sh", "-c", "for i in 1 2 3 4 5; do test -e /opt/ad/state/boots && exit 0; sleep .1; done; exit 1")
	// Root tcpdump receives real incoming HTTP traffic on the live interface.
	capture := exec.CommandContext(ctx, "docker", "exec", task.ContainerName, "tcpdump", "-p", "-Q", "in", "-i", "eth0", "-nn", "-c", "1", "tcp port 8080")
	if err = capture.Start(); err != nil {
		t.Fatal(err)
	}
	time.Sleep(500 * time.Millisecond)
	docker("exec", task.ContainerName, "python3", "-c", `from urllib.request import urlopen;urlopen('http://`+task.SSHHost+`:8080/health').read()`)
	// The previous request is local loopback; send from host to exercise eth0.
	request := exec.CommandContext(ctx, "wget", "-q", "-O", "/dev/null", "http://"+task.Endpoint+"/health")
	if out, err := request.CombinedOutput(); err != nil {
		t.Fatalf("HTTP: %v %s", err, out)
	}
	if err = capture.Wait(); err != nil {
		t.Fatal(err)
	}
	t.Log("ordinary tcpdump captured live incoming traffic")
	// Remove policies as the trusted operator and deliberately break the loader:
	// recovery must restart into the gate and keep application code blocked.
	info, err = e.inspectCapture(ctx, task.ContainerName)
	if err != nil {
		t.Fatal(err)
	}
	group, _, err = e.capturePaths(info, task)
	if err != nil {
		t.Fatal(err)
	}
	if err = e.capturePolicy(ctx, "detach", group); err != nil {
		t.Fatal(err)
	}
	e.captureSeen = nil
	originalTool := e.captureTool
	e.captureTool = "/bin/false"
	if err = e.sweepCapture(ctx); err == nil {
		t.Fatal("missing policy accepted")
	}
	info, _ = e.inspectCapture(ctx, task.ContainerName)
	if info.State.Running {
		t.Fatal("guard failure did not stop container")
	}
	e.captureTool = originalTool
	if err = e.EnsureService(ctx, task); err != nil {
		t.Fatal(err)
	}
	if err = e.FactoryResetService(ctx, task); err != nil {
		t.Fatal(err)
	}
	if _, err = e.execDocker(ctx, "exec", task.ContainerName, "test", "-e", "/root/capture-patch"); err == nil {
		t.Fatal("factory reset retained patch")
	}
	if _, err = e.execDocker(ctx, "exec", task.ContainerName, "test", "-e", "/opt/ad/state/capture-state"); err == nil {
		t.Fatal("factory reset retained state")
	}
	t.Log("guard failure stops the instance; recovery and factory reset reapply policies")
}
