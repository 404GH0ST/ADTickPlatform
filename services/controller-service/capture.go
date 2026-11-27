package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"adplatform/internal/services/apigateway"
)

const captureMount = "/opt/adplatform-capture"
const captureGate = captureMount + "/start"

type captureStamp struct {
	Started  string
	Verified time.Time
}
type captureInspect struct {
	ID    string `json:"Id"`
	State struct {
		Pid       int
		Running   bool
		StartedAt string
	}
	Config struct {
		Entrypoint  []string
		Cmd         []string
		Env         []string
		User        string
		Healthcheck struct{ Test []string }
		Labels      map[string]string
	}
	HostConfig struct {
		CapAdd      []string
		CapDrop     []string
		SecurityOpt []string
		Privileged  bool
		Init        bool
		PidMode     string
		NetworkMode string
	}
	Mounts []struct {
		Type, Name, Destination string
		RW                      bool
	}
	NetworkSettings struct {
		SandboxKey string
		Networks   map[string]struct{ IPAddress string }
	}
}

// Use an independent, bounded deadline so cancellation of the failed operation
// cannot cancel the stop that enforces its failure boundary.
func (e *dockerCLIExecutor) stopCapturedService(name string) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if _, err := e.execDocker(ctx, "stop", "--time", "1", name); err != nil {
		log.Printf("cannot stop capture instance %s after guard failure: %v", name, err)
	}
}

func (e *dockerCLIExecutor) captureEnabled() bool { return e.captureMode == "guarded" }
func (e *dockerCLIExecutor) validateCaptureConfig() error {
	if e.captureMode != "" && e.captureMode != "disabled" && !e.captureEnabled() {
		return fmt.Errorf("unsupported CONTROLLER_CAPTURE_MODE %q", e.captureMode)
	}
	if !e.captureEnabled() {
		for _, cap := range e.serviceSecurity.capAdd {
			if normalizedCapability(cap) == "NET_RAW" || normalizedCapability(cap) == "ALL" {
				return fmt.Errorf("NET_RAW requires guarded capture mode")
			}
		}
		dropped := false
		for _, cap := range e.serviceSecurity.capDrop {
			if normalizedCapability(cap) == "ALL" || normalizedCapability(cap) == "NET_RAW" {
				dropped = true
			}
		}
		if !dropped {
			return fmt.Errorf("disabled capture requires dropping NET_RAW or ALL; Docker defaults include raw packet access")
		}
		return nil
	}
	if len(e.serviceSecurity.capDrop) != 1 || !strings.EqualFold(e.serviceSecurity.capDrop[0], "ALL") {
		return fmt.Errorf("guarded capture requires CONTROLLER_SERVICE_CAP_DROP=ALL")
	}
	allowed := []string{"CHOWN", "DAC_OVERRIDE", "FOWNER", "SETGID", "SETUID", "NET_BIND_SERVICE", "SYS_CHROOT", "AUDIT_WRITE", "NET_RAW"}
	for _, cap := range e.serviceSecurity.capAdd {
		if !slices.Contains(allowed, normalizedCapability(cap)) {
			return fmt.Errorf("capability %s is not allowed in guarded challenge containers", cap)
		}
	}
	if len(e.serviceSecurity.securityOpt) != 1 || e.serviceSecurity.securityOpt[0] != "no-new-privileges:true" {
		return fmt.Errorf("guarded capture requires no-new-privileges and the default Docker seccomp policy")
	}
	if e.captureVolume == "" || e.captureTool == "" || e.captureObject == "" {
		return fmt.Errorf("capture volume, policy tool, and BPF object are required")
	}
	return nil
}
func normalizedCapability(cap string) string { return strings.TrimPrefix(strings.ToUpper(cap), "CAP_") }
func (e *dockerCLIExecutor) inspectCapture(ctx context.Context, name string) (captureInspect, error) {
	var info captureInspect
	output, err := e.execDocker(ctx, "inspect", name)
	if err != nil {
		return info, err
	}
	var list []captureInspect
	if err = json.Unmarshal(output, &list); err != nil || len(list) != 1 {
		return info, fmt.Errorf("invalid capture runtime metadata")
	}
	return list[0], nil
}

// capturedCheckerToken returns the AD_CHECKER_TOKEN baked into the inspected
// container, or "" when absent. A guarded container's environment is fixed at
// creation, so this is the token the in-container checker gate authenticates
// with.
func capturedCheckerToken(info captureInspect) string {
	for _, kv := range info.Config.Env {
		if token, ok := strings.CutPrefix(kv, "AD_CHECKER_TOKEN="); ok {
			return token
		}
	}
	return ""
}

// checkerTokenCurrent reports whether a running guarded container may be kept
// by a warm reconcile, or must be recreated because its baked checker token no
// longer matches the task's. A redeploy rotates the database token; the token
// is baked into the container environment and cannot change in place, so a warm
// reconcile would leave the gate rejecting every checker put. An empty task
// token means no token is managed for this task, which matches.
func checkerTokenCurrent(info captureInspect, task apigateway.ControllerRuntimeTask) bool {
	return task.CheckerToken == "" || capturedCheckerToken(info) == task.CheckerToken
}
func (e *dockerCLIExecutor) capturePolicy(ctx context.Context, args ...string) error {
	output, err := exec.CommandContext(ctx, e.captureTool, args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("capture kernel policy %s failed: %w: %.500s", args[0], err, strings.TrimSpace(string(output)))
	}
	return nil
}
func (e *dockerCLIExecutor) capturedRunArgs(ctx context.Context, network string, task apigateway.ControllerRuntimeTask) ([]string, error) {
	if task.StateVolume == e.captureVolume {
		return nil, fmt.Errorf("challenge state cannot alias the trusted capture bootstrap volume")
	}
	volume, err := e.execDocker(ctx, "volume", "inspect", "--format", "{{json .Labels}}", e.captureVolume)
	if err != nil {
		return nil, fmt.Errorf("capture bootstrap volume is unavailable: %w", err)
	}
	var labels map[string]string
	if json.Unmarshal(volume, &labels) != nil || labels["adplatform.capture-bootstrap"] != "true" {
		return nil, fmt.Errorf("capture bootstrap volume must be initialized by the trusted deployment")
	}
	output, err := e.execDocker(ctx, "image", "inspect", "--format", "{{json .Config}}", task.BaselineImage)
	if err != nil {
		return nil, err
	}
	var image struct {
		Entrypoint, Cmd []string
		User            string
	}
	if err = json.Unmarshal(output, &image); err != nil {
		return nil, err
	}
	user := strings.SplitN(image.User, ":", 2)[0]
	if user != "" && user != "root" && user != "0" {
		return nil, fmt.Errorf("guarded SSH challenge images must start as root and may drop application privileges in their entrypoint")
	}
	command := append(slices.Clone(image.Entrypoint), image.Cmd...)
	if len(command) == 0 {
		return nil, fmt.Errorf("challenge image has no original command")
	}
	security := e.serviceSecurity
	security.capAdd = append(slices.Clone(security.capAdd), "NET_RAW")
	args := buildDockerRunArgs(network, e.stateMountPath, security, task)
	args = args[:len(args)-1]
	args = append(args, "--no-healthcheck", "--entrypoint", captureGate, "--label", "adplatform.capture=guarded", "--label", "adplatform.capture_owner="+e.captureVolume, "--mount", "type=volume,src="+e.captureVolume+",dst="+captureMount+",readonly,volume-nocopy", task.BaselineImage)
	return append(append(args, "run"), command...), nil
}
func (e *dockerCLIExecutor) capturePaths(info captureInspect, task apigateway.ControllerRuntimeTask) (string, string, error) {
	if info.State.Pid <= 0 || !info.State.Running || len(info.Config.Entrypoint) != 1 || info.Config.Entrypoint[0] != captureGate {
		return "", "", fmt.Errorf("instance is not running the immutable capture startup gate; redeploy it to enable capture")
	}
	if info.Config.Labels["adplatform.capture"] != "guarded" || info.Config.Labels["adplatform.capture_owner"] != e.captureVolume || info.Config.Labels["adplatform.team_id"] != strconv.Itoa(task.TeamID) || info.Config.Labels["adplatform.challenge_id"] != strconv.Itoa(task.ChallengeID) {
		return "", "", fmt.Errorf("capture instance identity mismatch")
	}
	if info.HostConfig.Privileged || !info.HostConfig.Init || (info.HostConfig.PidMode != "" && info.HostConfig.PidMode != "private") || info.HostConfig.NetworkMode == "host" || len(info.NetworkSettings.Networks) != 1 {
		return "", "", fmt.Errorf("guarded capture requires Docker init, private PID/network namespaces, and one game interface")
	}
	if len(info.HostConfig.CapDrop) != 1 || !strings.EqualFold(info.HostConfig.CapDrop[0], "ALL") {
		return "", "", fmt.Errorf("unsafe capture capability bounds")
	}
	allowed := []string{"CHOWN", "DAC_OVERRIDE", "FOWNER", "SETGID", "SETUID", "NET_BIND_SERVICE", "SYS_CHROOT", "AUDIT_WRITE", "NET_RAW"}
	for _, cap := range info.HostConfig.CapAdd {
		if !slices.Contains(allowed, normalizedCapability(cap)) {
			return "", "", fmt.Errorf("unsafe instance capability %s", cap)
		}
	}
	if len(info.HostConfig.SecurityOpt) != 1 || (info.HostConfig.SecurityOpt[0] != "no-new-privileges:true" && info.HostConfig.SecurityOpt[0] != "no-new-privileges") {
		return "", "", fmt.Errorf("missing no-new-privileges")
	}
	if len(info.Config.Cmd) < 2 || info.Config.Cmd[0] != "run" || !slices.Equal(info.Config.Healthcheck.Test, []string{"NONE"}) {
		return "", "", fmt.Errorf("guarded capture requires the startup gate and disabled image healthchecks")
	}
	mounted := false
	for _, m := range info.Mounts {
		if m.Name == e.captureVolume && (m.RW || m.Destination != captureMount) {
			return "", "", fmt.Errorf("capture bootstrap volume cannot have writable or alternate mounts")
		}
		if m.Type != "volume" {
			return "", "", fmt.Errorf("host mounts are forbidden in guarded challenge containers")
		}
		if m.Destination == captureMount && m.Type == "volume" && m.Name == e.captureVolume && !m.RW {
			mounted = true
		}
	}
	if !mounted {
		return "", "", fmt.Errorf("capture startup executable must be on the trusted read-only volume")
	}
	for _, network := range info.NetworkSettings.Networks {
		if network.IPAddress != task.SSHHost || net.ParseIP(network.IPAddress).To4() == nil {
			return "", "", fmt.Errorf("capture IP does not match the service instance")
		}
	}
	data, err := os.ReadFile(filepath.Join(e.captureProcRoot, strconv.Itoa(info.State.Pid), "cgroup"))
	if err != nil {
		return "", "", err
	}
	var group string
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "0::/") {
			group = strings.TrimPrefix(line, "0::")
			break
		}
	}
	if group == "" || group == "/" || strings.Contains(group, "..") {
		return "", "", fmt.Errorf("dedicated cgroup v2 container scope required")
	}
	key := info.NetworkSettings.SandboxKey
	if filepath.Dir(key) != e.captureNetnsHostRoot || filepath.Base(key) == "default" {
		return "", "", fmt.Errorf("unexpected Docker network namespace path")
	}
	return filepath.Join("/sys/fs/cgroup", group), filepath.Join(e.captureNetnsRoot, filepath.Base(key)), nil
}

// The immutable binary checks Docker init's entrypoint child. A forged socket
// response cannot make its status command exit successfully.
func (e *dockerCLIExecutor) captureGateProcess(ctx context.Context, name string) error {
	_, err := e.execDocker(ctx, "exec", name, captureGate, "status")
	return err
}
func (e *dockerCLIExecutor) reconcileCapturedService(ctx context.Context, task apigateway.ControllerRuntimeTask) error {
	info, err := e.inspectCapture(ctx, task.ContainerName)
	if err != nil {
		return err
	}
	if !info.State.Running {
		if _, err = e.execDocker(ctx, "start", task.ContainerName); err != nil {
			return err
		}
		info, err = e.inspectCapture(ctx, task.ContainerName)
		if err != nil {
			return err
		}
	}
	group, namespace, err := e.capturePaths(info, task)
	if err != nil {
		return err
	}
	guardReady := e.capturePolicy(ctx, "check", group, e.captureObject) == nil && e.capturePolicy(ctx, "rx-check", namespace, "eth0", task.SSHHost, e.captureObject) == nil
	if !guardReady {
		// Even a genuine gate could have surviving application children.
		// A Docker restart kills the old process tree before any policy changes.
		if _, err = e.execDocker(ctx, "restart", task.ContainerName); err != nil {
			return err
		}
		info, err = e.inspectCapture(ctx, task.ContainerName)
		if err != nil {
			return err
		}
		group, namespace, err = e.capturePaths(info, task)
		if err != nil {
			return err
		}
	}
	gateErr := e.captureGateProcess(ctx, task.ContainerName)
	if gateErr != nil {
		if !guardReady {
			return fmt.Errorf("capture policy repair requires a fresh immutable startup gate: %w", gateErr)
		}
		e.rememberCapture(info)
		return nil
	}
	// The gate's nonce exists only in process memory. No participant-writable
	// readiness file can release a new boot, including automatic Docker restarts.
	var nonce string
	deadline := time.Now().Add(e.timeout)
	for {
		output, probeErr := e.execDocker(ctx, "exec", task.ContainerName, captureGate, "probe")
		if probeErr == nil {
			nonce = strings.TrimSpace(string(output))
			break
		}
		if time.Now().After(deadline) || ctx.Err() != nil {
			return fmt.Errorf("capture gate not ready or guard missing; application must remain stopped")
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(50 * time.Millisecond):
		}
	}
	if len(nonce) != 64 {
		return fmt.Errorf("invalid startup gate nonce")
	}
	if err = e.captureGateProcess(ctx, task.ContainerName); err != nil {
		return fmt.Errorf("capture startup gate changed before policy attachment: %w", err)
	}
	if e.capturePolicy(ctx, "check", group, e.captureObject) != nil {
		// The previous application process tree was killed by Docker restart.
		if err = e.capturePolicy(ctx, "detach", group); err != nil {
			return err
		}
		if err = e.capturePolicy(ctx, "attach", group, e.captureObject); err != nil {
			return err
		}
	}
	if e.capturePolicy(ctx, "rx-check", namespace, "eth0", task.SSHHost, e.captureObject) != nil {
		if err = e.capturePolicy(ctx, "rx-detach", namespace, "eth0"); err != nil {
			return err
		}
		if err = e.capturePolicy(ctx, "rx-attach", namespace, "eth0", task.SSHHost, e.captureObject); err != nil {
			return err
		}
	}
	if err = e.capturePolicy(ctx, "check", group, e.captureObject); err != nil {
		return err
	}
	if err = e.capturePolicy(ctx, "rx-check", namespace, "eth0", task.SSHHost, e.captureObject); err != nil {
		return err
	}
	after, err := e.inspectCapture(ctx, task.ContainerName)
	if err != nil {
		return err
	}
	if after.ID != info.ID || after.State.StartedAt != info.State.StartedAt || after.State.Pid != info.State.Pid {
		return fmt.Errorf("container restarted during capture policy attachment")
	}
	if _, err = e.execDocker(ctx, "exec", task.ContainerName, captureGate, "release", nonce); err != nil {
		return err
	}
	e.rememberCapture(after)
	return nil
}
func (e *dockerCLIExecutor) captureInstanceLock(name string) *sync.Mutex {
	value, _ := e.captureLocks.LoadOrStore(name, &sync.Mutex{})
	return value.(*sync.Mutex)
}

// lockCaptureInstance serializes operations on one instance. A team controls
// the shell inside its container, so a hung command must not stall other teams.
func (e *dockerCLIExecutor) lockCaptureInstance(name string) func() {
	lock := e.captureInstanceLock(name)
	lock.Lock()
	return lock.Unlock
}

func (e *dockerCLIExecutor) rememberCapture(info captureInspect) {
	e.captureMu.Lock()
	defer e.captureMu.Unlock()
	if e.captureSeen == nil {
		e.captureSeen = map[string]captureStamp{}
	}
	e.captureSeen[info.ID] = captureStamp{info.State.StartedAt, time.Now()}
}
func (e *dockerCLIExecutor) sweepCapture(ctx context.Context) error {
	output, err := e.execDocker(ctx, "ps", "--filter", "label=adplatform.capture=guarded", "--filter", "label=adplatform.capture_owner="+e.captureVolume, "--format", "{{.Names}}")
	if err != nil {
		return err
	}
	for _, name := range strings.Fields(string(output)) {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		// A busy instance is already being reconciled by its current operation.
		lock := e.captureInstanceLock(name)
		if !lock.TryLock() {
			continue
		}
		info, err := e.inspectCapture(ctx, name)
		if err == nil {
			e.captureMu.Lock()
			stamp := e.captureSeen[info.ID]
			e.captureMu.Unlock()
			if stamp.Started != info.State.StartedAt || time.Since(stamp.Verified) > 30*time.Second {
				team, _ := strconv.Atoi(info.Config.Labels["adplatform.team_id"])
				challenge, _ := strconv.Atoi(info.Config.Labels["adplatform.challenge_id"])
				task := apigateway.ControllerRuntimeTask{TeamID: team, ChallengeID: challenge, ContainerName: name, SSHHost: info.Config.Labels["adplatform.service_ip"]}
				err = e.reconcileCapturedService(ctx, task)
			}
		}
		if err != nil {
			e.stopCapturedService(name)
		}
		lock.Unlock()
		if err != nil {
			return fmt.Errorf("capture guard recovery for %s: %w", name, err)
		}
	}
	return nil
}
func (e *dockerCLIExecutor) startCaptureMonitor() func() {
	if !e.captureEnabled() {
		return func() {}
	}
	ctx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		interval := e.captureSweepInterval
		if interval <= 0 {
			interval = 2 * time.Second
		}
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
			cycle, stop := context.WithTimeout(ctx, e.timeout*3)
			err := e.sweepCapture(cycle)
			stop()
			if err != nil && ctx.Err() == nil {
				log.Printf("guarded capture recovery: %v", err)
			}
		}
	}()
	return func() { cancel(); wg.Wait() }
}
