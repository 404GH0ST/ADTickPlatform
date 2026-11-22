package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"adplatform/internal/services/apigateway"
)

// faultRunner records commands and fails those whose 1-based index is in
// failAt, or that a predicate rejects.
type faultRunner struct {
	mu     sync.Mutex
	calls  []string
	failAt map[int]bool
	reject func(call string) bool
}

func (r *faultRunner) Run(_ context.Context, binary string, args ...string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	call := strings.TrimSpace(binary + " " + strings.Join(args, " "))
	r.calls = append(r.calls, call)
	if r.failAt[len(r.calls)] || (r.reject != nil && r.reject(call)) {
		return errors.New("simulated failure: " + call)
	}
	return nil
}

func accessPolicies() []apigateway.ControllerServiceAccessPolicy {
	return []apigateway.ControllerServiceAccessPolicy{
		{TeamID: 101, ChallengeID: 1, ServiceIP: "10.80.1.101", ServicePort: 10001, SSHPort: 22, SSHUnlocked: true, AllowedPeerAddresses: []string{"10.70.1.2", "not-an-ip"}},
		{TeamID: 102, ChallengeID: 1, ServiceIP: "10.80.1.102", ServicePort: 10001, SSHPort: 22},
		{TeamID: 103, ChallengeID: 1, ServiceIP: "10.80.1.103", ServicePort: 0, SSHPort: 0, EgressEnabled: true},
		{TeamID: 104, ChallengeID: 2, ServiceIP: "10.80.2.104", NetworkClosed: true, AllowedPeerAddresses: []string{"10.70.0.9", "bogus"}},
		{TeamID: 105, ChallengeID: 2, ServiceIP: "invalid"},
	}
}

func hostAccess(t *testing.T, runner controllerCommandRunner, backend, iface, internet string) *hostServiceAccessExecutor {
	t.Helper()
	dir := t.TempDir()
	return &hostServiceAccessExecutor{
		fileServiceAccessExecutor: fileServiceAccessExecutor{mode: "host", interfaceName: iface, internetInterface: internet, firewallBackend: backend, firewallTable: "adp_access",
			paths: controllerAccessArtifactPaths{rulesPath: filepath.Join(dir, "rules.nft"), statusPath: filepath.Join(dir, "status.json")}},
		nftBinary: "nft", iptablesBinary: "iptables", applyTimeout: 5 * time.Second, runner: runner,
	}
}

// A failed host firewall apply must never be reported as applied.
func TestHostAccessApplyReportsEveryCommandFailure(t *testing.T) {
	now := time.Now()
	for _, cfg := range []struct{ backend, iface, internet string }{
		{"nftables", "wg0", "eth0"},
		{"iptables", "wg0", "eth0"},
		{"iptables", "", ""},
	} {
		clean := &faultRunner{}
		if status, err := hostAccess(t, clean, cfg.backend, cfg.iface, cfg.internet).Apply(context.Background(), accessPolicies(), now); err != nil || status.State == "error" {
			t.Fatalf("%+v: clean apply: %+v %v", cfg, status, err)
		}
		total := len(clean.calls)
		for _, width := range []int{1, 2} {
			for n := 1; n <= total; n++ {
				failAt := map[int]bool{n: true}
				if width == 2 {
					failAt[n+1] = true
				}
				status, err := hostAccess(t, &faultRunner{failAt: failAt}, cfg.backend, cfg.iface, cfg.internet).Apply(context.Background(), accessPolicies(), now)
				if (err != nil) != (status.State == "error") {
					t.Fatalf("%+v fail at %v: status %q disagrees with error %v", cfg, failAt, status.State, err)
				}
			}
		}
	}
}

func TestSelectIptablesBinaryFallbacks(t *testing.T) {
	ctx := context.Background()
	e := hostAccess(t, &faultRunner{reject: func(call string) bool {
		return strings.Contains(call, "-S DOCKER") || (strings.Contains(call, "-S FORWARD") && !strings.HasPrefix(call, "iptables-legacy"))
	}}, "iptables", "wg0", "")
	if binary, err := e.selectIptablesBinary(ctx); err != nil || binary != "iptables-legacy" {
		t.Fatalf("FORWARD fallback: %q %v", binary, err)
	}
	e = hostAccess(t, &faultRunner{reject: func(call string) bool {
		return strings.Contains(call, " -S DOCKER") && !strings.Contains(call, "DOCKER-USER")
	}}, "iptables", "wg0", "")
	if binary, err := e.selectIptablesBinary(ctx); err != nil || binary != "iptables" {
		t.Fatalf("DOCKER-USER fallback: %q %v", binary, err)
	}
	everything := &faultRunner{reject: func(string) bool { return true }}
	e = hostAccess(t, everything, "iptables", "wg0", "")
	e.iptablesBinary = " "
	if _, err := e.selectIptablesBinary(ctx); err == nil {
		t.Fatal("no usable backend must fail")
	}
	if status, err := e.Apply(ctx, accessPolicies(), time.Now()); err == nil || status.State != "error" {
		t.Fatalf("apply without a backend: %+v %v", status, err)
	}
	if err := e.Teardown(ctx); err == nil || !strings.Contains(err.Error(), "rules.nft.pause") {
		t.Fatalf("teardown must report a failed pause guard removal: %v", err)
	}
}

func TestHostAccessTeardownAndStatus(t *testing.T) {
	runner := &faultRunner{}
	e := hostAccess(t, runner, "nftables", "wg0", "")
	if _, err := e.Apply(context.Background(), accessPolicies(), time.Now()); err != nil {
		t.Fatal(err)
	}
	if status := e.Status(); status.State != "applied" || status.Mode != "host" {
		t.Fatalf("status after apply: %+v", status)
	}
	if err := e.Teardown(context.Background()); err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(runner.calls, "\n")
	for _, want := range []string{"nft delete table inet adp_access", "-D DOCKER-USER -j ADPLATFORM-WG-SERVICES", "-t raw -X ADPLATFORM-WG-RAW"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("teardown missing %q:\n%s", want, joined)
		}
	}
	if _, err := os.Stat(e.paths.rulesPath); !os.IsNotExist(err) {
		t.Fatal("teardown must remove the rules artifact")
	}
}

func TestFileAccessExecutorBranches(t *testing.T) {
	dir := t.TempDir()
	file := &fileServiceAccessExecutor{mode: "files", interfaceName: "wg0", firewallBackend: "nftables", firewallTable: "t", paths: controllerAccessArtifactPaths{rulesPath: filepath.Join(dir, "r"), statusPath: filepath.Join(dir, "s.json")}}
	if status := (&fileServiceAccessExecutor{mode: "files"}).Status(); status.State != "idle" {
		t.Fatalf("no status path: %+v", status)
	}
	if status := file.Status(); status.State != "idle" {
		t.Fatalf("missing status file: %+v", status)
	}
	mustWrite(t, file.paths.statusPath, "{broken")
	if status := file.Status(); status.State != "error" {
		t.Fatalf("corrupt status: %+v", status)
	}
	mustWrite(t, file.paths.statusPath, `{"state":"applied","mode":"","interface":"","firewall_backend":"","rules_path":"","status_path":""}`)
	if status := file.Status(); status.Mode != "files" || status.Interface != "wg0" || status.FirewallBackend != "nftables" || status.RulesPath == "" || status.StatusPath == "" {
		t.Fatalf("blank fields must fall back to configuration: %+v", status)
	}
	if _, err := file.Apply(context.Background(), accessPolicies(), time.Now()); err != nil {
		t.Fatal(err)
	}
	if err := file.Teardown(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := (&fileServiceAccessExecutor{}).Teardown(context.Background()); err != nil {
		t.Fatal(err)
	}

	blocker := filepath.Join(t.TempDir(), "file")
	mustWrite(t, blocker, "x")
	for _, paths := range []controllerAccessArtifactPaths{
		{rulesPath: filepath.Join(blocker, "r"), statusPath: filepath.Join(dir, "s2.json")},
		{rulesPath: filepath.Join(dir, "r2"), statusPath: filepath.Join(blocker, "s")},
		{rulesPath: dir, statusPath: filepath.Join(dir, "s3.json")},
	} {
		blocked := &fileServiceAccessExecutor{mode: "files", paths: paths}
		if status, err := blocked.Apply(context.Background(), accessPolicies(), time.Now()); err == nil || status.State != "error" {
			t.Fatalf("unwritable artifacts %+v: %+v %v", paths, status, err)
		}
		host := hostAccess(t, &faultRunner{}, "nftables", "wg0", "")
		host.paths = paths
		if status, err := host.Apply(context.Background(), accessPolicies(), time.Now()); err == nil || status.State != "error" {
			t.Fatalf("host apply with unwritable artifacts %+v: %+v %v", paths, status, err)
		}
	}

	// A status file that cannot be rewritten after a successful host apply.
	host := hostAccess(t, &statusBreaker{statusPath: filepath.Join(dir, "late.json")}, "nftables", "wg0", "")
	host.paths.statusPath = filepath.Join(dir, "late.json")
	if status, err := host.Apply(context.Background(), accessPolicies(), time.Now()); err == nil || status.State != "error" {
		t.Fatalf("final status write failure: %+v %v", status, err)
	}
	if err := writeControllerAccessStatus("", apigateway.ControllerAccessStatus{}); err != nil {
		t.Fatal("an unset status path is a no-op")
	}
}

// statusBreaker turns the status file into a directory mid-apply.
type statusBreaker struct{ statusPath string }

func (b *statusBreaker) Run(_ context.Context, binary string, args ...string) error {
	if binary == "nft" && len(args) > 0 && args[0] == "-f" {
		_ = os.Remove(b.statusPath)
		_ = os.MkdirAll(b.statusPath, 0o700)
	}
	return nil
}

func TestAccessExecutorSelectionAndHelpers(t *testing.T) {
	for mode, check := range map[string]func(serviceAccessExecutor) bool{
		"host":    func(e serviceAccessExecutor) bool { _, ok := e.(*hostServiceAccessExecutor); return ok },
		"files":   func(e serviceAccessExecutor) bool { _, ok := e.(*fileServiceAccessExecutor); return ok },
		"dry-run": func(e serviceAccessExecutor) bool { _, ok := e.(dryRunServiceAccessExecutor); return ok },
	} {
		t.Setenv("CONTROLLER_ACCESS_MODE", mode)
		if !check(newServiceAccessExecutor()) {
			t.Fatalf("mode %q selected the wrong executor", mode)
		}
	}
	dry := dryRunServiceAccessExecutor{}
	if dry.Status().State != "idle" || dry.Teardown(context.Background()) != nil {
		t.Fatal("dry-run access executor")
	}
	if status, err := dry.Apply(context.Background(), accessPolicies(), time.Now()); err != nil || status.Mode != "dry-run" {
		t.Fatalf("dry-run apply: %+v %v", status, err)
	}
	if err := (execControllerCommandRunner{}).Run(context.Background(), "true"); err != nil {
		t.Fatal(err)
	}
	if err := (execControllerCommandRunner{}).Run(context.Background(), "sh", "-c", "echo denied >&2; exit 1"); err == nil || !strings.Contains(err.Error(), "denied") {
		t.Fatalf("command failures must include output: %v", err)
	}
	for in, want := range map[string]string{"": "adplatform_service_access", "!!!": "adplatform_service_access", "Team-Table_9": "teamtable_9"} {
		if got := sanitizeControllerNftTableName(in); got != want {
			t.Fatalf("sanitize(%q) = %q, want %q", in, got, want)
		}
	}
}
