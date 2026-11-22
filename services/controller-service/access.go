package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"adplatform/internal/platform/config"
	"adplatform/internal/services/apigateway"
)

func isValidIP(ip string) bool {
	return net.ParseIP(strings.TrimSpace(ip)) != nil
}

type serviceAccessExecutor interface {
	Status() apigateway.ControllerAccessStatus
	Apply(context.Context, []apigateway.ControllerServiceAccessPolicy, time.Time) (apigateway.ControllerAccessStatus, error)
	Teardown(ctx context.Context) error
}

// marshalAccessStatus is a seam so tests can exercise encoding failures.
var marshalAccessStatus = json.MarshalIndent

type controllerAccessArtifactPaths struct {
	rulesPath  string
	statusPath string
}

type dryRunServiceAccessExecutor struct{}

type fileServiceAccessExecutor struct {
	mode              string
	interfaceName     string
	internetInterface string
	firewallBackend   string
	firewallTable     string
	paths             controllerAccessArtifactPaths
}

type hostServiceAccessExecutor struct {
	fileServiceAccessExecutor
	nftBinary      string
	iptablesBinary string
	applyTimeout   time.Duration
	runner         controllerCommandRunner
}

type controllerCommandRunner interface {
	Run(ctx context.Context, binary string, args ...string) error
}

type execControllerCommandRunner struct{}

func newServiceAccessExecutor() serviceAccessExecutor {
	base := fileServiceAccessExecutor{
		mode:              strings.ToLower(config.String("CONTROLLER_ACCESS_MODE", "dry-run")),
		interfaceName:     strings.TrimSpace(config.String("CONTROLLER_ACCESS_INTERFACE", "wg0")),
		internetInterface: strings.TrimSpace(config.String("CONTROLLER_INTERNET_INTERFACE", "eth0")),
		firewallBackend:   strings.ToLower(strings.TrimSpace(config.String("CONTROLLER_ACCESS_FIREWALL_BACKEND", "nftables"))),
		firewallTable:     sanitizeControllerNftTableName(config.String("CONTROLLER_ACCESS_FIREWALL_TABLE", "adplatform_service_access")),
		paths: controllerAccessArtifactPaths{
			rulesPath:  strings.TrimSpace(config.String("CONTROLLER_ACCESS_RULES_PATH", ".runtime/controller/access.nft")),
			statusPath: strings.TrimSpace(config.String("CONTROLLER_ACCESS_STATUS_PATH", ".runtime/controller/access-status.json")),
		},
	}
	if base.mode == "host" {
		return &hostServiceAccessExecutor{
			fileServiceAccessExecutor: base,
			nftBinary:                 strings.TrimSpace(config.String("CONTROLLER_ACCESS_NFT_BIN", "nft")),
			iptablesBinary:            strings.TrimSpace(config.String("CONTROLLER_ACCESS_IPTABLES_BIN", "iptables")),
			applyTimeout:              config.Duration("CONTROLLER_ACCESS_TIMEOUT", 10*time.Second),
			runner:                    execControllerCommandRunner{},
		}
	}
	if base.mode == "files" {
		return &base
	}
	return dryRunServiceAccessExecutor{}
}

func (dryRunServiceAccessExecutor) Status() apigateway.ControllerAccessStatus {
	return apigateway.ControllerAccessStatus{State: "idle", Mode: "dry-run"}
}

func (dryRunServiceAccessExecutor) Apply(_ context.Context, policies []apigateway.ControllerServiceAccessPolicy, now time.Time) (apigateway.ControllerAccessStatus, error) {
	return buildControllerAccessStatus("dry-run", "", "", "", "", policies, now), nil
}

func (dryRunServiceAccessExecutor) Teardown(_ context.Context) error {
	return nil
}

func (e *fileServiceAccessExecutor) Status() apigateway.ControllerAccessStatus {
	status := apigateway.ControllerAccessStatus{
		State:           "idle",
		Mode:            e.mode,
		Interface:       e.interfaceName,
		FirewallBackend: e.firewallBackend,
		RulesPath:       e.paths.rulesPath,
		StatusPath:      e.paths.statusPath,
	}
	if strings.TrimSpace(e.paths.statusPath) == "" {
		return status
	}
	payload, err := os.ReadFile(e.paths.statusPath)
	if err != nil {
		return status
	}
	if err := json.Unmarshal(payload, &status); err != nil {
		status.State = "error"
		status.LastError = err.Error()
	}
	if status.Mode == "" {
		status.Mode = e.mode
	}
	if status.Interface == "" {
		status.Interface = e.interfaceName
	}
	if status.FirewallBackend == "" {
		status.FirewallBackend = e.firewallBackend
	}
	if status.RulesPath == "" {
		status.RulesPath = e.paths.rulesPath
	}
	if status.StatusPath == "" {
		status.StatusPath = e.paths.statusPath
	}
	return status
}

func (e *fileServiceAccessExecutor) Apply(_ context.Context, policies []apigateway.ControllerServiceAccessPolicy, now time.Time) (apigateway.ControllerAccessStatus, error) {
	status := buildControllerAccessStatus(e.mode, e.interfaceName, e.internetInterface, e.firewallBackend, e.paths.rulesPath, policies, now)
	rules := renderControllerAccessRules(e.firewallTable, e.interfaceName, e.internetInterface, policies)
	if err := writeControllerAccessArtifacts(e.paths, rules, status); err != nil {
		status.State = "error"
		status.LastError = err.Error()
		_ = writeControllerAccessStatus(e.paths.statusPath, status)
		return status, err
	}
	return status, nil
}

func (e *fileServiceAccessExecutor) Teardown(_ context.Context) error {
	if strings.TrimSpace(e.paths.rulesPath) != "" {
		_ = os.Remove(e.paths.rulesPath) // #nosec G703 -- configured artifact path, not request input.
	}
	if strings.TrimSpace(e.paths.statusPath) != "" {
		_ = os.Remove(e.paths.statusPath) // #nosec G703 -- configured artifact path, not request input.
	}
	return nil
}

func (e *hostServiceAccessExecutor) Apply(ctx context.Context, policies []apigateway.ControllerServiceAccessPolicy, now time.Time) (apigateway.ControllerAccessStatus, error) {
	log.Printf("applying %d service access policies", len(policies))
	for _, p := range policies {
		log.Printf("policy: team=%d (%s), challenge=%d (%s), ip=%s, port=%d, unlocked=%v, closed=%v, peers=%v",
			p.TeamID, p.TeamName, p.ChallengeID, p.ChallengeName, p.ServiceIP, p.ServicePort, p.SSHUnlocked, p.NetworkClosed, p.AllowedPeerAddresses)
	}

	status := buildControllerAccessStatus("host", e.interfaceName, e.internetInterface, e.firewallBackend, e.paths.rulesPath, policies, now)
	rules := renderControllerAccessRules(e.firewallTable, e.interfaceName, e.internetInterface, policies)
	if err := writeControllerAccessArtifacts(e.paths, rules, status); err != nil {
		status.State = "error"
		status.LastError = err.Error()
		_ = writeControllerAccessStatus(e.paths.statusPath, status)
		return status, err
	}

	applyCtx, cancel := context.WithTimeout(ctx, e.applyTimeout)
	defer cancel()
	pauseRules, guarded := renderMatchPauseRules(e.firewallTable, e.interfaceName, policies)
	// Close the bridge before changing the routed rules. On resume keep the
	// pause guard until the ordinary access policy has been applied.
	if guarded {
		if err := e.applyMatchPauseRules(applyCtx, pauseRules); err != nil {
			status.State = "error"
			status.LastError = err.Error()
			_ = writeControllerAccessStatus(e.paths.statusPath, status)
			return status, err
		}
	}
	iptablesBinary, err := e.selectIptablesBinary(applyCtx)
	if err != nil {
		status.State = "error"
		status.LastError = err.Error()
		_ = writeControllerAccessStatus(e.paths.statusPath, status)
		return status, err
	}
	log.Printf("using iptables binary: %s", iptablesBinary)

	if e.firewallBackend == "nftables" {
		log.Printf("applying nftables rules from %s", e.paths.rulesPath)
		if err := e.runner.Run(applyCtx, e.nftBinary, "-f", e.paths.rulesPath); err != nil {
			status.State = "error"
			status.LastError = err.Error()
			_ = writeControllerAccessStatus(e.paths.statusPath, status)
			return status, err
		}
	}

	// Docker still uses the filter and raw hooks with either backend. Put the
	// restrictive filter rules in place before updating the raw bypasses.
	if err := e.applyDockerUserRules(applyCtx, iptablesBinary, policies); err != nil {
		status.State = "error"
		status.LastError = err.Error()
		_ = writeControllerAccessStatus(e.paths.statusPath, status)
		return status, err
	}
	if err := e.applyDockerRawRules(applyCtx, iptablesBinary, policies); err != nil {
		status.State = "error"
		status.LastError = err.Error()
		_ = writeControllerAccessStatus(e.paths.statusPath, status)
		return status, err
	}
	if e.firewallBackend != "nftables" {
		// Leave any previous nftables policy in place until its replacement
		// is active, including when switching backends.
		_ = e.runner.Run(applyCtx, e.nftBinary, "delete", "table", "inet", e.firewallTable)
	}
	if !guarded {
		if err := e.applyMatchPauseRules(applyCtx, pauseRules); err != nil {
			status.State = "error"
			status.LastError = err.Error()
			_ = writeControllerAccessStatus(e.paths.statusPath, status)
			return status, err
		}
	}
	log.Printf("access policies applied successfully")
	if err := writeControllerAccessStatus(e.paths.statusPath, status); err != nil {
		status.State = "error"
		status.LastError = err.Error()
		return status, err
	}
	return status, nil
}

func (e *hostServiceAccessExecutor) applyMatchPauseRules(ctx context.Context, rules string) error {
	path := e.paths.rulesPath + ".pause"
	if err := ensureControllerParentDir(path); err != nil {
		return err
	}
	if err := os.WriteFile(path, []byte(rules), 0o600); err != nil {
		return err
	}
	return e.runner.Run(ctx, e.nftBinary, "-f", path)
}

func (e *hostServiceAccessExecutor) Teardown(ctx context.Context) error {
	applyCtx, cancel := context.WithTimeout(ctx, e.applyTimeout)
	defer cancel()

	// Always clean up nftables table to prevent orphaned rules from conflicting.
	_ = e.runner.Run(applyCtx, e.nftBinary, "delete", "table", "inet", e.firewallTable)

	iptablesBinary, err := e.selectIptablesBinary(applyCtx)
	if err == nil {
		// Clean up User chains
		const userChain = "ADPLATFORM-WG-SERVICES"
		_ = e.runner.Run(applyCtx, iptablesBinary, "-t", "filter", "-D", "DOCKER-USER", "-j", userChain)
		_ = e.runner.Run(applyCtx, iptablesBinary, "-t", "filter", "-D", "FORWARD", "-j", userChain)
		_ = e.runner.Run(applyCtx, iptablesBinary, "-t", "filter", "-F", userChain)
		_ = e.runner.Run(applyCtx, iptablesBinary, "-t", "filter", "-X", userChain)

		// Clean up Raw chains
		const rawChain = "ADPLATFORM-WG-RAW"
		_ = e.runner.Run(applyCtx, iptablesBinary, "-t", "raw", "-D", "PREROUTING", "-j", rawChain)
		_ = e.runner.Run(applyCtx, iptablesBinary, "-t", "raw", "-F", rawChain)
		_ = e.runner.Run(applyCtx, iptablesBinary, "-t", "raw", "-X", rawChain)
	}

	pauseRules, _ := renderMatchPauseRules(e.firewallTable, e.interfaceName, nil)
	if err := e.applyMatchPauseRules(applyCtx, pauseRules); err != nil {
		return err
	}
	_ = os.Remove(e.paths.rulesPath + ".pause")
	return e.fileServiceAccessExecutor.Teardown(ctx)
}

func (execControllerCommandRunner) Run(ctx context.Context, binary string, args ...string) error {
	cmd := exec.CommandContext(ctx, binary, args...) // #nosec G204,G702 -- binary is host operator configuration; args are passed without shell expansion.
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s %s failed: %w: %s", binary, strings.Join(args, " "), err, strings.TrimSpace(string(output)))
	}
	return nil
}

func (e *hostServiceAccessExecutor) applyDockerRawRules(ctx context.Context, binary string, policies []apigateway.ControllerServiceAccessPolicy) error {
	const chain = "ADPLATFORM-WG-RAW"
	var rules strings.Builder
	fmt.Fprintf(&rules, "*raw\n:%s - [0:0]\n", chain)
	if err := e.runner.Run(ctx, binary, "-t", "raw", "-C", "PREROUTING", "-j", chain); err != nil {
		fmt.Fprintf(&rules, "-I PREROUTING 1 -j %s\n", chain)
	}
	for _, policy := range policies {
		if isValidIP(policy.ServiceIP) {
			// Filter still drops participant traffic to closed services.
			fmt.Fprintf(&rules, "-A %s%s -d %s/32 -j ACCEPT\n", chain, iptablesInterface("-i", e.interfaceName), policy.ServiceIP)
		}
	}
	return e.restoreIptablesTable(ctx, binary, rules.String())
}

func (e *hostServiceAccessExecutor) applyDockerUserRules(ctx context.Context, binary string, policies []apigateway.ControllerServiceAccessPolicy) error {
	const chain = "ADPLATFORM-WG-SERVICES"
	const dockerUser = "DOCKER-USER"

	var rules strings.Builder
	fmt.Fprintf(&rules, "*filter\n:%s - [0:0]\n", chain)
	if err := e.runner.Run(ctx, binary, "-S", dockerUser); err != nil {
		// A declaration would flush any existing Docker-owned rules.
		fmt.Fprintf(&rules, "-N %s\n", dockerUser)
	}
	for _, jump := range [][2]string{{"FORWARD", dockerUser}, {dockerUser, chain}, {"FORWARD", chain}} {
		if err := e.runner.Run(ctx, binary, "-C", jump[0], "-j", jump[1]); err != nil {
			fmt.Fprintf(&rules, "-I %s 1 -j %s\n", jump[0], jump[1])
		}
	}

	ingress := iptablesInterface("-i", e.interfaceName)
	egress := iptablesInterface("-o", e.interfaceName)
	// Closed services and SSH are checked before established connections.
	for _, policy := range policies {
		if !policy.NetworkClosed || !isValidIP(policy.ServiceIP) {
			continue
		}
		for _, peer := range policy.AllowedPeerAddresses {
			if !isValidIP(peer) {
				continue
			}
			if ingress != "" {
				fmt.Fprintf(&rules, "-A %s%s -s %s -d %s/32 -j ACCEPT\n", chain, ingress, peer, policy.ServiceIP)
			}
			fmt.Fprintf(&rules, "-A %s -s %s -d %s/32 -j ACCEPT\n", chain, peer, policy.ServiceIP)
		}
		fmt.Fprintf(&rules, "-A %s%s -d %s/32 -j DROP\n", chain, ingress, policy.ServiceIP)
	}
	appendIptablesSSHAccessRules(&rules, chain, ingress, policies)
	fmt.Fprintf(&rules, "-A %s -m conntrack --ctstate ESTABLISHED,RELATED -j ACCEPT\n", chain)
	for _, policy := range policies {
		if !isValidIP(policy.ServiceIP) {
			continue
		}
		if !policy.EgressEnabled && e.internetInterface != "" {
			fmt.Fprintf(&rules, "-A %s%s -s %s/32 -j DROP\n", chain, iptablesInterface("-o", e.internetInterface), policy.ServiceIP)
		}
		if policy.NetworkClosed {
			continue
		}
		if ingress != "" {
			fmt.Fprintf(&rules, "-A %s%s -d %s/32 -p tcp --dport %d -j ACCEPT\n", chain, ingress, policy.ServiceIP, policy.ServicePort)
		}
		fmt.Fprintf(&rules, "-A %s -d %s/32 -p tcp --dport %d -j ACCEPT\n", chain, policy.ServiceIP, policy.ServicePort)
		if egress != "" {
			// A source-port accept without an interface would let services
			// originate new packets using their service port as a bypass.
			fmt.Fprintf(&rules, "-A %s%s -s %s/32 -p tcp --sport %d -j ACCEPT\n", chain, egress, policy.ServiceIP, policy.ServicePort)
			fmt.Fprintf(&rules, "-A %s%s -s %s/32 -m conntrack --ctstate ESTABLISHED,RELATED -j ACCEPT\n", chain, egress, policy.ServiceIP)
			fmt.Fprintf(&rules, "-A %s%s -d %s/32 -j DROP\n", chain, ingress, policy.ServiceIP)
		}
	}
	return e.restoreIptablesTable(ctx, binary, rules.String())
}

func appendIptablesSSHAccessRules(rules *strings.Builder, chain, ingress string, policies []apigateway.ControllerServiceAccessPolicy) {
	for _, policy := range policies {
		if policy.NetworkClosed || !isValidIP(policy.ServiceIP) || policy.SSHPort <= 0 {
			continue
		}
		if policy.SSHUnlocked {
			for _, peer := range policy.AllowedPeerAddresses {
				if !isValidIP(peer) {
					continue
				}
				if ingress != "" {
					fmt.Fprintf(rules, "-A %s%s -s %s -d %s/32 -p tcp --dport %d -j ACCEPT\n", chain, ingress, peer, policy.ServiceIP, policy.SSHPort)
				}
				fmt.Fprintf(rules, "-A %s -s %s -d %s/32 -p tcp --dport %d -j ACCEPT\n", chain, peer, policy.ServiceIP, policy.SSHPort)
			}
		}
		if ingress != "" {
			fmt.Fprintf(rules, "-A %s%s -d %s/32 -p tcp --dport %d -j DROP\n", chain, ingress, policy.ServiceIP, policy.SSHPort)
		}
		fmt.Fprintf(rules, "-A %s -d %s/32 -p tcp --dport %d -j DROP\n", chain, policy.ServiceIP, policy.SSHPort)
	}
}

func iptablesInterface(flag, name string) string {
	if name == "" {
		return ""
	}
	return " " + flag + " " + name
}

func (e *hostServiceAccessExecutor) restoreIptablesTable(ctx context.Context, binary, rules string) error {
	file, err := os.CreateTemp(filepath.Dir(e.paths.rulesPath), ".iptables-restore-*")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	_, writeErr := file.WriteString(rules + "COMMIT\n")
	if err := file.Close(); err != nil {
		return err
	}
	if writeErr != nil {
		return writeErr
	}
	// --noflush preserves the rest of the table. Declaring our chain above
	// replaces only that chain, with its new rules and any jumps in one commit.
	return e.runner.Run(ctx, binary+"-restore", "--wait", "--noflush", file.Name())
}

func (e *hostServiceAccessExecutor) selectIptablesBinary(ctx context.Context) (string, error) {
	candidates := []string{e.iptablesBinary, "iptables", "iptables-legacy", "iptables-nft"}
	seen := make(map[string]struct{})
	for _, candidate := range candidates {
		candidate = strings.TrimSpace(candidate)
		if candidate == "" {
			continue
		}
		if _, ok := seen[candidate]; ok {
			continue
		}
		seen[candidate] = struct{}{}
		if err := e.runner.Run(ctx, candidate, "-S", "DOCKER"); err == nil {
			return candidate, nil
		}
		if err := e.runner.Run(ctx, candidate, "-S", "DOCKER-USER"); err == nil {
			return candidate, nil
		}
	}
	for _, candidate := range candidates {
		candidate = strings.TrimSpace(candidate)
		if candidate == "" {
			continue
		}
		if err := e.runner.Run(ctx, candidate, "-S", "FORWARD"); err == nil {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("no usable iptables backend found for controller access enforcement")
}

func buildControllerAccessStatus(mode, interfaceName, internetInterface, firewallBackend, rulesPath string, policies []apigateway.ControllerServiceAccessPolicy, now time.Time) apigateway.ControllerAccessStatus {
	sshOpen := 0
	egressDisabled := 0
	allowedPeers := make(map[string]struct{})
	for _, policy := range policies {
		if policy.SSHUnlocked {
			sshOpen++
		}
		if !policy.EgressEnabled {
			egressDisabled++
		}
		for _, peer := range policy.AllowedPeerAddresses {
			allowedPeers[peer] = struct{}{}
		}
	}
	tableName := sanitizeControllerNftTableName(config.String("CONTROLLER_ACCESS_FIREWALL_TABLE", "adplatform_service_access"))
	rules := renderControllerAccessRules(tableName, interfaceName, internetInterface, policies)
	if pauseRules, paused := renderMatchPauseRules(tableName, interfaceName, policies); paused {
		rules += pauseRules
	}
	return apigateway.ControllerAccessStatus{
		State:                  "applied",
		Mode:                   mode,
		Interface:              interfaceName,
		InternetInterface:      internetInterface,
		FirewallBackend:        firewallBackend,
		RulesPath:              rulesPath,
		PoliciesTotal:          len(policies),
		SSHOpenServices:        sshOpen,
		SSHLockedServices:      len(policies) - sshOpen,
		EgressDisabledServices: egressDisabled,
		AllowedPeersTotal:      len(allowedPeers),
		Revision:               controllerAccessRevision(rules),
		AppliedAt:              now.UTC().Format(time.RFC3339),
	}
}

func renderControllerAccessRules(tableName, interfaceName, internetInterface string, policies []apigateway.ControllerServiceAccessPolicy) string {
	var builder strings.Builder
	fmt.Fprintf(&builder, "add table inet %[1]s\ndelete table inet %[1]s\n", tableName)
	builder.WriteString(fmt.Sprintf("table inet %s {\n", tableName))
	// Forward: participant WG → services (closed = organizer allow + WG drop).
	// Output: host-local probes (nc on the contest host) — always allowed so
	// operators can check warm redeploys without a WG client.
	builder.WriteString("  chain forward {\n")
	builder.WriteString("    type filter hook forward priority filter;\n")
	builder.WriteString("    policy accept;\n")
	appendClosedNetworkRules(&builder, interfaceName, policies)
	appendOpenServiceSSHAccessRules(&builder, interfaceName, policies)
	builder.WriteString("    ct state established,related accept\n")
	appendControllerAccessPolicyRules(&builder, interfaceName, internetInterface, policies, "forward")
	builder.WriteString("  }\n")
	builder.WriteString("  chain output {\n")
	builder.WriteString("    type filter hook output priority filter;\n")
	builder.WriteString("    policy accept;\n")
	appendOpenServiceSSHAccessRules(&builder, "", policies)
	builder.WriteString("    ct state established,related accept\n")
	appendControllerAccessPolicyRules(&builder, "", internetInterface, policies, "output")
	builder.WriteString("  }\n")
	builder.WriteString("}\n")
	return builder.String()
}

func renderMatchPauseRules(tableName, wireGuardInterface string, policies []apigateway.ControllerServiceAccessPolicy) (string, bool) {
	pauseTable := tableName + "_pause"
	var rules strings.Builder
	// Adding an existing table is idempotent. Delete and recreate in the same
	// transaction to replace the policy on older supported kernels too.
	fmt.Fprintf(&rules, "add table bridge %[1]s\ndelete table bridge %[1]s\nadd table inet %[1]s\ndelete table inet %[1]s\n", pauseTable)
	var destinations []string
	for _, policy := range policies {
		ip := net.ParseIP(policy.ServiceIP).To4()
		if ip == nil {
			continue
		}
		if policy.MatchPaused || policy.MatchFinished || policy.PlayDeferred {
			destinations = append(destinations, ip.String())
		}
	}
	if len(destinations) == 0 {
		return rules.String(), false
	}

	// The bridge hook covers traffic switched between containers. Routed
	// traffic has its own guard so it cannot use the Docker gateway to bypass
	// the bridge hook. Host-originated probes remain available.
	fmt.Fprintf(&rules, "table bridge %s {\n  chain forward {\n    type filter hook forward priority -5; policy accept;\n", pauseTable)
	fmt.Fprintf(&rules, "    ip daddr { %s } drop\n  }\n}\n", strings.Join(destinations, ", "))
	fmt.Fprintf(&rules, "table inet %s {\n  chain forward {\n    type filter hook forward priority -5; policy accept;\n", pauseTable)
	if wireGuardInterface != "" {
		for _, policy := range policies {
			ip := net.ParseIP(policy.ServiceIP).To4()
			if (!policy.MatchPaused && !policy.MatchFinished && !policy.PlayDeferred) || ip == nil {
				continue
			}
			var organizers []string
			for _, addr := range policy.AllowedPeerAddresses {
				if peer := net.ParseIP(addr).To4(); peer != nil {
					organizers = append(organizers, peer.String())
				}
			}
			if len(organizers) > 0 {
				fmt.Fprintf(&rules, "    iifname %q ip saddr { %s } ip daddr %s accept\n", wireGuardInterface, strings.Join(organizers, ", "), ip)
			}
		}
	}
	fmt.Fprintf(&rules, "    ip daddr { %s } drop\n  }\n}\n", strings.Join(destinations, ", "))
	return rules.String(), true
}

func appendOpenServiceSSHAccessRules(builder *strings.Builder, interfaceName string, policies []apigateway.ControllerServiceAccessPolicy) {
	for _, policy := range policies {
		if policy.NetworkClosed || !isValidIP(policy.ServiceIP) || policy.SSHPort <= 0 {
			continue
		}
		if policy.SSHUnlocked {
			var validPeers []string
			for _, peer := range policy.AllowedPeerAddresses {
				if isValidIP(peer) {
					validPeers = append(validPeers, peer)
				}
			}
			if len(validPeers) > 0 {
				if interfaceName != "" {
					fmt.Fprintf(builder, "    iifname \"%s\" ip daddr %s tcp dport %d ip saddr { %s } accept\n", interfaceName, policy.ServiceIP, policy.SSHPort, strings.Join(validPeers, ", "))
				}
				fmt.Fprintf(builder, "    ip daddr %s tcp dport %d ip saddr { %s } accept\n", policy.ServiceIP, policy.SSHPort, strings.Join(validPeers, ", "))
			}
		}
		if interfaceName != "" {
			fmt.Fprintf(builder, "    iifname \"%s\" ip daddr %s tcp dport %d drop\n", interfaceName, policy.ServiceIP, policy.SSHPort)
		}
		fmt.Fprintf(builder, "    ip daddr %s tcp dport %d drop\n", policy.ServiceIP, policy.SSHPort)
	}
}

// appendClosedNetworkRules emits organizer allowlist + participant WG drop for
// closed service IPs (maintenance, match not started, deferred play_from_tick).
// Host-local traffic is not dropped here (see output chain / non-WG paths).
func appendClosedNetworkRules(builder *strings.Builder, interfaceName string, policies []apigateway.ControllerServiceAccessPolicy) {
	wgIF := strings.TrimSpace(interfaceName)
	for _, policy := range policies {
		if !policy.NetworkClosed || !isValidIP(policy.ServiceIP) {
			continue
		}
		var validPeers []string
		for _, peer := range policy.AllowedPeerAddresses {
			if isValidIP(peer) {
				validPeers = append(validPeers, peer)
			}
		}
		if len(validPeers) > 0 {
			// Full access for admin WireGuard (service + SSH + any probe ports).
			builder.WriteString(fmt.Sprintf("    ip saddr { %s } ip daddr %s accept\n", strings.Join(validPeers, ", "), policy.ServiceIP))
		}
		if wgIF != "" {
			// Block remaining WireGuard peers only; host nc does not use this iif.
			builder.WriteString(fmt.Sprintf("    iifname \"%s\" ip daddr %s drop\n", wgIF, policy.ServiceIP))
		} else {
			builder.WriteString(fmt.Sprintf("    ip daddr %s drop\n", policy.ServiceIP))
		}
	}
}

// appendControllerAccessPolicyRules writes per-service accept/drop rules for open
// (not NetworkClosed) services. chainKind is "forward" (with optional iif/oif)
// or "output".
func appendControllerAccessPolicyRules(
	builder *strings.Builder,
	interfaceName, internetInterface string,
	policies []apigateway.ControllerServiceAccessPolicy,
	chainKind string,
) {
	for _, policy := range policies {
		if !isValidIP(policy.ServiceIP) {
			continue
		}
		if !policy.EgressEnabled && strings.TrimSpace(internetInterface) != "" && chainKind == "forward" {
			builder.WriteString(fmt.Sprintf("    oifname \"%s\" ip saddr %s drop\n", internetInterface, policy.ServiceIP))
		}
		if policy.NetworkClosed {
			continue
		}
		ingressPrefix := ""
		egressPrefix := ""
		if chainKind == "forward" && strings.TrimSpace(interfaceName) != "" {
			ingressPrefix = fmt.Sprintf("iifname \"%s\" ", interfaceName)
			egressPrefix = fmt.Sprintf("oifname \"%s\" ", interfaceName)
		}
		if policy.ServicePort > 0 {
			if ingressPrefix != "" {
				builder.WriteString(fmt.Sprintf("    %sip daddr %s tcp dport %d accept\n", ingressPrefix, policy.ServiceIP, policy.ServicePort))
			}
			builder.WriteString(fmt.Sprintf("    ip daddr %s tcp dport %d accept\n", policy.ServiceIP, policy.ServicePort))
			if chainKind == "forward" {
				if egressPrefix != "" {
					builder.WriteString(fmt.Sprintf("    %sip saddr %s tcp sport %d accept\n", egressPrefix, policy.ServiceIP, policy.ServicePort))
				}
				// No interface-agnostic source-port accept here: reply traffic is
				// already covered by the global `ct state established,related accept`
				// at the top of the forward chain. A bare `ip saddr serviceIP tcp
				// sport servicePort accept` would only add NEW packets the service
				// originates itself (e.g. to another team's SSH port with a spoofed
				// source port), bypassing the SSH drop below, so it is omitted.
				if egressPrefix != "" {
					builder.WriteString(fmt.Sprintf("    %sip saddr %s ct state established,related accept\n", egressPrefix, policy.ServiceIP))
				}
			}
		}
		// Residual default-deny blocks other WireGuard destinations on this IP.
		// Scope to the WG interface so host-local operator probes (OUTPUT / no
		// iif) keep working.
		if chainKind == "forward" && ingressPrefix != "" {
			builder.WriteString(fmt.Sprintf("    %sip daddr %s drop\n", ingressPrefix, policy.ServiceIP))
		}
	}
}

func controllerAccessRevision(rules string) string {
	digest := sha256.Sum256([]byte(rules))
	return hex.EncodeToString(digest[:8])
}

func sanitizeControllerNftTableName(value string) string {
	trimmed := strings.TrimSpace(strings.ToLower(value))
	if trimmed == "" {
		return "adplatform_service_access"
	}
	var builder strings.Builder
	for _, r := range trimmed {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '_':
			builder.WriteRune(r)
		}
	}
	if builder.Len() == 0 {
		return "adplatform_service_access"
	}
	return builder.String()
}

func writeControllerAccessArtifacts(paths controllerAccessArtifactPaths, rules string, status apigateway.ControllerAccessStatus) error {
	if err := ensureControllerParentDir(paths.rulesPath); err != nil {
		return err
	}
	if err := ensureControllerParentDir(paths.statusPath); err != nil {
		return err
	}
	if err := os.WriteFile(paths.rulesPath, []byte(rules), 0o600); err != nil {
		return err
	}
	return writeControllerAccessStatus(paths.statusPath, status)
}

func writeControllerAccessStatus(path string, status apigateway.ControllerAccessStatus) error {
	if strings.TrimSpace(path) == "" {
		return nil
	}
	payload, err := marshalAccessStatus(status, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, payload, 0o600) // #nosec G703 -- caller resolves configured artifact path.
}

func ensureControllerParentDir(path string) error {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return nil
	}
	return os.MkdirAll(filepath.Dir(trimmed), 0o750)
}
