package main

import (
	"context"
	"crypto/ecdh"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unicode"

	"adplatform/internal/platform/config"
	"adplatform/internal/platform/httpapi"
	"adplatform/internal/services/apigateway"
)

type wireGuardGatewayServer struct {
	adminToken string
	store      apigateway.Store
	applier    wireGuardApplier
	metrics    *wireGuardGatewayMetrics
	now        func() time.Time

	mu         sync.Mutex
	applyMu    sync.Mutex
	lastStatus apigateway.WireGuardGatewayStatus
}

type wireGuardApplier interface {
	Status() apigateway.WireGuardGatewayStatus
	Apply(context.Context, wireGuardGatewaySnapshot) (apigateway.WireGuardGatewayStatus, error)
	Teardown(ctx context.Context) error
}

type wireGuardGatewaySnapshot struct {
	GeneratedAt  string
	Revision     string
	Config       string
	ServerIP     string
	Peers        []apigateway.WireGuardGatewayPeer
	PeersTotal   int
	PeersActive  int
	PeersRevoked int
	MatchPaused  bool
}

type wireGuardServerSettings struct {
	PrivateKey string
	Address    string
	ListenPort int
}

type artifactPaths struct {
	configPath string
	peersPath  string
	rulesPath  string
	statusPath string
}

type wireGuardPeerManifest struct {
	Revision    string                            `json:"revision"`
	GeneratedAt string                            `json:"generated_at"`
	Peers       []apigateway.WireGuardGatewayPeer `json:"peers"`
}

type dryRunWireGuardApplier struct{}

type fileWireGuardApplier struct {
	mode  string
	paths artifactPaths
}

type hostWireGuardApplier struct {
	fileWireGuardApplier
	interfaceName   string
	wgBinary        string
	firewallBackend string
	nftBinary       string
	nftTable        string
	applyTimeout    time.Duration
	runner          commandRunner
}

type commandRunner interface {
	Run(ctx context.Context, binary string, args ...string) error
}

type execCommandRunner struct{}

func newWireGuardGatewayServer(adminToken string, store apigateway.Store, applier wireGuardApplier) *wireGuardGatewayServer {
	return &wireGuardGatewayServer{
		adminToken: strings.TrimSpace(adminToken),
		store:      store,
		applier:    applier,
		metrics:    newWireGuardGatewayMetrics(),
		now:        time.Now,
		lastStatus: applier.Status(),
	}
}

func (s *wireGuardGatewayServer) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /internal/v1/wireguard/status", s.handleStatus)
	mux.HandleFunc("POST /internal/v1/wireguard/reconcile", s.handleReconcile)
	mux.HandleFunc("POST /internal/v1/wireguard/teardown", s.handleTeardown)
}

func (s *wireGuardGatewayServer) handleStatus(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdminAuth(w, r) {
		return
	}
	started := time.Now()
	status := s.status()
	s.metrics.recordOperation(wireGuardOperationStatus, time.Since(started), false)
	writeData(w, http.StatusOK, status)
}

func (s *wireGuardGatewayServer) handleReconcile(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdminAuth(w, r) {
		return
	}
	s.applyMu.Lock()
	defer s.applyMu.Unlock()
	started := time.Now()

	peers, err := s.store.ListWireGuardGatewayPeers(r.Context())
	if err != nil {
		s.metrics.recordReconcile(time.Since(started), 0, true)
		writeProblem(w, http.StatusInternalServerError, "Internal state unavailable", err.Error())
		return
	}

	paused, err := s.store.IsMatchPaused(r.Context())
	if err != nil {
		log.Printf("warning: could not determine match pause state, assuming paused: %v", err)
		paused = true
	}

	snapshot, err := buildWireGuardGatewaySnapshot(peers, s.now())
	if err != nil {
		s.metrics.recordReconcile(time.Since(started), len(peers), true)
		writeProblem(w, http.StatusInternalServerError, "WireGuard server configuration invalid", err.Error())
		return
	}
	snapshot.MatchPaused = paused

	status, err := s.applier.Apply(r.Context(), snapshot)
	if err != nil {
		s.rememberStatus(status)
		s.metrics.recordReconcile(time.Since(started), snapshot.PeersTotal, true)
		writeProblem(w, http.StatusInternalServerError, "WireGuard reconcile failed", status.LastError)
		return
	}

	s.rememberStatus(status)
	s.metrics.recordReconcile(time.Since(started), snapshot.PeersTotal, false)
	writeData(w, http.StatusOK, status)
}

func (s *wireGuardGatewayServer) handleTeardown(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdminAuth(w, r) {
		return
	}
	s.applyMu.Lock()
	defer s.applyMu.Unlock()
	started := time.Now()
	if err := s.applier.Teardown(r.Context()); err != nil {
		s.metrics.recordOperation(wireGuardOperationTeardown, time.Since(started), true)
		writeProblem(w, http.StatusInternalServerError, "WireGuard teardown failed", err.Error())
		return
	}
	s.metrics.recordOperation(wireGuardOperationTeardown, time.Since(started), false)
	w.WriteHeader(http.StatusNoContent)
}

func (s *wireGuardGatewayServer) requireAdminAuth(w http.ResponseWriter, r *http.Request) bool {
	token, ok := httpapi.BearerToken(r)
	if !ok || subtle.ConstantTimeCompare([]byte(token), []byte(s.adminToken)) != 1 {
		writeProblem(w, http.StatusForbidden, "Forbidden", "please authenticate before accessing wireguard gateway endpoints.")
		return false
	}
	return true
}

func writeData(w http.ResponseWriter, statusCode int, value any) {
	httpapi.WriteJSON(w, statusCode, value)
}

func writeProblem(w http.ResponseWriter, statusCode int, title, detail string) {
	httpapi.WriteProblem(w, statusCode, httpapi.ProblemDetails{
		Title:  title,
		Status: statusCode,
		Detail: detail,
	})
}

func (s *wireGuardGatewayServer) rememberStatus(status apigateway.WireGuardGatewayStatus) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lastStatus = status
}

func (s *wireGuardGatewayServer) status() apigateway.WireGuardGatewayStatus {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.lastStatus
}

func newWireGuardApplier() wireGuardApplier {
	paths := artifactPaths{
		configPath: strings.TrimSpace(config.String("WIREGUARD_GATEWAY_CONFIG_PATH", ".runtime/wireguard/wg0.conf")),
		peersPath:  strings.TrimSpace(config.String("WIREGUARD_GATEWAY_PEERS_PATH", ".runtime/wireguard/peers.json")),
		rulesPath:  strings.TrimSpace(config.String("WIREGUARD_GATEWAY_RULES_PATH", ".runtime/wireguard/nftables.conf")),
		statusPath: strings.TrimSpace(config.String("WIREGUARD_GATEWAY_STATUS_PATH", ".runtime/wireguard/status.json")),
	}

	switch strings.ToLower(config.String("WIREGUARD_GATEWAY_MODE", "dry-run")) {
	case "files":
		return &fileWireGuardApplier{mode: "files", paths: paths}
	case "host":
		return &hostWireGuardApplier{
			fileWireGuardApplier: fileWireGuardApplier{mode: "host", paths: paths},
			interfaceName:        strings.TrimSpace(config.String("WIREGUARD_GATEWAY_INTERFACE", "wg0")),
			wgBinary:             strings.TrimSpace(config.String("WIREGUARD_GATEWAY_WG_BIN", "wg")),
			firewallBackend:      strings.ToLower(strings.TrimSpace(config.String("WIREGUARD_GATEWAY_FIREWALL_BACKEND", "nftables"))),
			nftBinary:            strings.TrimSpace(config.String("WIREGUARD_GATEWAY_NFT_BIN", "nft")),
			nftTable:             sanitizeNftTableName(config.String("WIREGUARD_GATEWAY_FIREWALL_TABLE", "adplatform_wireguard")),
			applyTimeout:         config.Duration("WIREGUARD_GATEWAY_APPLY_TIMEOUT", 10*time.Second),
			runner:               execCommandRunner{},
		}
	default:
		return dryRunWireGuardApplier{}
	}
}

func (dryRunWireGuardApplier) Status() apigateway.WireGuardGatewayStatus {
	return apigateway.WireGuardGatewayStatus{State: "idle", Mode: "dry-run"}
}

func (dryRunWireGuardApplier) Apply(_ context.Context, snapshot wireGuardGatewaySnapshot) (apigateway.WireGuardGatewayStatus, error) {
	return apigateway.WireGuardGatewayStatus{
		State:        "applied",
		Mode:         "dry-run",
		PeersTotal:   snapshot.PeersTotal,
		PeersActive:  snapshot.PeersActive,
		PeersRevoked: snapshot.PeersRevoked,
		Revision:     snapshot.Revision,
		AppliedAt:    snapshot.GeneratedAt,
	}, nil
}

func (dryRunWireGuardApplier) Teardown(_ context.Context) error {
	return nil
}

func (a *fileWireGuardApplier) Status() apigateway.WireGuardGatewayStatus {
	status := a.baseStatus(wireGuardGatewaySnapshot{})
	status.State = "idle"
	if strings.TrimSpace(a.paths.statusPath) == "" {
		return status
	}
	payload, err := os.ReadFile(a.paths.statusPath)
	if err != nil {
		return status
	}
	if err := json.Unmarshal(payload, &status); err != nil {
		status.LastError = err.Error()
		status.State = "error"
	}
	if status.Mode == "" {
		status.Mode = a.mode
	}
	if status.ConfigPath == "" {
		status.ConfigPath = a.paths.configPath
	}
	if status.PeersPath == "" {
		status.PeersPath = a.paths.peersPath
	}
	if status.RulesPath == "" {
		status.RulesPath = a.paths.rulesPath
	}
	if status.StatusPath == "" {
		status.StatusPath = a.paths.statusPath
	}
	return status
}

func (a *fileWireGuardApplier) Apply(_ context.Context, snapshot wireGuardGatewaySnapshot) (apigateway.WireGuardGatewayStatus, error) {
	status := a.baseStatus(snapshot)
	if err := writeWireGuardArtifacts(a.paths, snapshot, status, ""); err != nil {
		status.State = "error"
		status.LastError = err.Error()
		_ = writeStatusArtifact(a.paths.statusPath, status)
		return status, err
	}
	return status, nil
}

func (a *fileWireGuardApplier) Teardown(_ context.Context) error {
	if strings.TrimSpace(a.paths.configPath) != "" {
		_ = os.Remove(a.paths.configPath) // #nosec G703 -- configured artifact path, not request input.
	}
	if strings.TrimSpace(a.paths.rulesPath) != "" {
		_ = os.Remove(a.paths.rulesPath) // #nosec G703 -- configured artifact path, not request input.
	}
	if strings.TrimSpace(a.paths.peersPath) != "" {
		_ = os.Remove(a.paths.peersPath) // #nosec G703 -- configured artifact path, not request input.
	}
	if strings.TrimSpace(a.paths.statusPath) != "" {
		_ = os.Remove(a.paths.statusPath) // #nosec G703 -- configured artifact path, not request input.
	}
	return nil
}

func (a *fileWireGuardApplier) baseStatus(snapshot wireGuardGatewaySnapshot) apigateway.WireGuardGatewayStatus {
	return apigateway.WireGuardGatewayStatus{
		State:        "applied",
		Mode:         a.mode,
		ConfigPath:   a.paths.configPath,
		PeersPath:    a.paths.peersPath,
		RulesPath:    a.paths.rulesPath,
		StatusPath:   a.paths.statusPath,
		PeersTotal:   snapshot.PeersTotal,
		PeersActive:  snapshot.PeersActive,
		PeersRevoked: snapshot.PeersRevoked,
		Revision:     snapshot.Revision,
		AppliedAt:    snapshot.GeneratedAt,
	}
}

func (a *hostWireGuardApplier) Status() apigateway.WireGuardGatewayStatus {
	status := a.fileWireGuardApplier.Status()
	status.Mode = "host"
	status.Interface = a.interfaceName
	status.FirewallBackend = a.firewallBackend
	return status
}

func (a *hostWireGuardApplier) Apply(ctx context.Context, snapshot wireGuardGatewaySnapshot) (apigateway.WireGuardGatewayStatus, error) {
	status := a.baseStatus(snapshot)
	status.Mode = "host"
	status.Interface = a.interfaceName
	status.FirewallBackend = a.firewallBackend

	rulesBody := ""
	if a.firewallBackend == "nftables" {
		peers := snapshot.Peers
		if snapshot.MatchPaused {
			var organizers []apigateway.WireGuardGatewayPeer
			for _, p := range peers {
				if p.TeamName == "Organizer" {
					organizers = append(organizers, p)
				}
			}
			peers = organizers
		}
		rulesBody = renderNftablesRules(a.nftTable, a.interfaceName, snapshot.ServerIP, activeWireGuardPeers(peers))
	}
	if err := writeWireGuardArtifacts(a.paths, snapshot, status, rulesBody); err != nil {
		status.State = "error"
		status.LastError = err.Error()
		_ = writeStatusArtifact(a.paths.statusPath, status)
		return status, err
	}

	applyCtx, cancel := context.WithTimeout(ctx, a.applyTimeout)
	defer cancel()

	if err := a.runner.Run(applyCtx, a.wgBinary, "syncconf", a.interfaceName, a.paths.configPath); err != nil {
		status.State = "error"
		status.LastError = err.Error()
		_ = writeStatusArtifact(a.paths.statusPath, status)
		return status, err
	}
	if a.firewallBackend == "nftables" {
		_ = a.runner.Run(applyCtx, a.nftBinary, "delete", "table", "inet", a.nftTable)
		if err := a.runner.Run(applyCtx, a.nftBinary, "-f", a.paths.rulesPath); err != nil {
			status.State = "error"
			status.LastError = err.Error()
			_ = writeStatusArtifact(a.paths.statusPath, status)
			return status, err
		}
	}

	if err := writeStatusArtifact(a.paths.statusPath, status); err != nil {
		status.State = "error"
		status.LastError = err.Error()
		return status, err
	}
	return status, nil
}

func (a *hostWireGuardApplier) Teardown(ctx context.Context) error {
	applyCtx, cancel := context.WithTimeout(ctx, a.applyTimeout)
	defer cancel()

	_ = a.runner.Run(applyCtx, "ip", "link", "delete", a.interfaceName)
	if a.firewallBackend == "nftables" {
		_ = a.runner.Run(applyCtx, a.nftBinary, "delete", "table", "inet", a.nftTable)
	}

	return a.fileWireGuardApplier.Teardown(ctx)
}

func (execCommandRunner) Run(ctx context.Context, binary string, args ...string) error {
	cmd := exec.CommandContext(ctx, binary, args...) // #nosec G204,G702 -- binary is host operator configuration; args are passed without shell expansion.
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s %s failed: %w: %s", binary, strings.Join(args, " "), err, strings.TrimSpace(string(output)))
	}
	return nil
}

func writeWireGuardArtifacts(paths artifactPaths, snapshot wireGuardGatewaySnapshot, status apigateway.WireGuardGatewayStatus, rulesBody string) error {
	if err := ensureParentDir(paths.configPath); err != nil {
		return err
	}
	if err := ensureParentDir(paths.peersPath); err != nil {
		return err
	}
	if err := ensureParentDir(paths.rulesPath); err != nil {
		return err
	}
	if err := ensureParentDir(paths.statusPath); err != nil {
		return err
	}
	if err := os.WriteFile(paths.configPath, []byte(snapshot.Config), 0o600); err != nil {
		return err
	}

	manifest, err := json.MarshalIndent(wireGuardPeerManifest{Revision: snapshot.Revision, GeneratedAt: snapshot.GeneratedAt, Peers: snapshot.Peers}, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(paths.peersPath, manifest, 0o600); err != nil {
		return err
	}
	if strings.TrimSpace(rulesBody) != "" && strings.TrimSpace(paths.rulesPath) != "" {
		if err := os.WriteFile(paths.rulesPath, []byte(rulesBody), 0o600); err != nil {
			return err
		}
	}
	return writeStatusArtifact(paths.statusPath, status)
}

func writeStatusArtifact(path string, status apigateway.WireGuardGatewayStatus) error {
	if strings.TrimSpace(path) == "" {
		return nil
	}
	payload, err := json.MarshalIndent(status, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, payload, 0o600) // #nosec G703 -- caller resolves configured artifact path.
}

func ensureParentDir(path string) error {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return nil
	}
	return os.MkdirAll(filepath.Dir(trimmed), 0o750)
}

func buildWireGuardGatewaySnapshot(peers []apigateway.WireGuardGatewayPeer, now time.Time) (wireGuardGatewaySnapshot, error) {
	settings, err := resolveWireGuardServerSettings()
	if err != nil {
		return wireGuardGatewaySnapshot{}, err
	}
	activePeers := activeWireGuardPeers(peers)
	configBody := renderWireGuardGatewayConfig(settings, activePeers)
	revision := wireGuardGatewayRevision(configBody)
	return wireGuardGatewaySnapshot{
		GeneratedAt:  now.UTC().Format(time.RFC3339),
		Revision:     revision,
		Config:       configBody,
		ServerIP:     wireGuardServerIP(settings.Address),
		Peers:        peers,
		PeersTotal:   len(peers),
		PeersActive:  len(activePeers),
		PeersRevoked: len(peers) - len(activePeers),
	}, nil
}

func activeWireGuardPeers(peers []apigateway.WireGuardGatewayPeer) []apigateway.WireGuardGatewayPeer {
	active := make([]apigateway.WireGuardGatewayPeer, 0, len(peers))
	for _, peer := range peers {
		if strings.EqualFold(strings.TrimSpace(peer.Status), "revoked") {
			continue
		}
		if !isValidWireGuardPublicKey(peer.ClientPublicKey) {
			log.Printf("warning: skipping peer %s: invalid ClientPublicKey format", peer.WireGuardPeer)
			continue
		}
		if peer.PresharedKey != "" && !isValidWireGuardPublicKey(peer.PresharedKey) {
			log.Printf("warning: skipping peer %s: invalid PresharedKey format", peer.WireGuardPeer)
			continue
		}
		if !isValidIP(peer.Address) {
			log.Printf("warning: skipping peer %s: invalid Address format %q", peer.WireGuardPeer, peer.Address)
			continue
		}
		active = append(active, peer)
	}
	return active
}

func isValidWireGuardPublicKey(key string) bool {
	decoded, err := base64.StdEncoding.DecodeString(strings.TrimSpace(key))
	if err != nil {
		return false
	}
	return len(decoded) == 32
}

func isValidIP(ip string) bool {
	parsed := net.ParseIP(strings.TrimSpace(ip))
	return parsed != nil && parsed.To4() != nil
}

// sanitizeConfigComment strips control characters (notably CR/LF) from values
// rendered into wg0.conf comment lines. Without this, a newline in an
// attacker-controlled display name or team name could break out of the comment
// and inject [Peer]/AllowedIPs directives that wg syncconf would apply.
func sanitizeConfigComment(value string) string {
	cleaned := strings.Map(func(r rune) rune {
		if r == '\n' || r == '\r' || unicode.IsControl(r) {
			return ' '
		}
		return r
	}, value)
	cleaned = strings.TrimSpace(cleaned)
	if len(cleaned) > 96 {
		cleaned = cleaned[:96]
	}
	return cleaned
}

func renderWireGuardGatewayConfig(settings wireGuardServerSettings, peers []apigateway.WireGuardGatewayPeer) string {
	var builder strings.Builder
	builder.WriteString("# AD Platform WireGuard gateway\n")
	builder.WriteString(fmt.Sprintf("# Active peers: %d\n", len(peers)))
	if strings.TrimSpace(settings.Address) != "" {
		builder.WriteString(fmt.Sprintf("# Interface address managed outside wg syncconf: %s\n", settings.Address))
	}
	builder.WriteString("[Interface]\n")
	builder.WriteString(fmt.Sprintf("PrivateKey = %s\n", settings.PrivateKey))
	builder.WriteString(fmt.Sprintf("ListenPort = %d\n", settings.ListenPort))
	for _, peer := range peers {
		builder.WriteString("\n[Peer]\n")
		builder.WriteString(fmt.Sprintf("# %s | %s | %s\n", sanitizeConfigComment(peer.WireGuardPeer), sanitizeConfigComment(peer.DisplayName), sanitizeConfigComment(peer.TeamName)))
		builder.WriteString(fmt.Sprintf("PublicKey = %s\n", peer.ClientPublicKey))
		builder.WriteString(fmt.Sprintf("PresharedKey = %s\n", peer.PresharedKey))
		builder.WriteString(fmt.Sprintf("AllowedIPs = %s/32\n", peer.Address))
	}
	return builder.String()
}

func renderNftablesRules(tableName, interfaceName, serverIP string, peers []apigateway.WireGuardGatewayPeer) string {
	return renderNftablesRulesWithForwardCIDRs(tableName, interfaceName, serverIP, peers, wireGuardForwardAllowedCIDRs())
}

// renderNftablesRulesWithForwardCIDRs builds the WG nftables policy. Forward
// traffic from active peers is restricted to forwardCIDRs so participants cannot
// reach host-routable infra (e.g. docker-socket-proxy) by widening client AllowedIPs.
func renderNftablesRulesWithForwardCIDRs(tableName, interfaceName, serverIP string, peers []apigateway.WireGuardGatewayPeer, forwardCIDRs []string) string {
	var builder strings.Builder
	builder.WriteString(fmt.Sprintf("table inet %s {\n", tableName))
	builder.WriteString("  set active_peers {\n")
	builder.WriteString("    type ipv4_addr\n")
	if len(peers) > 0 {
		builder.WriteString("    elements = { ")
		for index, peer := range peers {
			if index > 0 {
				builder.WriteString(", ")
			}
			builder.WriteString(peer.Address)
		}
		builder.WriteString(" }\n")
	}
	builder.WriteString("  }\n\n")
	builder.WriteString("  chain input {\n")
	builder.WriteString("    type filter hook input priority filter;\n")
	builder.WriteString("    policy accept;\n")
	if strings.TrimSpace(serverIP) != "" {
		builder.WriteString(fmt.Sprintf("    iifname \"%s\" ip saddr @active_peers ip daddr %s icmp type echo-request accept\n", interfaceName, serverIP))
		builder.WriteString(fmt.Sprintf("    iifname \"%s\" ip saddr @active_peers ip daddr %s tcp dport { 80, 443 } accept\n", interfaceName, serverIP))
		builder.WriteString(fmt.Sprintf("    iifname \"%s\" ip saddr @active_peers drop\n", interfaceName))
	}
	builder.WriteString(fmt.Sprintf("    iifname \"%s\" ip saddr != @active_peers drop\n", interfaceName))
	builder.WriteString("  }\n\n")
	builder.WriteString("  chain forward {\n")
	builder.WriteString("    type filter hook forward priority filter;\n")
	builder.WriteString("    policy accept;\n")
	builder.WriteString("    ct state established,related accept\n")
	builder.WriteString(fmt.Sprintf("    iifname \"%s\" ip saddr != @active_peers drop\n", interfaceName))
	if len(forwardCIDRs) > 0 {
		builder.WriteString(fmt.Sprintf("    iifname \"%s\" ip saddr @active_peers ip daddr { %s } accept\n", interfaceName, strings.Join(forwardCIDRs, ", ")))
	}
	// Default-deny remaining WireGuard ingress on the forward path (blocks
	// docker-control, host LANs, and any destination not explicitly allowlisted).
	builder.WriteString(fmt.Sprintf("    iifname \"%s\" drop\n", interfaceName))
	builder.WriteString("  }\n")
	builder.WriteString("}\n")
	return builder.String()
}

// wireGuardForwardAllowedCIDRs is the destination allowlist for WG-forwarded
// traffic. Prefer WIREGUARD_FORWARD_ALLOWED_CIDRS; otherwise reuse the client
// AllowedIPs list (game + VPN nets), never host docker-control bridges.
func wireGuardForwardAllowedCIDRs() []string {
	raw := strings.TrimSpace(config.String("WIREGUARD_FORWARD_ALLOWED_CIDRS", ""))
	if raw == "" {
		raw = strings.TrimSpace(config.String("WIREGUARD_SERVER_ALLOWED_IPS", "10.70.0.0/16,10.80.0.0/16"))
	}
	return parseIPv4CIDRList(raw)
}

func parseIPv4CIDRList(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	cidrs := make([]string, 0, len(parts))
	seen := make(map[string]struct{}, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed == "" {
			continue
		}
		// Accept bare IPv4 as /32 for convenience.
		if ip := net.ParseIP(trimmed); ip != nil && ip.To4() != nil && !strings.Contains(trimmed, "/") {
			trimmed = trimmed + "/32"
		}
		if _, network, err := net.ParseCIDR(trimmed); err != nil || network == nil || network.IP.To4() == nil {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		cidrs = append(cidrs, trimmed)
	}
	return cidrs
}

func wireGuardServerIP(addressCIDR string) string {
	trimmed := strings.TrimSpace(addressCIDR)
	if trimmed == "" {
		return ""
	}
	host, _, found := strings.Cut(trimmed, "/")
	if found {
		return strings.TrimSpace(host)
	}
	return trimmed
}

func sanitizeNftTableName(value string) string {
	trimmed := strings.TrimSpace(strings.ToLower(value))
	if trimmed == "" {
		return "adplatform_wireguard"
	}
	var builder strings.Builder
	for _, r := range trimmed {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '_':
			builder.WriteRune(r)
		}
	}
	if builder.Len() == 0 {
		return "adplatform_wireguard"
	}
	return builder.String()
}

func wireGuardGatewayRevision(configBody string) string {
	digest := sha256.Sum256([]byte(configBody))
	return hex.EncodeToString(digest[:8])
}

func resolveWireGuardServerSettings() (wireGuardServerSettings, error) {
	privateKey := strings.TrimSpace(config.String("WIREGUARD_SERVER_PRIVATE_KEY", ""))
	if privateKey == "" {
		if !allowDevWireGuardGatewayKeyFallback() {
			return wireGuardServerSettings{}, fmt.Errorf("WIREGUARD_SERVER_PRIVATE_KEY must be set outside dry-run memory mode")
		}
		privateKey, _ = defaultWireGuardGatewayKeypair()
	} else if err := validateWireGuardGatewayPrivateKey(privateKey); err != nil {
		return wireGuardServerSettings{}, err
	}
	return wireGuardServerSettings{
		PrivateKey: privateKey,
		Address:    strings.TrimSpace(config.String("WIREGUARD_SERVER_ADDRESS", "10.70.0.1/16")),
		ListenPort: config.Int("WIREGUARD_SERVER_LISTEN_PORT", 51820),
	}, nil
}

func allowDevWireGuardGatewayKeyFallback() bool {
	apiBackend := strings.ToLower(strings.TrimSpace(config.String("API_GATEWAY_STATE_BACKEND", "memory")))
	wgBackend := strings.ToLower(strings.TrimSpace(config.String("WIREGUARD_GATEWAY_STATE_BACKEND", apiBackend)))
	mode := strings.ToLower(strings.TrimSpace(config.String("WIREGUARD_GATEWAY_MODE", "dry-run")))
	return apiBackend != "postgres" && wgBackend != "postgres" && mode == "dry-run"
}

func validateWireGuardGatewayPrivateKey(privateKey string) error {
	decoded, err := base64.StdEncoding.DecodeString(privateKey)
	if err != nil {
		return fmt.Errorf("WIREGUARD_SERVER_PRIVATE_KEY is invalid: %w", err)
	}
	if len(decoded) != 32 {
		return fmt.Errorf("WIREGUARD_SERVER_PRIVATE_KEY must decode to 32 bytes")
	}
	if _, err := ecdh.X25519().NewPrivateKey(decoded); err != nil {
		return fmt.Errorf("WIREGUARD_SERVER_PRIVATE_KEY is invalid: %w", err)
	}
	return nil
}

func defaultWireGuardGatewayKeypair() (string, string) {
	curve := ecdh.X25519()
	seed := sha256.Sum256([]byte("adplatform-dev-wireguard-server"))
	privateKey, err := curve.NewPrivateKey(seed[:])
	if err != nil {
		encodedSeed := base64.StdEncoding.EncodeToString(seed[:])
		return encodedSeed, encodedSeed
	}
	return base64.StdEncoding.EncodeToString(privateKey.Bytes()), base64.StdEncoding.EncodeToString(privateKey.PublicKey().Bytes())
}
