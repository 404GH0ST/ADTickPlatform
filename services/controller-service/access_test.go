package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"adplatform/internal/services/apigateway"
)

func TestRenderControllerAccessRulesEnforcesPublicServiceAndSSHAllowlist(t *testing.T) {
	rules := renderControllerAccessRules("adplatform_service_access", "wg0", "eth0", []apigateway.ControllerServiceAccessPolicy{
		{TeamID: 101, ChallengeID: 1, ChallengeName: "banking", ServiceIP: "10.80.1.11", ServicePort: 10001, SSHPort: 22, SSHUnlocked: true, EgressEnabled: true, AllowedPeerAddresses: []string{"10.70.11.20", "10.70.11.21"}},
		{TeamID: 102, ChallengeID: 1, ChallengeName: "banking", ServiceIP: "10.80.1.12", ServicePort: 10001, SSHPort: 22, SSHUnlocked: false, EgressEnabled: true},
	})

	if !strings.Contains(rules, `ct state established,related accept`) {
		t.Fatalf("expected established traffic rule")
	}
	if !strings.Contains(rules, `iifname "wg0" ip daddr 10.80.1.11 tcp dport 10001 accept`) {
		t.Fatalf("expected public service port rule")
	}
	if !strings.Contains(rules, `ip daddr 10.80.1.11 tcp dport 10001 accept`) {
		t.Fatalf("expected public service fallback rule")
	}
	if !strings.Contains(rules, `oifname "wg0" ip saddr 10.80.1.11 tcp sport 10001 accept`) {
		t.Fatalf("expected reverse service port rule")
	}
	// The interface-agnostic source-port accept must NOT be emitted: replies are
	// covered by the global established/related accept, and a bare version would
	// let a service originate NEW packets (e.g. to another team's SSH) past the
	// SSH drop. Only the oifname-scoped reverse rule above is allowed; guard
	// against an unscoped variant (a line beginning `ip saddr ... sport`) being
	// reintroduced.
	if strings.Contains(rules, "\n    ip saddr 10.80.1.11 tcp sport 10001") {
		t.Fatalf("interface-agnostic reverse service-port accept must not be emitted")
	}
	if !strings.Contains(rules, `oifname "wg0" ip saddr 10.80.1.11 ct state established,related accept`) {
		t.Fatalf("expected reverse service rule")
	}
	if !strings.Contains(rules, `ip daddr 10.80.1.11 tcp dport 22 ip saddr { 10.70.11.20, 10.70.11.21 } accept`) {
		t.Fatalf("expected ssh fallback allowlist rule")
	}
	if !strings.Contains(rules, `ip daddr 10.80.1.12 tcp dport 22 drop`) {
		t.Fatalf("expected locked ssh fallback drop rule")
	}
	for _, chain := range strings.Split(rules, "  chain ")[1:] {
		allow := strings.Index(chain, `ip daddr 10.80.1.11 tcp dport 22 ip saddr { 10.70.11.20, 10.70.11.21 } accept`)
		deny := strings.Index(chain, `ip daddr 10.80.1.11 tcp dport 22 drop`)
		established := strings.Index(chain, `ct state established,related accept`)
		if allow < 0 || deny <= allow || established <= deny {
			t.Fatalf("expected SSH allowlist and drop before accepting established traffic: %s", chain)
		}
	}
	// Residual default-deny for other WG destinations on open service IPs.
	if !strings.Contains(rules, `iifname "wg0" ip daddr 10.80.1.11 drop`) {
		t.Fatalf("expected residual WG drop for open service 10.80.1.11, got:\n%s", rules)
	}
	if !strings.Contains(rules, `iifname "wg0" ip daddr 10.80.1.12 drop`) {
		t.Fatalf("expected residual WG drop for open service 10.80.1.12, got:\n%s", rules)
	}
	// Residual drop must come after service-port accept.
	acceptIdx := strings.Index(rules, `iifname "wg0" ip daddr 10.80.1.11 tcp dport 10001 accept`)
	dropIdx := strings.Index(rules, `iifname "wg0" ip daddr 10.80.1.11 drop`)
	if acceptIdx < 0 || dropIdx < 0 || acceptIdx > dropIdx {
		t.Fatalf("expected service accept before residual drop, got:\n%s", rules)
	}
	if strings.Contains(rules, "flush table inet") {
		t.Fatalf("expected rules to omit flush table directive")
	}
}

func TestRenderControllerAccessRulesDropsTrafficWhenNetworkClosed(t *testing.T) {
	rules := renderControllerAccessRules("adplatform_service_access", "wg0", "eth0", []apigateway.ControllerServiceAccessPolicy{
		{
			TeamID: 101, ChallengeID: 1, ChallengeName: "banking",
			ServiceIP: "10.80.1.11", ServicePort: 10001, SSHPort: 22,
			SSHUnlocked: true, EgressEnabled: true,
			// While closed, AllowedPeerAddresses are organizer WireGuard peers only.
			AllowedPeerAddresses: []string{"10.70.0.2"},
			NetworkClosed:        true,
		},
	})

	// Organizer WG may probe; other WG peers dropped; host nc stays open.
	if !strings.Contains(rules, `ip saddr { 10.70.0.2 } ip daddr 10.80.1.11 accept`) {
		t.Fatalf("expected organizer WireGuard accept while closed, got:\n%s", rules)
	}
	if !strings.Contains(rules, `iifname "wg0" ip daddr 10.80.1.11 drop`) {
		t.Fatalf("expected WG-interface drop for closed service, got:\n%s", rules)
	}
	// Blanket dest drop would block host-local nc — must not appear when wg iif is set.
	for _, line := range strings.Split(rules, "\n") {
		if strings.TrimSpace(line) == "ip daddr 10.80.1.11 drop" {
			t.Fatalf("did not expect full destination drop (blocks host nc):\n%s", rules)
		}
	}
	acceptIdx := strings.Index(rules, `ip saddr { 10.70.0.2 } ip daddr 10.80.1.11 accept`)
	dropIdx := strings.Index(rules, `iifname "wg0" ip daddr 10.80.1.11 drop`)
	// Closed rules live only in forward; output must not drop closed destinations.
	outputIdx := strings.Index(rules, "chain output")
	if outputIdx < 0 {
		t.Fatalf("expected output chain, got:\n%s", rules)
	}
	outputSection := rules[outputIdx:]
	if strings.Contains(outputSection, `ip daddr 10.80.1.11 drop`) || strings.Contains(outputSection, `iifname "wg0" ip daddr 10.80.1.11 drop`) {
		t.Fatalf("output chain must allow host nc while closed, got:\n%s", outputSection)
	}
	if acceptIdx < 0 || dropIdx < 0 || acceptIdx > dropIdx {
		t.Fatalf("expected organizer accept before WG drop, got:\n%s", rules)
	}
	if strings.Contains(rules, `tcp dport 10001 accept`) {
		t.Fatalf("did not expect public service accept while network closed:\n%s", rules)
	}
}

func TestRenderControllerAccessRulesKeepsEgressDisabledWhenNetworkClosed(t *testing.T) {
	rules := renderControllerAccessRules("adplatform_service_access", "wg0", "eth0", []apigateway.ControllerServiceAccessPolicy{{
		TeamID: 101, ChallengeID: 7, ChallengeName: "airgapped",
		ServiceIP: "10.80.7.11", ServicePort: 10007, SSHPort: 22,
		EgressEnabled: false, NetworkClosed: true,
	}})

	if !strings.Contains(rules, `oifname "eth0" ip saddr 10.80.7.11 drop`) {
		t.Fatalf("expected egress drop for closed airgapped service, got:\n%s", rules)
	}
}

func TestMatchPauseRulesCloseBridgedAndRoutedServiceTraffic(t *testing.T) {
	policy := apigateway.ControllerServiceAccessPolicy{
		ServiceIP: "10.80.1.11", NetworkClosed: true, AllowedPeerAddresses: []string{"10.70.0.2"},
	}
	if rules, paused := renderMatchPauseRules("access", "wg0", []apigateway.ControllerServiceAccessPolicy{policy}); paused || strings.Contains(rules, "hook forward") {
		t.Fatalf("pre-match warmup must not be blocked: %s", rules)
	}
	policy.MatchPaused = true
	rules, paused := renderMatchPauseRules("access", "wg0", []apigateway.ControllerServiceAccessPolicy{policy})
	if !paused || !strings.Contains(rules, "table bridge access_pause {") || !strings.Contains(rules, "table inet access_pause {") ||
		strings.Count(rules, "ip daddr { 10.80.1.11 } drop") != 2 ||
		!strings.Contains(rules, `iifname "wg0" ip saddr { 10.70.0.2 } ip daddr 10.80.1.11 accept`) ||
		strings.Contains(rules, "hook output") {
		t.Fatalf("pause must close both forwarding paths and retain operator access: %s", rules)
	}
	policy.MatchPaused = false
	policy.PlayDeferred = true
	rules, paused = renderMatchPauseRules("access", "wg0", []apigateway.ControllerServiceAccessPolicy{policy})
	if !paused || strings.Count(rules, "ip daddr { 10.80.1.11 } drop") != 2 ||
		!strings.Contains(rules, `iifname "wg0" ip saddr { 10.70.0.2 } ip daddr 10.80.1.11 accept`) {
		t.Fatalf("deferred play must close both forwarding paths and retain operator access: %s", rules)
	}
	policy.PlayDeferred = false
	policy.MatchFinished = true
	rules, paused = renderMatchPauseRules("access", "wg0", []apigateway.ControllerServiceAccessPolicy{policy})
	if !paused || !strings.Contains(rules, "table bridge access_pause {") ||
		strings.Count(rules, "ip daddr { 10.80.1.11 } drop") != 2 ||
		!strings.Contains(rules, `iifname "wg0" ip saddr { 10.70.0.2 } ip daddr 10.80.1.11 accept`) {
		t.Fatalf("finished match must close bridged and routed service traffic: %s", rules)
	}
}

func TestFileServiceAccessExecutorWritesArtifacts(t *testing.T) {
	tmpDir := t.TempDir()
	executor := &fileServiceAccessExecutor{
		mode:            "files",
		interfaceName:   "wg0",
		firewallBackend: "nftables",
		firewallTable:   "adplatform_service_access",
		paths: controllerAccessArtifactPaths{
			rulesPath:  filepath.Join(tmpDir, "access.nft"),
			statusPath: filepath.Join(tmpDir, "access-status.json"),
		},
	}

	status, err := executor.Apply(context.Background(), []apigateway.ControllerServiceAccessPolicy{{
		TeamID: 101, TeamName: "Team Alpha", ChallengeID: 1, ChallengeName: "banking", ServiceIP: "10.80.1.11", ServicePort: 10001, SSHPort: 22, SSHUnlocked: true, AllowedPeerAddresses: []string{"10.70.11.20"},
	}}, time.Date(2026, time.March, 10, 8, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("expected apply to succeed, got %v", err)
	}
	if status.Mode != "files" || status.PoliciesTotal != 1 || status.SSHOpenServices != 1 {
		t.Fatalf("unexpected status %+v", status)
	}
	body, err := os.ReadFile(executor.paths.rulesPath)
	if err != nil {
		t.Fatalf("expected rules file, got %v", err)
	}
	if !strings.Contains(string(body), "10.80.1.11") {
		t.Fatalf("expected service ip in rules")
	}
}

func TestHostServiceAccessExecutorRunsNft(t *testing.T) {
	tmpDir := t.TempDir()
	runner := &recordingControllerCommandRunner{
		failCommands: map[string]error{
			"iptables -C DOCKER-USER -j ADPLATFORM-WG-SERVICES":  errors.New("rule not found"),
			"iptables -C FORWARD -j DOCKER-USER":                 errors.New("rule not found"),
			"iptables -C FORWARD -j ADPLATFORM-WG-SERVICES":      errors.New("rule not found"),
			"iptables -t raw -C PREROUTING -j ADPLATFORM-WG-RAW": errors.New("rule not found"),
		},
	}
	executor := &hostServiceAccessExecutor{
		fileServiceAccessExecutor: fileServiceAccessExecutor{
			mode:            "host",
			interfaceName:   "wg0",
			firewallBackend: "nftables",
			firewallTable:   "adplatform_service_access",
			paths: controllerAccessArtifactPaths{
				rulesPath:  filepath.Join(tmpDir, "access.nft"),
				statusPath: filepath.Join(tmpDir, "access-status.json"),
			},
		},
		nftBinary:      "nft",
		iptablesBinary: "iptables",
		applyTimeout:   5 * time.Second,
		runner:         runner,
	}

	status, err := executor.Apply(context.Background(), []apigateway.ControllerServiceAccessPolicy{{
		TeamID: 101, TeamName: "Team Alpha", ChallengeID: 1, ChallengeName: "banking", ServiceIP: "10.80.1.11", ServicePort: 10001, SSHPort: 22, SSHUnlocked: true, AllowedPeerAddresses: []string{"10.70.11.20"},
	}}, time.Date(2026, time.March, 10, 8, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("expected apply to succeed, got %v", err)
	}
	if len(runner.commands) < 8 {
		t.Fatalf("unexpected command count %#v", runner.commands)
	}
	if !containsControllerCommand(runner.commands, "iptables -S DOCKER") {
		t.Fatalf("expected backend detection command %#v", runner.commands)
	}
	if !containsControllerCommand(runner.rules, "-I PREROUTING 1 -j ADPLATFORM-WG-RAW") {
		t.Fatalf("expected raw prerouting jump command %#v", runner.commands)
	}
	if !containsControllerCommand(runner.rules, "-A ADPLATFORM-WG-RAW -i wg0 -d 10.80.1.11/32 -j ACCEPT") {
		t.Fatalf("expected raw service bypass command %#v", runner.commands)
	}
	if containsControllerCommand(runner.commands, "nft delete table inet adplatform_service_access") {
		t.Fatalf("nftables apply must replace the table in a single command: %v", runner.commands)
	}
	nftRules, err := os.ReadFile(executor.paths.rulesPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(nftRules), "add table inet adplatform_service_access\ndelete table inet adplatform_service_access\ntable inet adplatform_service_access {") {
		t.Fatalf("nftables replacement must be in the rules file: %s", nftRules)
	}
	if !slices.Equal(runner.tables, []string{"*filter", "*raw"}) {
		t.Fatalf("expected one restore per table, with filter first: %v", runner.tables)
	}
	for _, rule := range []string{":ADPLATFORM-WG-SERVICES - [0:0]", ":ADPLATFORM-WG-RAW - [0:0]"} {
		if !slices.Contains(runner.rules, rule) {
			t.Fatalf("missing owned chain declaration %q in %v", rule, runner.rules)
		}
	}
	if slices.Contains(runner.rules, ":DOCKER-USER - [0:0]") {
		t.Fatalf("restoring an existing Docker chain would flush its rules: %v", runner.rules)
	}
	if !containsControllerCommand(runner.commands, "nft -f "+executor.paths.rulesPath) {
		t.Fatalf("expected nft apply command %#v", runner.commands)
	}
	if !containsControllerCommand(runner.rules, "-I DOCKER-USER 1 -j ADPLATFORM-WG-SERVICES") {
		t.Fatalf("expected docker-user jump command %#v", runner.commands)
	}
	if !containsControllerCommand(runner.rules, "-I FORWARD 1 -j DOCKER-USER") {
		t.Fatalf("expected forward jump command %#v", runner.commands)
	}
	if !containsControllerCommand(runner.rules, "-I FORWARD 1 -j ADPLATFORM-WG-SERVICES") {
		t.Fatalf("expected direct chain forward jump command %#v", runner.commands)
	}
	if !containsControllerCommand(runner.rules, "-A ADPLATFORM-WG-SERVICES -i wg0 -d 10.80.1.11/32 -p tcp --dport 10001 -j ACCEPT") {
		t.Fatalf("expected service accept command %#v", runner.commands)
	}
	if !containsControllerCommand(runner.rules, "-A ADPLATFORM-WG-SERVICES -d 10.80.1.11/32 -p tcp --dport 10001 -j ACCEPT") {
		t.Fatalf("expected service fallback accept command %#v", runner.commands)
	}
	if !containsControllerCommand(runner.rules, "-A ADPLATFORM-WG-SERVICES -o wg0 -s 10.80.1.11/32 -p tcp --sport 10001 -j ACCEPT") {
		t.Fatalf("expected reverse service port accept command %#v", runner.commands)
	}
	// The interface-agnostic source-port accept must not be emitted (replies are
	// covered by the global ESTABLISHED,RELATED accept; a bare version would let
	// a service originate NEW packets past the SSH drop).
	if containsControllerCommand(runner.rules, "-A ADPLATFORM-WG-SERVICES -s 10.80.1.11/32 -p tcp --sport 10001 -j ACCEPT") ||
		containsControllerCommand(runner.rules, "-A ADPLATFORM-WG-SERVICES -s 10.80.1.11/32 -p tcp --sport 10001 -m conntrack --ctstate ESTABLISHED,RELATED -j ACCEPT") {
		t.Fatalf("interface-agnostic reverse service-port accept must not be emitted %#v", runner.commands)
	}
	if !containsControllerCommand(runner.rules, "-A ADPLATFORM-WG-SERVICES -o wg0 -s 10.80.1.11/32 -m conntrack --ctstate ESTABLISHED,RELATED -j ACCEPT") {
		t.Fatalf("expected reverse accept command %#v", runner.commands)
	}
	if !containsControllerCommand(runner.rules, "-A ADPLATFORM-WG-SERVICES -s 10.70.11.20 -d 10.80.1.11/32 -p tcp --dport 22 -j ACCEPT") {
		t.Fatalf("expected ssh fallback allowlist command %#v", runner.commands)
	}
	if !containsControllerCommand(runner.rules, "-A ADPLATFORM-WG-SERVICES -d 10.80.1.11/32 -p tcp --dport 22 -j DROP") {
		t.Fatalf("expected ssh fallback drop command %#v", runner.commands)
	}
	allowSSH := slices.Index(runner.rules, "-A ADPLATFORM-WG-SERVICES -s 10.70.11.20 -d 10.80.1.11/32 -p tcp --dport 22 -j ACCEPT")
	denySSH := slices.Index(runner.rules, "-A ADPLATFORM-WG-SERVICES -d 10.80.1.11/32 -p tcp --dport 22 -j DROP")
	established := slices.Index(runner.rules, "-A ADPLATFORM-WG-SERVICES -m conntrack --ctstate ESTABLISHED,RELATED -j ACCEPT")
	if allowSSH < 0 || denySSH <= allowSSH || established <= denySSH {
		t.Fatalf("expected SSH allowlist and drop before accepting established traffic: %v", runner.commands)
	}
	if !containsControllerCommand(runner.rules, "-A ADPLATFORM-WG-SERVICES -i wg0 -d 10.80.1.11/32 -j DROP") {
		t.Fatalf("expected residual WG default-deny for open service %#v", runner.commands)
	}
	if status.Mode != "host" || status.FirewallBackend != "nftables" {
		t.Fatalf("unexpected status %+v", status)
	}
}

func TestHostServiceAccessExecutorSurfacesRunnerError(t *testing.T) {
	tmpDir := t.TempDir()
	executor := &hostServiceAccessExecutor{
		fileServiceAccessExecutor: fileServiceAccessExecutor{
			mode:            "host",
			interfaceName:   "wg0",
			firewallBackend: "nftables",
			firewallTable:   "adplatform_service_access",
			paths: controllerAccessArtifactPaths{
				rulesPath:  filepath.Join(tmpDir, "access.nft"),
				statusPath: filepath.Join(tmpDir, "access-status.json"),
			},
		},
		nftBinary:      "nft",
		iptablesBinary: "iptables",
		applyTimeout:   5 * time.Second,
		runner: failingControllerCommandRunner{
			defaultErr: errors.New("nft apply failed"),
			allowCommands: map[string]struct{}{
				"iptables -S DOCKER": {},
			},
		},
	}

	status, err := executor.Apply(context.Background(), nil, time.Date(2026, time.March, 10, 8, 0, 0, 0, time.UTC))
	if err == nil {
		t.Fatal("expected apply to fail")
	}
	if status.State != "error" || !strings.Contains(status.LastError, "nft apply failed") {
		t.Fatalf("unexpected status %+v", status)
	}
}

func TestHostServiceAccessExecutorIptablesOnlyDeletesNftTable(t *testing.T) {
	tmpDir := t.TempDir()
	runner := &recordingControllerCommandRunner{
		failCommands: map[string]error{
			"iptables -C DOCKER-USER -j ADPLATFORM-WG-SERVICES":  errors.New("rule not found"),
			"iptables -C FORWARD -j DOCKER-USER":                 errors.New("rule not found"),
			"iptables -C FORWARD -j ADPLATFORM-WG-SERVICES":      errors.New("rule not found"),
			"iptables -t raw -C PREROUTING -j ADPLATFORM-WG-RAW": errors.New("rule not found"),
		},
	}
	executor := &hostServiceAccessExecutor{
		fileServiceAccessExecutor: fileServiceAccessExecutor{
			mode:            "host",
			interfaceName:   "wg0",
			firewallBackend: "iptables",
			firewallTable:   "adplatform_service_access",
			paths: controllerAccessArtifactPaths{
				rulesPath:  filepath.Join(tmpDir, "access.nft"),
				statusPath: filepath.Join(tmpDir, "access-status.json"),
			},
		},
		nftBinary:      "nft",
		iptablesBinary: "iptables",
		applyTimeout:   5 * time.Second,
		runner:         runner,
	}

	status, err := executor.Apply(context.Background(), []apigateway.ControllerServiceAccessPolicy{{
		TeamID: 102, TeamName: "tim2", ChallengeID: 1, ChallengeName: "test", ServiceIP: "10.80.2.12", ServicePort: 2323, SSHPort: 22, SSHUnlocked: true, AllowedPeerAddresses: []string{"10.70.11.20", "10.70.12.22"},
	}}, time.Date(2026, time.March, 12, 8, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("expected apply to succeed, got %v", err)
	}
	if status.Mode != "host" || status.FirewallBackend != "iptables" {
		t.Fatalf("unexpected status %+v", status)
	}
	// Must still delete any orphaned nftables table
	if !containsControllerCommand(runner.commands, "nft delete table inet adplatform_service_access") {
		t.Fatalf("expected nft delete command when using iptables backend, got %#v", runner.commands)
	}
	// The regular rules use iptables. nftables still owns the pause guard.
	if containsControllerCommand(runner.commands, "nft -f "+executor.paths.rulesPath) ||
		!containsControllerCommand(runner.commands, "nft -f "+executor.paths.rulesPath+".pause") {
		t.Fatalf("unexpected nftables apply commands: %#v", runner.commands)
	}
	// SSH allowlist rules must still be applied via iptables
	if !containsControllerCommand(runner.rules, "-A ADPLATFORM-WG-SERVICES -i wg0 -s 10.70.11.20 -d 10.80.2.12/32 -p tcp --dport 22 -j ACCEPT") {
		t.Fatalf("expected ssh allowlist for organizer %#v", runner.commands)
	}
	if !containsControllerCommand(runner.rules, "-A ADPLATFORM-WG-SERVICES -i wg0 -s 10.70.12.22 -d 10.80.2.12/32 -p tcp --dport 22 -j ACCEPT") {
		t.Fatalf("expected ssh allowlist for team member %#v", runner.commands)
	}
	if !containsControllerCommand(runner.rules, "-A ADPLATFORM-WG-SERVICES -i wg0 -d 10.80.2.12/32 -p tcp --dport 22 -j DROP") {
		t.Fatalf("expected ssh drop rule %#v", runner.commands)
	}
}

type recordingControllerCommandRunner struct {
	commands     []string
	tables       []string
	rules        []string
	failCommands map[string]error
}

func (r *recordingControllerCommandRunner) Run(_ context.Context, binary string, args ...string) error {
	command := strings.TrimSpace(binary + " " + strings.Join(args, " "))
	r.commands = append(r.commands, command)
	if err, ok := r.failCommands[command]; ok {
		return err
	}
	if strings.HasSuffix(binary, "-restore") {
		if len(args) != 3 || args[0] != "--wait" || args[1] != "--noflush" {
			return fmt.Errorf("unexpected restore options: %v", args)
		}
		body, err := os.ReadFile(args[2])
		if err != nil {
			return err
		}
		lines := strings.Split(strings.TrimSpace(string(body)), "\n")
		if lines[len(lines)-1] != "COMMIT" {
			return fmt.Errorf("restore is missing COMMIT: %s", body)
		}
		r.tables = append(r.tables, lines[0])
		r.rules = append(r.rules, lines[1:len(lines)-1]...)
	}
	return nil
}

type failingControllerCommandRunner struct {
	defaultErr    error
	allowCommands map[string]struct{}
}

func (f failingControllerCommandRunner) Run(_ context.Context, binary string, args ...string) error {
	command := strings.TrimSpace(binary + " " + strings.Join(args, " "))
	if _, ok := f.allowCommands[command]; ok {
		return nil
	}
	return f.defaultErr
}

func containsControllerCommand(commands []string, want string) bool {
	for _, command := range commands {
		if command == want {
			return true
		}
	}
	return false
}

func TestHostServiceAccessExecutorIptablesDropsWhenNetworkClosed(t *testing.T) {
	tmpDir := t.TempDir()
	runner := &recordingControllerCommandRunner{
		failCommands: map[string]error{
			"iptables -C DOCKER-USER -j ADPLATFORM-WG-SERVICES":  errors.New("rule not found"),
			"iptables -C FORWARD -j DOCKER-USER":                 errors.New("rule not found"),
			"iptables -C FORWARD -j ADPLATFORM-WG-SERVICES":      errors.New("rule not found"),
			"iptables -t raw -C PREROUTING -j ADPLATFORM-WG-RAW": errors.New("rule not found"),
		},
	}
	executor := &hostServiceAccessExecutor{
		fileServiceAccessExecutor: fileServiceAccessExecutor{
			mode:              "host",
			interfaceName:     "wg0",
			internetInterface: "eth0",
			firewallBackend:   "iptables",
			firewallTable:     "adplatform_service_access",
			paths: controllerAccessArtifactPaths{
				rulesPath:  filepath.Join(tmpDir, "access.nft"),
				statusPath: filepath.Join(tmpDir, "access-status.json"),
			},
		},
		nftBinary:      "nft",
		iptablesBinary: "iptables",
		applyTimeout:   5 * time.Second,
		runner:         runner,
	}

	_, err := executor.Apply(context.Background(), []apigateway.ControllerServiceAccessPolicy{{
		TeamID: 101, ChallengeID: 1, ChallengeName: "sealbroker",
		ServiceIP: "10.80.1.12", ServicePort: 8160, SSHPort: 22,
		SSHUnlocked: true, EgressEnabled: false,
		AllowedPeerAddresses: []string{"10.70.0.2"}, // organizer WG
		NetworkClosed:        true,
	}}, time.Date(2026, time.March, 10, 8, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("expected apply to succeed, got %v", err)
	}
	if !containsControllerCommand(runner.rules, "-A ADPLATFORM-WG-SERVICES -i wg0 -s 10.70.0.2 -d 10.80.1.12/32 -j ACCEPT") {
		t.Fatalf("expected organizer WireGuard accept while closed, got %#v", runner.commands)
	}
	if !containsControllerCommand(runner.rules, "-A ADPLATFORM-WG-SERVICES -i wg0 -d 10.80.1.12/32 -j DROP") {
		t.Fatalf("expected WG-interface drop while closed, got %#v", runner.commands)
	}
	if !containsControllerCommand(runner.rules, "-A ADPLATFORM-WG-SERVICES -o eth0 -s 10.80.1.12/32 -j DROP") {
		t.Fatalf("expected egress drop while closed, got %#v", runner.commands)
	}
	// Full dest DROP would block host paths that hairpin through FORWARD.
	if containsControllerCommand(runner.rules, "-A ADPLATFORM-WG-SERVICES -d 10.80.1.12/32 -j DROP") {
		t.Fatalf("did not expect full destination DROP while wg iif is set, got %#v", runner.commands)
	}
	// No public service-port ACCEPT for participants.
	if containsControllerCommand(runner.rules, "-A ADPLATFORM-WG-SERVICES -d 10.80.1.12/32 -p tcp --dport 8160 -j ACCEPT") {
		t.Fatalf("did not expect public service ACCEPT while closed, got %#v", runner.commands)
	}
}

func TestHostServiceAccessExecutorIptablesDropsEgressWhenDisabled(t *testing.T) {
	tmpDir := t.TempDir()
	runner := &recordingControllerCommandRunner{
		failCommands: map[string]error{
			"iptables -C DOCKER-USER -j ADPLATFORM-WG-SERVICES":  errors.New("rule not found"),
			"iptables -C FORWARD -j DOCKER-USER":                 errors.New("rule not found"),
			"iptables -C FORWARD -j ADPLATFORM-WG-SERVICES":      errors.New("rule not found"),
			"iptables -t raw -C PREROUTING -j ADPLATFORM-WG-RAW": errors.New("rule not found"),
		},
	}
	executor := &hostServiceAccessExecutor{
		fileServiceAccessExecutor: fileServiceAccessExecutor{
			mode:              "host",
			interfaceName:     "wg0",
			internetInterface: "eth0",
			firewallBackend:   "iptables",
			firewallTable:     "adplatform_service_access",
			paths: controllerAccessArtifactPaths{
				rulesPath:  filepath.Join(tmpDir, "access.nft"),
				statusPath: filepath.Join(tmpDir, "access-status.json"),
			},
		},
		nftBinary:      "nft",
		iptablesBinary: "iptables",
		applyTimeout:   5 * time.Second,
		runner:         runner,
	}

	_, err := executor.Apply(context.Background(), []apigateway.ControllerServiceAccessPolicy{{
		TeamID: 101, TeamName: "Team Alpha", ChallengeID: 1, ChallengeName: "airgapped",
		ServiceIP: "10.80.7.11", ServicePort: 10007, SSHPort: 22, SSHUnlocked: false, EgressEnabled: false,
	}}, time.Date(2026, time.March, 10, 8, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("expected apply to succeed, got %v", err)
	}
	if !containsControllerCommand(runner.rules, "-A ADPLATFORM-WG-SERVICES -o eth0 -s 10.80.7.11/32 -j DROP") {
		t.Fatalf("expected iptables egress drop for airgapped service, got %#v", runner.commands)
	}
}

func TestHostServiceAccessExecutorIptablesSkipsEgressDropWithoutInternetInterface(t *testing.T) {
	tmpDir := t.TempDir()
	runner := &recordingControllerCommandRunner{
		failCommands: map[string]error{
			"iptables -C DOCKER-USER -j ADPLATFORM-WG-SERVICES":  errors.New("rule not found"),
			"iptables -C FORWARD -j DOCKER-USER":                 errors.New("rule not found"),
			"iptables -C FORWARD -j ADPLATFORM-WG-SERVICES":      errors.New("rule not found"),
			"iptables -t raw -C PREROUTING -j ADPLATFORM-WG-RAW": errors.New("rule not found"),
		},
	}
	executor := &hostServiceAccessExecutor{
		fileServiceAccessExecutor: fileServiceAccessExecutor{
			mode:              "host",
			interfaceName:     "wg0",
			internetInterface: "",
			firewallBackend:   "iptables",
			firewallTable:     "adplatform_service_access",
			paths: controllerAccessArtifactPaths{
				rulesPath:  filepath.Join(tmpDir, "access.nft"),
				statusPath: filepath.Join(tmpDir, "access-status.json"),
			},
		},
		nftBinary:      "nft",
		iptablesBinary: "iptables",
		applyTimeout:   5 * time.Second,
		runner:         runner,
	}

	_, err := executor.Apply(context.Background(), []apigateway.ControllerServiceAccessPolicy{{
		TeamID: 101, ServiceIP: "10.80.7.11", ServicePort: 10007, SSHPort: 22, SSHUnlocked: false, EgressEnabled: false,
	}}, time.Date(2026, time.March, 10, 8, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("expected apply to succeed, got %v", err)
	}
	for _, cmd := range runner.rules {
		if strings.HasPrefix(cmd, "-A ADPLATFORM-WG-SERVICES -o eth0") {
			t.Fatalf("expected no -o eth0 rule without internet interface, got %q", cmd)
		}
	}
}

func TestRenderControllerAccessRulesDropsEgressWhenDisabled(t *testing.T) {
	rules := renderControllerAccessRules("adplatform_service_access", "wg0", "eth0", []apigateway.ControllerServiceAccessPolicy{
		{TeamID: 101, ChallengeID: 1, ChallengeName: "airgapped", ServiceIP: "10.80.7.11", ServicePort: 10007, SSHPort: 22, SSHUnlocked: false, EgressEnabled: false},
		{TeamID: 102, ChallengeID: 2, ChallengeName: "open", ServiceIP: "10.80.8.11", ServicePort: 10008, SSHPort: 22, SSHUnlocked: false, EgressEnabled: true},
	})

	if !strings.Contains(rules, `oifname "eth0" ip saddr 10.80.7.11 drop`) {
		t.Fatalf("expected egress drop for airgapped service, got:\n%s", rules)
	}
	if strings.Contains(rules, `oifname "eth0" ip saddr 10.80.8.11 drop`) {
		t.Fatalf("expected no egress drop for open service, got:\n%s", rules)
	}
}

func TestRenderControllerAccessRulesSkipsEgressDropWithoutInternetInterface(t *testing.T) {
	rules := renderControllerAccessRules("adplatform_service_access", "wg0", "", []apigateway.ControllerServiceAccessPolicy{
		{TeamID: 101, ChallengeID: 1, ChallengeName: "airgapped", ServiceIP: "10.80.7.11", ServicePort: 10007, SSHPort: 22, SSHUnlocked: false, EgressEnabled: false},
	})

	if strings.Contains(rules, "eth0") {
		t.Fatalf("expected no internet interface rules when controller is unconfigured, got:\n%s", rules)
	}
}

func TestBuildControllerAccessStatusCountsEgressDisabledServices(t *testing.T) {
	status := buildControllerAccessStatus("files", "wg0", "eth0", "nftables", "/tmp/rules", []apigateway.ControllerServiceAccessPolicy{
		{TeamID: 101, ServiceIP: "10.80.7.11", ServicePort: 10007, EgressEnabled: false},
		{TeamID: 102, ServiceIP: "10.80.7.12", ServicePort: 10007, EgressEnabled: false},
		{TeamID: 103, ServiceIP: "10.80.7.13", ServicePort: 10007, EgressEnabled: true},
	}, time.Date(2026, time.March, 10, 8, 0, 0, 0, time.UTC))

	if status.EgressDisabledServices != 2 {
		t.Fatalf("expected 2 egress disabled services, got %d", status.EgressDisabledServices)
	}
	if status.InternetInterface != "eth0" {
		t.Fatalf("expected internet interface to be reported, got %q", status.InternetInterface)
	}
}
