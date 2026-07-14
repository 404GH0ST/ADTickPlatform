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
		_ = e.runner.Run(applyCtx, e.nftBinary, "delete", "table", "inet", e.firewallTable)
		if err := e.runner.Run(applyCtx, e.nftBinary, "-f", e.paths.rulesPath); err != nil {
			status.State = "error"
			status.LastError = err.Error()
			_ = writeControllerAccessStatus(e.paths.statusPath, status)
			return status, err
		}
		// Docker traffic still traverses iptables raw/filter hooks on the host.
		// Keep those jumps and bypass rules aligned even when nftables owns the
		// service policy table itself.
		if err := e.applyDockerRawRules(applyCtx, iptablesBinary, policies); err != nil {
			status.State = "error"
			status.LastError = err.Error()
			_ = writeControllerAccessStatus(e.paths.statusPath, status)
			return status, err
		}
		if err := e.applyDockerUserRules(applyCtx, iptablesBinary, policies); err != nil {
			status.State = "error"
			status.LastError = err.Error()
			_ = writeControllerAccessStatus(e.paths.statusPath, status)
			return status, err
		}
	} else {
		// When using iptables backend, clean up any orphaned nftables table
		// to prevent it from silently overriding iptables rules.
		_ = e.runner.Run(applyCtx, e.nftBinary, "delete", "table", "inet", e.firewallTable)

		log.Printf("applying iptables rules")
		if err := e.applyDockerRawRules(applyCtx, iptablesBinary, policies); err != nil {
			status.State = "error"
			status.LastError = err.Error()
			_ = writeControllerAccessStatus(e.paths.statusPath, status)
			return status, err
		}
		if err := e.applyDockerUserRules(applyCtx, iptablesBinary, policies); err != nil {
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

func (e *hostServiceAccessExecutor) applyDockerRawRules(ctx context.Context, iptablesBinary string, policies []apigateway.ControllerServiceAccessPolicy) error {
	const chainName = "ADPLATFORM-WG-RAW"

	if err := e.ensureIptablesChain(ctx, iptablesBinary, "raw", chainName); err != nil {
		return err
	}
	if err := e.runner.Run(ctx, iptablesBinary, "-t", "raw", "-C", "PREROUTING", "-j", chainName); err != nil {
		if err := e.runner.Run(ctx, iptablesBinary, "-t", "raw", "-I", "PREROUTING", "1", "-j", chainName); err != nil {
			return err
		}
	}
	if err := e.runner.Run(ctx, iptablesBinary, "-t", "raw", "-F", chainName); err != nil {
		return err
	}
	for _, policy := range policies {
		if !isValidIP(policy.ServiceIP) {
			continue
		}
		// Always accept in raw for docker/WG path quirks. Closed participant
		// traffic is still dropped in filter; organizer peers are allowlisted.
		serviceCIDR := fmt.Sprintf("%s/32", policy.ServiceIP)
		if strings.TrimSpace(e.interfaceName) != "" {
			if err := e.runner.Run(ctx, iptablesBinary, "-t", "raw", "-A", chainName, "-i", e.interfaceName, "-d", serviceCIDR, "-j", "ACCEPT"); err != nil {
				return err
			}
			continue
		}
		if err := e.runner.Run(ctx, iptablesBinary, "-t", "raw", "-A", chainName, "-d", serviceCIDR, "-j", "ACCEPT"); err != nil {
			return err
		}
	}
	return nil
}

func (e *hostServiceAccessExecutor) applyDockerUserRules(ctx context.Context, iptablesBinary string, policies []apigateway.ControllerServiceAccessPolicy) error {
	const chainName = "ADPLATFORM-WG-SERVICES"
	const dockerUserChain = "DOCKER-USER"

	if err := e.ensureIptablesChain(ctx, iptablesBinary, "filter", dockerUserChain); err != nil {
		return err
	}
	if err := e.runner.Run(ctx, iptablesBinary, "-C", "FORWARD", "-j", dockerUserChain); err != nil {
		if err := e.runner.Run(ctx, iptablesBinary, "-I", "FORWARD", "1", "-j", dockerUserChain); err != nil {
			return err
		}
	}

	if err := e.ensureIptablesChain(ctx, iptablesBinary, "filter", chainName); err != nil {
		return err
	}
	if err := e.runner.Run(ctx, iptablesBinary, "-F", chainName); err != nil {
		return err
	}
	if err := e.runner.Run(ctx, iptablesBinary, "-C", dockerUserChain, "-j", chainName); err != nil {
		if err := e.runner.Run(ctx, iptablesBinary, "-I", dockerUserChain, "1", "-j", chainName); err != nil {
			return err
		}
	}
	if err := e.runner.Run(ctx, iptablesBinary, "-C", "FORWARD", "-j", chainName); err != nil {
		if err := e.runner.Run(ctx, iptablesBinary, "-I", "FORWARD", "1", "-j", chainName); err != nil {
			return err
		}
	}
	// Closed services first (before ESTABLISHED): organizer WG peers may probe;
	// other WG peers are dropped. Host-local paths (no WG iif) stay open for nc.
	for _, policy := range policies {
		if !policy.NetworkClosed || !isValidIP(policy.ServiceIP) {
			continue
		}
		serviceCIDR := fmt.Sprintf("%s/32", policy.ServiceIP)
		for _, peer := range policy.AllowedPeerAddresses {
			if !isValidIP(peer) {
				continue
			}
			if strings.TrimSpace(e.interfaceName) != "" {
				if err := e.runner.Run(ctx, iptablesBinary, "-A", chainName, "-i", e.interfaceName, "-s", peer, "-d", serviceCIDR, "-j", "ACCEPT"); err != nil {
					return err
				}
			}
			if err := e.runner.Run(ctx, iptablesBinary, "-A", chainName, "-s", peer, "-d", serviceCIDR, "-j", "ACCEPT"); err != nil {
				return err
			}
		}
		if strings.TrimSpace(e.interfaceName) != "" {
			if err := e.runner.Run(ctx, iptablesBinary, "-A", chainName, "-i", e.interfaceName, "-d", serviceCIDR, "-j", "DROP"); err != nil {
				return err
			}
		} else {
			// No WG interface configured: fall back to full dest drop (host path
			// still works via OUTPUT, which is outside this FORWARD chain).
			if err := e.runner.Run(ctx, iptablesBinary, "-A", chainName, "-d", serviceCIDR, "-j", "DROP"); err != nil {
				return err
			}
		}
	}
	if err := e.runner.Run(ctx, iptablesBinary, "-A", chainName, "-m", "conntrack", "--ctstate", "ESTABLISHED,RELATED", "-j", "ACCEPT"); err != nil {
		return err
	}
	for _, policy := range policies {
		if !isValidIP(policy.ServiceIP) {
			continue
		}
		serviceCIDR := fmt.Sprintf("%s/32", policy.ServiceIP)
		if !policy.EgressEnabled && strings.TrimSpace(e.internetInterface) != "" {
			if err := e.runner.Run(ctx, iptablesBinary, "-A", chainName, "-o", e.internetInterface, "-s", serviceCIDR, "-j", "DROP"); err != nil {
				return err
			}
		}
		if policy.NetworkClosed {
			continue
		}
		servicePort := fmt.Sprintf("%d", policy.ServicePort)
		sshPort := fmt.Sprintf("%d", policy.SSHPort)
		if err := e.runner.Run(ctx, iptablesBinary, "-A", chainName, "-i", e.interfaceName, "-d", serviceCIDR, "-p", "tcp", "--dport", servicePort, "-j", "ACCEPT"); err != nil {
			return err
		}
		if err := e.runner.Run(ctx, iptablesBinary, "-A", chainName, "-d", serviceCIDR, "-p", "tcp", "--dport", servicePort, "-j", "ACCEPT"); err != nil {
			return err
		}
		if err := e.runner.Run(ctx, iptablesBinary, "-A", chainName, "-o", e.interfaceName, "-s", serviceCIDR, "-p", "tcp", "--sport", servicePort, "-j", "ACCEPT"); err != nil {
			return err
		}
		if err := e.runner.Run(ctx, iptablesBinary, "-A", chainName, "-s", serviceCIDR, "-p", "tcp", "--sport", servicePort, "-j", "ACCEPT"); err != nil {
			return err
		}
		if err := e.runner.Run(ctx, iptablesBinary, "-A", chainName, "-o", e.interfaceName, "-s", serviceCIDR, "-m", "conntrack", "--ctstate", "ESTABLISHED,RELATED", "-j", "ACCEPT"); err != nil {
			return err
		}
		if policy.SSHUnlocked && len(policy.AllowedPeerAddresses) > 0 {
			for _, peer := range policy.AllowedPeerAddresses {
				if !isValidIP(peer) {
					continue
				}
				if err := e.runner.Run(ctx, iptablesBinary, "-A", chainName, "-i", e.interfaceName, "-s", peer, "-d", serviceCIDR, "-p", "tcp", "--dport", sshPort, "-j", "ACCEPT"); err != nil {
					return err
				}
				if err := e.runner.Run(ctx, iptablesBinary, "-A", chainName, "-s", peer, "-d", serviceCIDR, "-p", "tcp", "--dport", sshPort, "-j", "ACCEPT"); err != nil {
					return err
				}
			}
		}
		if err := e.runner.Run(ctx, iptablesBinary, "-A", chainName, "-i", e.interfaceName, "-d", serviceCIDR, "-p", "tcp", "--dport", sshPort, "-j", "DROP"); err != nil {
			return err
		}
		if err := e.runner.Run(ctx, iptablesBinary, "-A", chainName, "-d", serviceCIDR, "-p", "tcp", "--dport", sshPort, "-j", "DROP"); err != nil {
			return err
		}
		// Residual default-deny on the WG path for non-service ports.
		if strings.TrimSpace(e.interfaceName) != "" {
			if err := e.runner.Run(ctx, iptablesBinary, "-A", chainName, "-i", e.interfaceName, "-d", serviceCIDR, "-j", "DROP"); err != nil {
				return err
			}
		}
	}
	return nil
}

func (e *hostServiceAccessExecutor) ensureIptablesChain(ctx context.Context, iptablesBinary, tableName, chainName string) error {
	createArgs := []string{"-N", chainName}
	showArgs := []string{"-S", chainName}
	if strings.TrimSpace(tableName) != "" {
		createArgs = append([]string{"-t", tableName}, createArgs...)
		showArgs = append([]string{"-t", tableName}, showArgs...)
	}
	if err := e.runner.Run(ctx, iptablesBinary, createArgs...); err != nil {
		if err := e.runner.Run(ctx, iptablesBinary, showArgs...); err != nil {
			return err
		}
	}
	return nil
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
		if _, ok := seen[candidate]; !ok {
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
	rules := renderControllerAccessRules(sanitizeControllerNftTableName(config.String("CONTROLLER_ACCESS_FIREWALL_TABLE", "adplatform_service_access")), interfaceName, internetInterface, policies)
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
	builder.WriteString(fmt.Sprintf("table inet %s {\n", tableName))
	// Forward: participant WG → services (closed = organizer allow + WG drop).
	// Output: host-local probes (nc on the contest host) — always allowed so
	// operators can check warm redeploys without a WG client.
	builder.WriteString("  chain forward {\n")
	builder.WriteString("    type filter hook forward priority filter;\n")
	builder.WriteString("    policy accept;\n")
	appendClosedNetworkRules(&builder, interfaceName, policies)
	builder.WriteString("    ct state established,related accept\n")
	appendControllerAccessPolicyRules(&builder, interfaceName, internetInterface, policies, "forward")
	builder.WriteString("  }\n")
	builder.WriteString("  chain output {\n")
	builder.WriteString("    type filter hook output priority filter;\n")
	builder.WriteString("    policy accept;\n")
	builder.WriteString("    ct state established,related accept\n")
	appendControllerAccessPolicyRules(&builder, "", internetInterface, policies, "output")
	builder.WriteString("  }\n")
	builder.WriteString("}\n")
	return builder.String()
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
				builder.WriteString(fmt.Sprintf("    ip saddr %s tcp sport %d accept\n", policy.ServiceIP, policy.ServicePort))
				if egressPrefix != "" {
					builder.WriteString(fmt.Sprintf("    %sip saddr %s ct state established,related accept\n", egressPrefix, policy.ServiceIP))
				}
			}
		}
		if policy.SSHUnlocked && len(policy.AllowedPeerAddresses) > 0 {
			var validPeers []string
			for _, peer := range policy.AllowedPeerAddresses {
				if isValidIP(peer) {
					validPeers = append(validPeers, peer)
				}
			}
			if len(validPeers) > 0 {
				if ingressPrefix != "" {
					builder.WriteString(fmt.Sprintf("    %sip daddr %s tcp dport %d ip saddr { %s } accept\n", ingressPrefix, policy.ServiceIP, policy.SSHPort, strings.Join(validPeers, ", ")))
				}
				builder.WriteString(fmt.Sprintf("    ip daddr %s tcp dport %d ip saddr { %s } accept\n", policy.ServiceIP, policy.SSHPort, strings.Join(validPeers, ", ")))
			}
		}
		if policy.SSHPort > 0 {
			if ingressPrefix != "" {
				builder.WriteString(fmt.Sprintf("    %sip daddr %s tcp dport %d drop\n", ingressPrefix, policy.ServiceIP, policy.SSHPort))
			}
			builder.WriteString(fmt.Sprintf("    ip daddr %s tcp dport %d drop\n", policy.ServiceIP, policy.SSHPort))
		}
		// Residual default-deny for open services: after service-port accept and
		// SSH allowlist/drop, block other WireGuard destinations on this IP.
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
	payload, err := json.MarshalIndent(status, "", "  ")
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
