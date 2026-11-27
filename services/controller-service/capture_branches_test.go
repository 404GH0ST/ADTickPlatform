package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"adplatform/internal/services/apigateway"
)

const (
	gateStatus  = "exec svc-101-1 /opt/adplatform-capture/start status"
	gateProbe   = "exec svc-101-1 /opt/adplatform-capture/start probe"
	gateRelease = "exec svc-101-1 /opt/adplatform-capture/start release *"
	inspectSvc  = "inspect svc-101-1"
)

var testNonce = strings.Repeat("ab", 32)

func guardedInspect() captureInspect {
	var info captureInspect
	info.ID = "cid-1"
	info.Name = "/svc-101-1"
	info.State.Pid = 4242
	info.State.Running = true
	info.State.StartedAt = "2026-11-19T00:00:00Z"
	info.Config.Entrypoint = []string{captureGate}
	info.Config.Cmd = []string{"run", "/app"}
	info.Config.Healthcheck.Test = []string{"NONE"}
	info.Config.Labels = map[string]string{"adplatform.capture": "guarded", "adplatform.capture_owner": "capvol", "adplatform.team_id": "101", "adplatform.challenge_id": "1", "adplatform.service_ip": "10.80.1.101"}
	info.HostConfig.CapDrop = []string{"ALL"}
	info.HostConfig.CapAdd = []string{"NET_RAW", "CAP_CHOWN"}
	info.HostConfig.SecurityOpt = []string{"no-new-privileges:true"}
	info.HostConfig.Init = true
	info.Mounts = append(info.Mounts, struct {
		Type, Name, Destination string
		RW                      bool
	}{Type: "volume", Name: "capvol", Destination: captureMount})
	info.NetworkSettings.SandboxKey = "/var/run/docker/netns/abc"
	info.NetworkSettings.Networks = map[string]struct{ IPAddress string }{"game": {IPAddress: "10.80.1.101"}}
	return info
}

func inspectJSON(t *testing.T, infos ...captureInspect) string {
	t.Helper()
	data, err := json.Marshal(infos)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func captureProc(t *testing.T, cgroup string) string {
	t.Helper()
	root := t.TempDir()
	if cgroup != "" {
		mustWrite(t, filepath.Join(root, "4242", "cgroup"), cgroup)
	}
	return root
}

func mustWrite(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

func guardedExecutor(t *testing.T, docker, tool string) *dockerCLIExecutor {
	t.Helper()
	e := dockerExecutor(docker, 2*time.Second)
	e.captureMode = "guarded"
	e.captureVolume = "capvol"
	e.captureTool = tool
	e.captureObject = "capture.bpf.o"
	e.captureProcRoot = captureProc(t, "1:name=x:/\n0::/system.slice/docker-cid.scope\n")
	e.captureNetnsRoot = "/host/netns"
	e.captureNetnsHostRoot = "/var/run/docker/netns"
	e.serviceSecurity = dockerRunSecurity{capDrop: []string{"ALL"}, capAdd: []string{"CHOWN"}, securityOpt: []string{"no-new-privileges:true"}}
	return e
}

func TestValidateCaptureConfigBranches(t *testing.T) {
	cases := map[string]func(*dockerCLIExecutor){
		"unknown mode":         func(e *dockerCLIExecutor) { e.captureMode = "sometimes" },
		"raw without guard":    func(e *dockerCLIExecutor) { e.captureMode = ""; e.serviceSecurity.capAdd = []string{"NET_RAW"} },
		"all without guard":    func(e *dockerCLIExecutor) { e.captureMode = "disabled"; e.serviceSecurity.capAdd = []string{"ALL"} },
		"raw not dropped":      func(e *dockerCLIExecutor) { e.captureMode = ""; e.serviceSecurity.capDrop = []string{"MKNOD"} },
		"guarded partial drop": func(e *dockerCLIExecutor) { e.serviceSecurity.capDrop = []string{"NET_RAW"} },
		"guarded bad cap":      func(e *dockerCLIExecutor) { e.serviceSecurity.capAdd = []string{"SYS_ADMIN"} },
		"guarded no nnp":       func(e *dockerCLIExecutor) { e.serviceSecurity.securityOpt = []string{"seccomp=unconfined"} },
		"guarded no tool":      func(e *dockerCLIExecutor) { e.captureTool = "" },
	}
	for name, mutate := range cases {
		e := guardedExecutor(t, "docker", "tool")
		mutate(e)
		if err := e.validateCaptureConfig(); err == nil {
			t.Errorf("%s: unsafe capture configuration accepted", name)
		}
	}
	e := guardedExecutor(t, "docker", "tool")
	e.captureMode = "disabled"
	e.serviceSecurity.capDrop = []string{"NET_RAW"}
	if err := e.validateCaptureConfig(); err != nil {
		t.Fatalf("disabled capture dropping NET_RAW is valid: %v", err)
	}
	if err := guardedExecutor(t, "docker", "tool").validateCaptureConfig(); err != nil {
		t.Fatalf("a correct guarded configuration is valid: %v", err)
	}
}

// Every property capturePaths checks guards NET_RAW capture granted to a
// participant-controlled container, so each must be enforced.
func TestCapturePathsRejectsEveryUnsafeProperty(t *testing.T) {
	task := runtimeTask()
	cases := map[string]func(*captureInspect){
		"not running":        func(i *captureInspect) { i.State.Running = false },
		"no pid":             func(i *captureInspect) { i.State.Pid = 0 },
		"other entrypoint":   func(i *captureInspect) { i.Config.Entrypoint = []string{"/bin/sh"} },
		"wrong capture mode": func(i *captureInspect) { i.Config.Labels["adplatform.capture"] = "off" },
		"foreign owner":      func(i *captureInspect) { i.Config.Labels["adplatform.capture_owner"] = "other" },
		"other team":         func(i *captureInspect) { i.Config.Labels["adplatform.team_id"] = "102" },
		"other challenge":    func(i *captureInspect) { i.Config.Labels["adplatform.challenge_id"] = "2" },
		"privileged":         func(i *captureInspect) { i.HostConfig.Privileged = true },
		"no init":            func(i *captureInspect) { i.HostConfig.Init = false },
		"host pid":           func(i *captureInspect) { i.HostConfig.PidMode = "host" },
		"host network":       func(i *captureInspect) { i.HostConfig.NetworkMode = "host" },
		"two networks": func(i *captureInspect) {
			i.NetworkSettings.Networks["second"] = struct{ IPAddress string }{"10.0.0.2"}
		},
		"no drop all": func(i *captureInspect) { i.HostConfig.CapDrop = nil },
		"sys admin":   func(i *captureInspect) { i.HostConfig.CapAdd = append(i.HostConfig.CapAdd, "SYS_ADMIN") },
		"no nnp":      func(i *captureInspect) { i.HostConfig.SecurityOpt = []string{"seccomp=unconfined"} },
		"extra secopt": func(i *captureInspect) {
			i.HostConfig.SecurityOpt = append(i.HostConfig.SecurityOpt, "apparmor=unconfined")
		},
		"no gate command":   func(i *captureInspect) { i.Config.Cmd = []string{"/app"} },
		"healthcheck":       func(i *captureInspect) { i.Config.Healthcheck.Test = []string{"CMD", "true"} },
		"writable gate vol": func(i *captureInspect) { i.Mounts[0].RW = true },
		"gate vol elsewhere": func(i *captureInspect) {
			i.Mounts[0].Destination = "/elsewhere"
		},
		"bind mount": func(i *captureInspect) {
			i.Mounts = append(i.Mounts, struct {
				Type, Name, Destination string
				RW                      bool
			}{Type: "bind", Destination: "/host"})
		},
		"gate vol missing": func(i *captureInspect) { i.Mounts[0].Name = "other" },
		"wrong ip": func(i *captureInspect) {
			i.NetworkSettings.Networks["game"] = struct{ IPAddress string }{"10.80.1.102"}
		},
		"ipv6":          func(i *captureInspect) { i.NetworkSettings.Networks["game"] = struct{ IPAddress string }{"fd00::1"} },
		"foreign netns": func(i *captureInspect) { i.NetworkSettings.SandboxKey = "/tmp/abc" },
		"default netns": func(i *captureInspect) { i.NetworkSettings.SandboxKey = "/var/run/docker/netns/default" },
	}
	for name, mutate := range cases {
		e := guardedExecutor(t, "docker", "tool")
		info := guardedInspect()
		mutate(&info)
		if name == "ipv6" {
			task.SSHHost = "fd00::1"
		} else {
			task.SSHHost = "10.80.1.101"
		}
		if _, _, err := e.capturePaths(info, task); err == nil {
			t.Errorf("%s: unsafe container accepted for capture", name)
		}
	}
	task.SSHHost = "10.80.1.101"
	for name, cgroup := range map[string]string{"missing file": "", "no v2 line": "1:cpu:/x\n", "root group": "0::/\n", "traversal": "0::/a/../../b\n"} {
		e := guardedExecutor(t, "docker", "tool")
		e.captureProcRoot = captureProc(t, cgroup)
		if _, _, err := e.capturePaths(guardedInspect(), task); err == nil {
			t.Errorf("cgroup %s: accepted", name)
		}
	}
	group, namespace, err := guardedExecutor(t, "docker", "tool").capturePaths(guardedInspect(), task)
	if err != nil || group != "/sys/fs/cgroup/system.slice/docker-cid.scope" || namespace != "/host/netns/abc" {
		t.Fatalf("a safe container must yield its cgroup and netns: %q %q %v", group, namespace, err)
	}
}

func TestCapturedRunArgs(t *testing.T) {
	ctx := context.Background()
	task := runtimeTask()
	labels := `{"adplatform.capture-bootstrap":"true"}`
	image := `{"Entrypoint":["/entry"],"Cmd":["serve"],"User":"root:root"}`
	docker, _ := fakeCLI(t, on("volume inspect --format * capvol", reply(labels, 0)), on("image inspect --format *", reply(image, 0)))
	args, err := guardedExecutor(t, docker, "tool").capturedRunArgs(ctx, "net", task)
	joined := strings.Join(args, " ")
	if err != nil || !strings.Contains(joined, "--entrypoint "+captureGate) || !strings.Contains(joined, "src=capvol,dst="+captureMount+",readonly") || !strings.HasSuffix(joined, "img/svc:1 run /entry serve") || !strings.Contains(joined, "--cap-add NET_RAW") {
		t.Fatalf("guarded run args: %v %v", joined, err)
	}

	alias := task
	alias.StateVolume = "capvol"
	cases := map[string]struct {
		docker string
		task   apigateway.ControllerRuntimeTask
	}{
		"state aliases bootstrap": {docker, alias},
		"volume missing":          {must(fakeCLI(t, on("volume inspect *", reply("no such volume", 1)))), task},
		"untrusted volume":        {must(fakeCLI(t, on("volume inspect *", reply(`{}`, 0)))), task},
		"image missing":           {must(fakeCLI(t, on("volume inspect *", reply(labels, 0)), on("image inspect *", reply("", 1)))), task},
		"image bad json":          {must(fakeCLI(t, on("volume inspect *", reply(labels, 0)), on("image inspect *", reply("{", 0)))), task},
		"non-root user":           {must(fakeCLI(t, on("volume inspect *", reply(labels, 0)), on("image inspect *", reply(`{"Cmd":["x"],"User":"app"}`, 0)))), task},
		"no command":              {must(fakeCLI(t, on("volume inspect *", reply(labels, 0)), on("image inspect *", reply(`{"User":"0"}`, 0)))), task},
	}
	for name, tc := range cases {
		if _, err := guardedExecutor(t, tc.docker, "tool").capturedRunArgs(ctx, "net", tc.task); err == nil {
			t.Errorf("%s: unsafe guarded run accepted", name)
		}
	}
	guardFail := guardedExecutor(t, cases["volume missing"].docker, "tool")
	if err := guardFail.runFreshContainer(ctx, task); err == nil {
		t.Fatal("guarded runs must stop when the bootstrap volume is unusable")
	}
}

func must(bin string, _ func() []string) string { return bin }

// captureWorld wires a fake docker and policy tool for reconcileCapturedService.
type captureWorld struct {
	dockerRules []cliRule
	toolRules   []cliRule
}

func (w captureWorld) executor(t *testing.T) (*dockerCLIExecutor, func() []string, func() []string) {
	t.Helper()
	docker, dockerCalls := fakeCLI(t, w.dockerRules...)
	tool, toolCalls := fakeCLI(t, w.toolRules...)
	return guardedExecutor(t, docker, tool), dockerCalls, toolCalls
}

func TestReconcileCapturedServiceHappyPaths(t *testing.T) {
	ctx := context.Background()
	task := runtimeTask()
	good := inspectJSON(t, guardedInspect())

	// Guard already in place and the gate already released: nothing to do.
	e, dockerCalls, _ := captureWorld{dockerRules: []cliRule{on(inspectSvc, reply(good, 0)), on(gateStatus, reply("not leader", 1))}}.executor(t)
	if err := e.reconcileCapturedService(ctx, task); err != nil || callsMatching(dockerCalls(), "restart") != 0 || len(e.captureSeen) != 1 {
		t.Fatalf("a guarded running instance needs no action: %v %v", err, dockerCalls())
	}

	// Stopped, then the guard is missing: start, restart, attach, release.
	stopped := guardedInspect()
	stopped.State.Running = false
	e, dockerCalls, toolCalls := captureWorld{
		dockerRules: []cliRule{
			seq(inspectSvc, reply(inspectJSON(t, stopped), 0), reply(good, 0)),
			seq(gateProbe, reply("starting", 1), reply(testNonce+"\n", 0)),
		},
		toolRules: []cliRule{
			seq("check *", reply("missing", 1), reply("missing", 1), reply("", 0)),
			seq("rx-check *", reply("missing", 1), reply("", 0)),
		},
	}.executor(t)
	if err := e.reconcileCapturedService(ctx, task); err != nil {
		t.Fatalf("guard repair: %v\ndocker: %v\ntool: %v", err, dockerCalls(), toolCalls())
	}
	for _, want := range []string{"start svc-101-1", "restart svc-101-1", "exec svc-101-1 /opt/adplatform-capture/start release " + testNonce} {
		if callsMatching(dockerCalls(), want) != 1 {
			t.Fatalf("missing %q in %v", want, dockerCalls())
		}
	}
	for _, want := range []string{"detach", "attach", "rx-detach", "rx-attach"} {
		if callsMatching(toolCalls(), want+" ") != 1 {
			t.Fatalf("missing policy %q in %v", want, toolCalls())
		}
	}
}

func TestReconcileCapturedServiceFailures(t *testing.T) {
	ctx := context.Background()
	task := runtimeTask()
	good := inspectJSON(t, guardedInspect())
	stopped := guardedInspect()
	stopped.State.Running = false
	restarted := guardedInspect()
	restarted.State.StartedAt = "later"
	missing := []cliRule{seq("check *", reply("", 1), reply("", 0)), seq("rx-check *", reply("", 1), reply("", 0))}
	probeOK := on(gateProbe, reply(testNonce, 0))

	cases := map[string]captureWorld{
		"inspect fails":        {dockerRules: []cliRule{on(inspectSvc, reply("gone", 1))}},
		"inspect not one":      {dockerRules: []cliRule{on(inspectSvc, reply("[]", 0))}},
		"start fails":          {dockerRules: []cliRule{on(inspectSvc, reply(inspectJSON(t, stopped), 0)), on("start *", reply("", 1))}},
		"reinspect fails":      {dockerRules: []cliRule{seq(inspectSvc, reply(inspectJSON(t, stopped), 0), reply("", 1))}},
		"unsafe instance":      {dockerRules: []cliRule{on(inspectSvc, reply(inspectJSON(t, stopped), 0))}},
		"restart fails":        {dockerRules: []cliRule{on(inspectSvc, reply(good, 0)), on("restart *", reply("", 1))}, toolRules: missing},
		"post-restart inspect": {dockerRules: []cliRule{seq(inspectSvc, reply(good, 0), reply("", 1))}, toolRules: missing},
		"post-restart unsafe":  {dockerRules: []cliRule{seq(inspectSvc, reply(good, 0), reply(inspectJSON(t, stopped), 0))}, toolRules: missing},
		"stale gate":           {dockerRules: []cliRule{on(inspectSvc, reply(good, 0)), on(gateStatus, reply("", 1))}, toolRules: missing},
		"bad nonce":            {dockerRules: []cliRule{on(inspectSvc, reply(good, 0)), on(gateProbe, reply("short", 0))}, toolRules: missing},
		"gate changed":         {dockerRules: []cliRule{on(inspectSvc, reply(good, 0)), probeOK, seq(gateStatus, reply("", 0), reply("", 1))}, toolRules: missing},
		"detach fails":         {dockerRules: []cliRule{on(inspectSvc, reply(good, 0)), probeOK}, toolRules: []cliRule{on("check *", reply("", 1)), on("detach *", reply("", 1))}},
		"attach fails":         {dockerRules: []cliRule{on(inspectSvc, reply(good, 0)), probeOK}, toolRules: []cliRule{on("check *", reply("", 1)), on("attach *", reply("", 1))}},
		"rx-detach fails":      {dockerRules: []cliRule{on(inspectSvc, reply(good, 0)), probeOK}, toolRules: []cliRule{on("rx-check *", reply("", 1)), on("rx-detach *", reply("", 1))}},
		"rx-attach fails":      {dockerRules: []cliRule{on(inspectSvc, reply(good, 0)), probeOK}, toolRules: []cliRule{on("rx-check *", reply("", 1)), on("rx-attach *", reply("", 1))}},
		"final check fails":    {dockerRules: []cliRule{on(inspectSvc, reply(good, 0)), probeOK}, toolRules: []cliRule{on("check *", reply("", 1))}},
		"final rx-check fails": {dockerRules: []cliRule{on(inspectSvc, reply(good, 0)), probeOK}, toolRules: []cliRule{on("rx-check *", reply("", 1))}},
		"final inspect fails":  {dockerRules: []cliRule{seq(inspectSvc, reply(good, 0), reply(good, 0), reply("", 1)), probeOK}, toolRules: missing},
		"restarted mid-attach": {dockerRules: []cliRule{seq(inspectSvc, reply(good, 0), reply(good, 0), reply(inspectJSON(t, restarted), 0)), probeOK}, toolRules: missing},
		"release fails":        {dockerRules: []cliRule{on(inspectSvc, reply(good, 0)), probeOK, on(gateRelease, reply("", 1))}, toolRules: missing},
	}
	for name, world := range cases {
		e, _, _ := world.executor(t)
		if err := e.reconcileCapturedService(ctx, task); err == nil {
			t.Errorf("%s: an unverified capture guard was accepted", name)
		}
	}

	// A gate that never answers the probe keeps the application stopped.
	e, _, _ := captureWorld{dockerRules: []cliRule{on(inspectSvc, reply(good, 0)), on(gateProbe, reply("", 1))}, toolRules: missing}.executor(t)
	e.timeout = 150 * time.Millisecond
	if err := e.reconcileCapturedService(ctx, task); err == nil || !strings.Contains(err.Error(), "must remain stopped") {
		t.Fatalf("an unresponsive gate: %v", err)
	}
	e, _, _ = captureWorld{dockerRules: []cliRule{on(inspectSvc, reply(good, 0)), on(gateProbe, reply("", 1))}, toolRules: missing}.executor(t)
	cancelled, cancel := context.WithCancel(ctx)
	time.AfterFunc(120*time.Millisecond, cancel)
	if err := e.reconcileCapturedService(cancelled, task); err == nil {
		t.Fatal("a cancelled reconcile must stop waiting for the gate")
	}
}

func TestCaptureModeRuntimeOperations(t *testing.T) {
	ctx := context.Background()
	task := runtimeTask()
	good := inspectJSON(t, guardedInspect())
	legacy := guardedInspect()
	legacy.Config.Entrypoint = []string{"/entry"}
	labels := `{"adplatform.capture-bootstrap":"true"}`
	image := `{"Cmd":["serve"],"User":""}`
	exists := on(psName, reply("svc-101-1", 0))
	released := on(gateStatus, reply("", 1)) // gate already released, guard intact

	e, _, _ := captureWorld{dockerRules: []cliRule{exists, on(inspectSvc, reply(good, 0)), released}}.executor(t)
	if err := e.EnsureService(ctx, task); err != nil {
		t.Fatalf("ensure on a guarded instance reconciles in place: %v", err)
	}
	e, dockerCalls, _ := captureWorld{dockerRules: []cliRule{exists, seq(inspectSvc, reply(inspectJSON(t, legacy), 0), reply(good, 0)), on("volume inspect *", reply(labels, 0)), on("image inspect --format *", reply(image, 0)), released}}.executor(t)
	if err := e.EnsureService(ctx, task); err != nil || callsMatching(dockerCalls(), "rm -f svc-101-1") != 1 {
		t.Fatalf("a legacy instance is replaced by a guarded one: %v %v", err, dockerCalls())
	}
	e, _, _ = captureWorld{dockerRules: []cliRule{exists, on(inspectSvc, reply("", 1))}}.executor(t)
	if e.EnsureService(ctx, task) == nil || e.RestartService(ctx, task) == nil {
		t.Fatal("an uninspectable guarded instance must fail")
	}
	e, dockerCalls, _ = captureWorld{dockerRules: []cliRule{exists, on(inspectSvc, reply(inspectJSON(t, guardedStopped(t)), 0)), on("start *", reply("", 1))}}.executor(t)
	if err := e.EnsureService(ctx, task); err == nil || callsMatching(dockerCalls(), "stop --time 1 svc-101-1") != 1 {
		t.Fatalf("a failed guard reconcile must stop the instance: %v %v", err, dockerCalls())
	}

	e, _, _ = captureWorld{dockerRules: []cliRule{exists, on(inspectSvc, reply(inspectJSON(t, legacy), 0))}}.executor(t)
	if err := e.RestartService(ctx, task); err == nil || !strings.Contains(err.Error(), "legacy instance") {
		t.Fatalf("legacy restarts must require redeployment: %v", err)
	}
	e, _, _ = captureWorld{dockerRules: []cliRule{exists, on(inspectSvc, reply(good, 0)), released}}.executor(t)
	if err := e.RestartService(ctx, task); err != nil {
		t.Fatalf("guarded restart: %v", err)
	}
	e, _, _ = captureWorld{dockerRules: []cliRule{exists, seq(inspectSvc, reply(good, 0), reply("", 1))}}.executor(t)
	if err := e.RestartService(ctx, task); err == nil {
		t.Fatal("a failed post-restart reconcile must fail")
	}

	e, _, _ = captureWorld{dockerRules: []cliRule{exists, on(inspectSvc, reply(good, 0)), released}}.executor(t)
	if err := e.ApplySSHCredential(ctx, task, apigateway.ControllerSSHCredential{Password: "p"}); err != nil {
		t.Fatalf("guarded credential apply: %v", err)
	}
	e, dockerCalls, _ = captureWorld{dockerRules: []cliRule{exists, on(inspectSvc, reply("", 1))}}.executor(t)
	if err := e.ApplySSHCredential(ctx, task, apigateway.ControllerSSHCredential{Password: "p"}); err == nil || callsMatching(dockerCalls(), "stop --time 1") != 1 {
		t.Fatalf("a failed guard check must stop the instance: %v", err)
	}

	e, _, _ = captureWorld{dockerRules: []cliRule{on("volume inspect *", reply(labels, 0)), on("image inspect --format *", reply(image, 0)), on(inspectSvc, reply(good, 0)), released}}.executor(t)
	if err := e.FactoryResetService(ctx, task); err != nil {
		t.Fatalf("guarded factory reset: %v", err)
	}
	e, _, _ = captureWorld{dockerRules: []cliRule{on("stop *", reply("", 1))}}.executor(t)
	e.stopCapturedService("svc-101-1") // logs only
}

func guardedStopped(t *testing.T) captureInspect {
	t.Helper()
	info := guardedInspect()
	info.State.Running = false
	return info
}

func TestSweepCaptureAndMonitor(t *testing.T) {
	ctx := context.Background()
	good := inspectJSON(t, guardedInspect())
	psGuarded := "ps --filter label=adplatform.capture=guarded *"

	e, _, _ := captureWorld{dockerRules: []cliRule{on(psGuarded, reply("", 1))}}.executor(t)
	if err := e.sweepCapture(ctx); err == nil {
		t.Fatal("a failed listing must fail the sweep")
	}
	e, dockerCalls, _ := captureWorld{dockerRules: []cliRule{on(psGuarded, reply("svc-101-1\n", 0)), on(inspectSvc, reply(good, 0)), on(gateStatus, reply("", 1))}}.executor(t)
	if err := e.sweepCapture(ctx); err != nil {
		t.Fatalf("a healthy instance passes the sweep: %v", err)
	}
	if err := e.sweepCapture(ctx); err != nil || callsMatching(dockerCalls(), gateStatus) != 1 {
		t.Fatalf("a freshly verified instance is not re-verified: %v %v", err, dockerCalls())
	}
	e, dockerCalls, _ = captureWorld{dockerRules: []cliRule{on(psGuarded, reply("svc-101-1\n", 0)), on(inspectSvc, reply("", 1))}}.executor(t)
	if err := e.sweepCapture(ctx); err == nil || callsMatching(dockerCalls(), "stop --time 1 svc-101-1") != 1 {
		t.Fatalf("an unverifiable instance is stopped: %v %v", err, dockerCalls())
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	e, _, _ = captureWorld{dockerRules: []cliRule{on(psGuarded, reply("svc-101-1\n", 0))}}.executor(t)
	if err := e.sweepCapture(cancelled); err == nil {
		t.Fatal("a cancelled sweep must stop")
	}

	disabled := dockerExecutor("docker", time.Second)
	disabled.startCaptureMonitor()() // no-op when capture is disabled

	e, dockerCalls, _ = captureWorld{dockerRules: []cliRule{on(psGuarded, reply("", 1))}}.executor(t)
	e.captureSweepInterval = 20 * time.Millisecond
	stop := e.startCaptureMonitor()
	time.Sleep(120 * time.Millisecond)
	stop()
	if callsMatching(dockerCalls(), "ps --filter") == 0 {
		t.Fatal("the monitor must sweep periodically")
	}
	defaulted := guardedExecutor(t, "docker", "tool")
	defaulted.startCaptureMonitor()() // default interval, stopped immediately
}

func TestReconcileStopsWaitingWhenCancelledDuringProbeBackoff(t *testing.T) {
	good := inspectJSON(t, guardedInspect())
	missing := []cliRule{seq("check *", reply("", 1), reply("", 0)), seq("rx-check *", reply("", 1), reply("", 0))}
	e, dockerCalls, _ := captureWorld{dockerRules: []cliRule{on(inspectSvc, reply(good, 0)), on(gateProbe, reply("", 1))}, toolRules: missing}.executor(t)
	e.timeout = 10 * time.Second
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		for callsMatching(dockerCalls(), gateProbe) == 0 {
			time.Sleep(time.Millisecond)
		}
		// The call is logged when the probe starts; give it time to exit so the
		// cancel lands inside the 50ms backoff that follows.
		time.Sleep(25 * time.Millisecond)
		cancel()
	}()
	if err := e.reconcileCapturedService(ctx, runtimeTask()); err == nil {
		t.Fatal("cancellation during the probe backoff must end the reconcile")
	}
}

// cancelAfterFirstCheck reports cancellation from its second Err call on, but
// never closes Done, so commands already running are not killed.
type cancelAfterFirstCheck struct {
	context.Context
	checks int
}

func (c *cancelAfterFirstCheck) Err() error {
	c.checks++
	if c.checks > 1 {
		return context.Canceled
	}
	return nil
}

func TestSweepCaptureStopsWhenCancelled(t *testing.T) {
	good := inspectJSON(t, guardedInspect())
	e, dockerCalls, _ := captureWorld{dockerRules: []cliRule{on("ps --filter *", reply("svc-101-1\nsvc-102-1\n", 0)), on("inspect *", reply(good, 0))}}.executor(t)
	// Stamps fresh so only cancellation, not staleness, can end the sweep.
	e.captureSeen = map[string]captureStamp{"cid-1": {Started: guardedInspect().State.StartedAt, Verified: time.Now()}}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := e.sweepCapture(ctx); err == nil {
		t.Fatal("a cancelled sweep must stop")
	}
	if callsMatching(dockerCalls(), gateStatus) != 0 || callsMatching(dockerCalls(), "restart ") != 0 || callsMatching(dockerCalls(), "stop ") != 0 {
		t.Fatalf("a cancelled sweep must not reconcile or stop any instance: %v", dockerCalls())
	}
}

// TestSweepCaptureBatchesInspects proves the whole fleet is inspected in a
// single docker call rather than one per container.
func TestSweepCaptureBatchesInspects(t *testing.T) {
	first := guardedInspect()
	second := guardedInspect()
	second.ID = "cid-2"
	second.Name = "/svc-102-1"
	second.Config.Labels = map[string]string{"adplatform.capture": "guarded", "adplatform.capture_owner": "capvol", "adplatform.team_id": "102", "adplatform.challenge_id": "1", "adplatform.service_ip": "10.80.1.102"}
	both := inspectJSON(t, first, second)
	e, dockerCalls, _ := captureWorld{dockerRules: []cliRule{on("ps --filter *", reply("svc-101-1\nsvc-102-1\n", 0)), on("inspect *", reply(both, 0))}}.executor(t)
	// Fresh stamps: the sweep should inspect once and reconcile nothing.
	now := time.Now()
	e.captureSeen = map[string]captureStamp{"cid-1": {Started: first.State.StartedAt, Verified: now}, "cid-2": {Started: second.State.StartedAt, Verified: now}}
	if err := e.sweepCapture(context.Background()); err != nil {
		t.Fatalf("batched sweep failed: %v", err)
	}
	if n := callsMatching(dockerCalls(), "inspect "); n != 1 {
		t.Fatalf("the fleet must be inspected in one docker call, saw %d: %v", n, dockerCalls())
	}
}
