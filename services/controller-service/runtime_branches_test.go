package main

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"adplatform/internal/platform/gamenet"
	"adplatform/internal/services/apigateway"
)

const (
	psName   = "ps -a --filter name=^/svc-101-1$ --format {{.Names}}"
	execSvc  = "exec svc-101-1 *"
	runCmd   = "run -d *"
	volumeLs = "volume ls --filter name=^vol-101-1$ --format {{.Name}}"
)

func runtimeTask() apigateway.ControllerRuntimeTask {
	return apigateway.ControllerRuntimeTask{TeamID: 101, ChallengeID: 1, RuntimeKind: "docker", ContainerName: "svc-101-1", StateVolume: "vol-101-1", BaselineImage: "img/svc:1", SSHHost: "10.80.1.101", Endpoint: "10.80.1.101:10001", ServicePort: 10001}
}

func dockerExecutor(bin string, timeout time.Duration) *dockerCLIExecutor {
	return &dockerCLIExecutor{binary: bin, network: "adplatform_game", networkLayout: "per-service", stateMountPath: "/state", serviceSecurity: testServiceSecurity(), sshContractMode: "verify", sshApplyMode: "docker-exec", timeout: timeout}
}

func TestDockerEnsureServiceBranches(t *testing.T) {
	ctx := context.Background()
	task := runtimeTask()

	bin, _ := fakeCLI(t)
	if err := dockerExecutor(bin, time.Second).EnsureService(ctx, apigateway.ControllerRuntimeTask{RuntimeKind: "podman"}); err == nil {
		t.Fatal("unsupported runtime kinds must be rejected")
	}
	bad := dockerExecutor(bin, time.Second)
	bad.captureMode = "bogus"
	if err := bad.EnsureService(ctx, task); err == nil {
		t.Fatal("invalid capture config must be rejected")
	}

	bin, _ = fakeCLI(t, on(psName, reply("daemon down", 1)))
	if err := dockerExecutor(bin, time.Second).EnsureService(ctx, task); err == nil {
		t.Fatal("a failed existence probe must fail")
	}

	bin, _ = fakeCLI(t, on(psName, reply("svc-101-1", 0)), on("rm -f svc-101-1", reply("busy", 1)))
	if err := dockerExecutor(bin, time.Second).EnsureService(ctx, task); err == nil {
		t.Fatal("a failed removal of the old container must fail")
	}

	bin, calls := fakeCLI(t, on(psName, reply("svc-101-1", 0)), on(runCmd, reply("exec format error", 1)))
	if err := dockerExecutor(bin, time.Second).EnsureService(ctx, task); err == nil || callsMatching(calls(), "rm -f svc-101-1") < 2 {
		t.Fatalf("a failed run must clean up the partial container: %v %v", err, calls())
	}

	bin, calls = fakeCLI(t, seq(runCmd, reply("Conflict. name in use", 1), reply("cid", 0)), seq(execSvc, reply("container svc-101-1 is restarting", 1), reply("", 0)))
	if err := dockerExecutor(bin, 2*time.Second).EnsureService(ctx, task); err != nil {
		t.Fatalf("a name conflict and a restarting container must be retried: %v", err)
	}
	if callsMatching(calls(), "run -d") != 2 || callsMatching(calls(), "exec svc-101-1") != 2 {
		t.Fatalf("unexpected retry pattern %v", calls())
	}

	bin, _ = fakeCLI(t, on(runCmd, reply("Conflict.", 1)))
	if err := dockerExecutor(bin, 2*time.Second).EnsureService(ctx, task); err == nil || !strings.Contains(err.Error(), "onflict") {
		t.Fatalf("persistent conflicts must give up with the run error: %v", err)
	}
	if err := dockerExecutor(bin, 100*time.Millisecond).EnsureService(ctx, task); err == nil || !strings.Contains(err.Error(), "timeout waiting for container name release") {
		t.Fatalf("conflicts past the deadline must time out: %v", err)
	}

	bin, _ = fakeCLI(t, on(execSvc, reply("missing ssh daemon", 1)))
	if err := dockerExecutor(bin, time.Second).EnsureService(ctx, task); err == nil {
		t.Fatal("a failed SSH contract must fail")
	}
	bin, _ = fakeCLI(t, on(execSvc, reply("container is not running", 1)))
	if err := dockerExecutor(bin, 200*time.Millisecond).VerifyForTest(ctx, task); err == nil || !strings.Contains(err.Error(), "did not become ready") {
		t.Fatalf("a container that never starts must time out: %v", err)
	}
	disabled := dockerExecutor(bin, time.Second)
	disabled.sshContractMode = "disabled"
	if err := disabled.verifySSHContract(ctx, task); err != nil {
		t.Fatal("a disabled contract check must pass")
	}
	if isTransientDockerExecState(nil) {
		t.Fatal("nil is not transient")
	}

	noHost := task
	noHost.SSHHost, noHost.Endpoint = "", "nonsense"
	bin, _ = fakeCLI(t)
	if err := dockerExecutor(bin, time.Second).EnsureService(ctx, noHost); err == nil {
		t.Fatal("an unroutable service must be rejected")
	}
	endpointOnly := task
	endpointOnly.SSHHost = ""
	if err := dockerExecutor(bin, time.Second).EnsureService(ctx, endpointOnly); err != nil {
		t.Fatalf("the endpoint host must be used when no SSH host is set: %v", err)
	}
}

func (e *dockerCLIExecutor) VerifyForTest(ctx context.Context, task apigateway.ControllerRuntimeTask) error {
	runCtx, cancel := context.WithTimeout(ctx, e.timeout)
	defer cancel()
	return e.verifySSHContract(runCtx, task)
}

func TestDockerImageAndNetworkPreparation(t *testing.T) {
	ctx := context.Background()
	bin, calls := fakeCLI(t)
	e := dockerExecutor(bin, time.Second)
	if err := e.refreshServiceImage(ctx, "  "); err != nil || len(calls()) != 1 || calls()[0] != "" {
		t.Fatalf("an empty image needs no docker calls: %v %v", err, calls())
	}
	if err := e.refreshServiceImage(ctx, "local/svc"); err != nil || callsMatching(calls(), "pull") != 0 {
		t.Fatalf("local images present on the host are not pulled: %v %v", err, calls())
	}

	bin, _ = fakeCLI(t, on("pull *", reply("registry down", 1)))
	if err := dockerExecutor(bin, time.Second).refreshServiceImage(ctx, "img/svc:1"); err != nil {
		t.Fatalf("a failed pull falls back to a local image: %v", err)
	}
	bin, _ = fakeCLI(t, on("pull *", reply("registry down", 1)), on("image inspect *", reply("no such image", 1)))
	if err := dockerExecutor(bin, time.Second).refreshServiceImage(ctx, "img/svc:1"); err == nil || !strings.Contains(err.Error(), "local fallback") {
		t.Fatalf("no image anywhere must fail: %v", err)
	}
	if err := dockerExecutor(bin, time.Second).runFreshContainer(ctx, runtimeTask()); err == nil {
		t.Fatal("a missing image must stop the run")
	}

	bin, calls = fakeCLI(t, on("network inspect *", reply("no such network", 1)))
	plan, err := dockerExecutor(bin, time.Second).networkPlan(runtimeTask())
	if err != nil {
		t.Fatal(err)
	}
	if err := dockerExecutor(bin, time.Second).ensureNetwork(ctx, plan); err != nil || callsMatching(calls(), "network create") != 1 || !strings.Contains(strings.Join(calls(), "\n"), "--subnet") {
		t.Fatalf("a missing per-service network must be created with its subnet: %v %v", err, calls())
	}
	if err := dockerExecutor(bin, time.Second).ensureNetwork(ctx, plan); err != nil {
		t.Fatal(err)
	}
	shared := plan
	shared.Subnet = ""
	if err := dockerExecutor(bin, time.Second).ensureNetwork(ctx, shared); err != nil {
		t.Fatal(err)
	}
	if err := dockerExecutor(bin, time.Second).ensureNetwork(ctx, shared); err != nil {
		t.Fatal(err)
	}
	if err := dockerExecutor(bin, time.Second).ensureNetwork(ctx, gamenet.DockerNetworkPlan{}); err != nil {
		t.Fatal("an unnamed network plan needs no docker calls")
	}
	bin, _ = fakeCLI(t, on("network inspect *", reply("", 1)), on("network create *", reply("pool overlaps", 1)))
	if err := dockerExecutor(bin, time.Second).runFreshContainer(ctx, runtimeTask()); err == nil {
		t.Fatal("a network that cannot be created must stop the run")
	}
}

func TestDockerFactoryResetPreflightClassification(t *testing.T) {
	ctx := context.Background()
	task := runtimeTask()
	expect := func(name string, err error, preflight bool, changed bool) {
		t.Helper()
		var pre resetPreflightError
		if errors.As(err, &pre) != preflight || (preflight && pre.runtimeMayHaveChanged != changed) {
			t.Fatalf("%s: want preflight=%v changed=%v, got %v (%#v)", name, preflight, changed, err, pre)
		}
		if preflight && errors.Unwrap(pre) == nil {
			t.Fatalf("%s: a preflight error must wrap its cause", name)
		}
	}
	bin, _ := fakeCLI(t)
	bad := dockerExecutor(bin, time.Second)
	bad.captureMode = "bogus"
	expect("bad config", bad.FactoryResetService(ctx, task), true, false)
	expect("bad kind", dockerExecutor(bin, time.Second).FactoryResetService(ctx, apigateway.ControllerRuntimeTask{RuntimeKind: "lxc"}), true, false)

	bin, _ = fakeCLI(t, on(psName, reply("svc-101-1", 0)), on("rm -f svc-101-1", reply("busy", 1)))
	expect("remove container", dockerExecutor(bin, time.Second).FactoryResetService(ctx, task), true, true)

	bin, _ = fakeCLI(t, on(volumeLs, reply("daemon down", 1)))
	err := dockerExecutor(bin, time.Second).FactoryResetService(ctx, task)
	expect("volume lookup", err, true, true)
	var lookup volumeLookupError
	if !errors.As(err, &lookup) || lookup.Error() == "" || errors.Unwrap(lookup) == nil {
		t.Fatalf("lookup failures must be identifiable: %v", err)
	}

	bin, _ = fakeCLI(t, on(volumeLs, reply("vol-101-1", 0)), on("volume rm *", reply("in use", 1)))
	expect("volume still exists", dockerExecutor(bin, time.Second).FactoryResetService(ctx, task), true, true)

	bin, _ = fakeCLI(t, on(volumeLs, reply("vol-101-1", 0)), on("volume rm *", reply("in use", 1)), on("volume inspect *", reply("no such volume", 1)))
	expect("volume gone despite error", dockerExecutor(bin, time.Second).FactoryResetService(ctx, task), false, false)

	bin, _ = fakeCLI(t, on(runCmd, reply("boom", 1)))
	expect("run after delete", dockerExecutor(bin, time.Second).FactoryResetService(ctx, task), false, false)

	bin, calls := fakeCLI(t, on(volumeLs, reply("vol-101-1", 0)))
	if err := dockerExecutor(bin, time.Second).FactoryResetService(ctx, task); err != nil || callsMatching(calls(), "volume rm -f vol-101-1") != 1 {
		t.Fatalf("a reset must delete the state volume and rerun: %v %v", err, calls())
	}
	if err := dockerExecutor(bin, time.Second).removeVolumeIfExists(ctx, " "); err != nil {
		t.Fatal("an unset volume needs no removal")
	}
}

func TestDockerRestartAndCredentialBranches(t *testing.T) {
	ctx := context.Background()
	task := runtimeTask()
	bin, _ := fakeCLI(t)
	bad := dockerExecutor(bin, time.Second)
	bad.captureMode = "bogus"
	if bad.RestartService(ctx, task) == nil || bad.ApplySSHCredential(ctx, task, apigateway.ControllerSSHCredential{Password: "p"}) == nil {
		t.Fatal("invalid capture config must be rejected")
	}
	other := apigateway.ControllerRuntimeTask{RuntimeKind: "lxc"}
	if dockerExecutor(bin, time.Second).RestartService(ctx, other) == nil || dockerExecutor(bin, time.Second).ApplySSHCredential(ctx, other, apigateway.ControllerSSHCredential{}) == nil {
		t.Fatal("unsupported runtime kinds must be rejected")
	}
	off := dockerExecutor(bin, time.Second)
	off.sshApplyMode = "disabled"
	if err := off.ApplySSHCredential(ctx, task, apigateway.ControllerSSHCredential{Password: "p"}); err == nil {
		t.Fatal("a disabled credential apply must fail")
	}

	bin, _ = fakeCLI(t, on(psName, reply("daemon down", 1)))
	if dockerExecutor(bin, time.Second).RestartService(ctx, task) == nil || dockerExecutor(bin, time.Second).ApplySSHCredential(ctx, task, apigateway.ControllerSSHCredential{Password: "p"}) == nil {
		t.Fatal("a failed existence probe must fail")
	}

	// A missing container is recreated before restart or credential apply.
	bin, calls := fakeCLI(t)
	if err := dockerExecutor(bin, time.Second).RestartService(ctx, task); err != nil || callsMatching(calls(), "run -d") != 1 || callsMatching(calls(), "restart") != 0 {
		t.Fatalf("restart of a missing container must recreate it: %v %v", err, calls())
	}
	if err := dockerExecutor(bin, time.Second).ApplySSHCredential(ctx, task, apigateway.ControllerSSHCredential{Password: "pw"}); err != nil || callsMatching(calls(), "run -d") != 2 {
		t.Fatalf("credential apply to a missing container must recreate it: %v %v", err, calls())
	}
	bin, _ = fakeCLI(t, on(runCmd, reply("boom", 1)))
	if dockerExecutor(bin, time.Second).RestartService(ctx, task) == nil || dockerExecutor(bin, time.Second).ApplySSHCredential(ctx, task, apigateway.ControllerSSHCredential{Password: "p"}) == nil {
		t.Fatal("a failed recreation must fail")
	}

	bin, calls = fakeCLI(t, on(psName, reply("svc-101-1", 0)))
	if err := dockerExecutor(bin, time.Second).RestartService(ctx, task); err != nil || callsMatching(calls(), "restart svc-101-1") != 1 {
		t.Fatalf("an existing container is restarted in place: %v %v", err, calls())
	}
	bin, _ = fakeCLI(t, on(psName, reply("svc-101-1", 0)), on("restart *", reply("boom", 1)))
	if err := dockerExecutor(bin, time.Second).RestartService(ctx, task); err == nil {
		t.Fatal("a failed restart must fail")
	}
	bin, _ = fakeCLI(t, on(psName, reply("svc-101-1", 0)), on("exec -e AD_PLATFORM_ROOT_PASSWORD=*", reply("chpasswd failed", 1)))
	if err := dockerExecutor(bin, time.Second).ApplySSHCredential(ctx, task, apigateway.ControllerSSHCredential{Password: "p"}); err == nil {
		t.Fatal("a failed password set must fail")
	}
	bin, _ = fakeCLI(t, on(psName, reply("svc-101-1", 0)), on(execSvc, reply("no sshd", 1)))
	if err := dockerExecutor(bin, time.Second).ApplySSHCredential(ctx, task, apigateway.ControllerSSHCredential{Password: "p"}); err == nil {
		t.Fatal("a failed SSH contract must stop the credential apply")
	}
}

type scriptedValidator struct {
	result apigateway.CheckerValidationResult
	err    error
}

func (v scriptedValidator) Validate(context.Context, apigateway.CheckerValidationRequest) (apigateway.CheckerValidationResult, error) {
	return v.result, v.err
}

func TestDockerValidateChallengeRuntimeBranches(t *testing.T) {
	ctx := context.Background()
	request := apigateway.ChallengeValidationRequest{ChallengeID: 1, BaselineImage: "img", CheckerImage: "chk"}
	bin, _ := fakeCLI(t)
	if result, _ := dockerExecutor(bin, time.Second).ValidateChallengeRuntime(ctx, apigateway.ChallengeValidationRequest{ChallengeID: 1, CheckerImage: "chk"}); result.Status != "invalid" {
		t.Fatalf("a missing baseline image is invalid: %+v", result)
	}
	failing, _ := fakeCLI(t, on("run --rm *", reply("no sshd", 1)))
	if result, _ := dockerExecutor(failing, time.Second).ValidateChallengeRuntime(ctx, request); result.Status != "invalid" || result.BaselineSSHContractOK {
		t.Fatalf("a failing baseline probe is invalid: %+v", result)
	}
	baselineOnly, _ := fakeCLI(t, seq("run --rm *", reply("", 0), reply("no checker", 1)))
	if result, _ := dockerExecutor(baselineOnly, time.Second).ValidateChallengeRuntime(ctx, request); result.Status != "invalid" || !result.BaselineSSHContractOK {
		t.Fatalf("a failing checker probe is invalid: %+v", result)
	}
	for name, tc := range map[string]struct {
		validator scriptedValidator
		wantErr   bool
		want      string
	}{
		"runner error": {scriptedValidator{err: errors.New("runner down")}, true, ""},
		"invalid":      {scriptedValidator{result: apigateway.CheckerValidationResult{Status: "invalid", Message: "bad checker"}}, false, "invalid"},
		"no state":     {scriptedValidator{result: apigateway.CheckerValidationResult{Status: "valid", ContractOK: true}}, false, "invalid"},
		"fully valid":  {scriptedValidator{result: apigateway.CheckerValidationResult{Status: "valid", ContractOK: true, ServiceStateContractOK: true}}, false, "valid"},
	} {
		e := dockerExecutor(bin, time.Second)
		e.checkerValidator = tc.validator
		result, err := e.ValidateChallengeRuntime(ctx, request)
		if (err != nil) != tc.wantErr || (!tc.wantErr && result.Status != tc.want) {
			t.Fatalf("%s: %+v %v", name, result, err)
		}
	}
}

func TestDockerRemoveByFilter(t *testing.T) {
	ctx := context.Background()
	listing := "c1 adplatform.team_id=101,adplatform.state_volume=vol-a\n\nc2 adplatform.team_id=101\nc3\n"
	bin, calls := fakeCLI(t, on("ps -a --filter label=*", reply(listing, 0)), on("rm -f c2", reply("busy", 1)), on("volume ls --filter name=^vol-a$ *", reply("vol-a", 0)), on("volume rm *", reply("in use", 1)))
	e := dockerExecutor(bin, time.Second)
	if err := e.RemoveTeamServices(ctx, 101); err == nil {
		t.Fatal("a failed container removal must be reported")
	}
	if callsMatching(calls(), "rm -f c1") != 1 || callsMatching(calls(), "rm -f c3") != 1 || callsMatching(calls(), "volume rm -f vol-a") != 1 {
		t.Fatalf("every container must be attempted and its volume removed: %v", calls())
	}
	bin, _ = fakeCLI(t, on("ps -a --filter label=*", reply("daemon down", 1)))
	e = dockerExecutor(bin, time.Second)
	if e.RemoveService(ctx, 101, 1) == nil || e.RemoveChallengeServices(ctx, 1) == nil {
		t.Fatal("a failed listing must fail")
	}
	bin, _ = fakeCLI(t)
	e = dockerExecutor(bin, time.Second)
	if e.RemoveService(ctx, 101, 1) != nil || e.RemoveChallengeServices(ctx, 1) != nil {
		t.Fatal("no matching containers is a success")
	}
}

func TestRuntimeExecutorConfiguration(t *testing.T) {
	t.Setenv("CONTROLLER_RUNTIME_MODE", "docker")
	t.Setenv("CONTROLLER_SERVICE_CAP_ADD", " NET_BIND_SERVICE, ,CHOWN ")
	t.Setenv("CONTROLLER_SERVICE_MEMORY", "none")
	e, ok := newRuntimeExecutor().(*dockerCLIExecutor)
	if !ok || strings.Join(e.serviceSecurity.capAdd, ",") != "NET_BIND_SERVICE,CHOWN" || e.serviceSecurity.memory != "" {
		t.Fatalf("docker executor config: %+v", e)
	}
	t.Setenv("CONTROLLER_SERVICE_CAP_ADD", "disabled")
	if e := newRuntimeExecutor().(*dockerCLIExecutor); e.serviceSecurity.capAdd != nil {
		t.Fatalf("disabled list: %v", e.serviceSecurity.capAdd)
	}
	t.Setenv("CONTROLLER_RUNTIME_MODE", "dry-run")
	dry, ok := newRuntimeExecutor().(dryRunExecutor)
	if !ok {
		t.Fatal("dry-run mode")
	}
	ctx := context.Background()
	task := runtimeTask()
	if dry.EnsureService(ctx, task) != nil || dry.FactoryResetService(ctx, task) != nil || dry.RestartService(ctx, task) != nil || dry.ApplySSHCredential(ctx, task, apigateway.ControllerSSHCredential{}) != nil ||
		dry.RemoveService(ctx, 1, 1) != nil || dry.RemoveTeamServices(ctx, 1) != nil || dry.RemoveChallengeServices(ctx, 1) != nil {
		t.Fatal("dry-run operations succeed")
	}
	if result, err := dry.ValidateChallengeRuntime(ctx, apigateway.ChallengeValidationRequest{ChallengeID: 3}); err != nil || result.Status != "valid" {
		t.Fatalf("dry-run validation: %+v %v", result, err)
	}
}

func TestFactoryResetReportsFailedExistenceProbe(t *testing.T) {
	bin, _ := fakeCLI(t, on(psName, reply("daemon down", 1)))
	var pre resetPreflightError
	if err := dockerExecutor(bin, time.Second).FactoryResetService(context.Background(), runtimeTask()); !errors.As(err, &pre) || !pre.runtimeMayHaveChanged {
		t.Fatalf("a failed probe before deletion is a preflight failure: %v", err)
	}
}
