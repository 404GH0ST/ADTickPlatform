package main

import (
	"context"
	"crypto/ecdh"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"adplatform/internal/platform/config"
	"adplatform/internal/platform/httpapi"
	"adplatform/internal/services/apigateway"
)

type wireGuardGatewayServer struct {
	adminToken string
	store      apigateway.Store
	applier    wireGuardApplier
	now        func() time.Time

	mu         sync.Mutex
	lastStatus apigateway.WireGuardGatewayStatus
}

type successEnvelope[T any] struct {
	Status string `json:"status"`
	Data   T      `json:"data"`
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
	httpapi.WriteJSON(w, http.StatusOK, successEnvelope[apigateway.WireGuardGatewayStatus]{Status: "success", Data: s.status()})
}

func (s *wireGuardGatewayServer) handleReconcile(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdminAuth(w, r) {
		return
	}

	peers, err := s.store.ListWireGuardGatewayPeers(r.Context())
	if err != nil {
		httpapi.WriteJSON(w, http.StatusInternalServerError, httpapi.ErrorEnvelope{Status: "failed", Message: err.Error()})
		return
	}

	snapshot := buildWireGuardGatewaySnapshot(peers, s.now())
	status, err := s.applier.Apply(r.Context(), snapshot)
	if err != nil {
		s.rememberStatus(status)
		httpapi.WriteJSON(w, http.StatusInternalServerError, httpapi.ErrorEnvelope{Status: "failed", Message: status.LastError})
		return
	}

	s.rememberStatus(status)
	httpapi.WriteJSON(w, http.StatusOK, successEnvelope[apigateway.WireGuardGatewayStatus]{Status: "success", Data: status})
}

func (s *wireGuardGatewayServer) handleTeardown(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdminAuth(w, r) {
		return
	}
	if err := s.applier.Teardown(r.Context()); err != nil {
		httpapi.WriteJSON(w, http.StatusInternalServerError, httpapi.ErrorEnvelope{Status: "failed", Message: err.Error()})
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, map[string]string{"status": "success"})
}

func (s *wireGuardGatewayServer) requireAdminAuth(w http.ResponseWriter, r *http.Request) bool {
	token, ok := httpapi.BearerToken(r)
	if !ok || token != s.adminToken {
		httpapi.WriteJSON(w, http.StatusForbidden, httpapi.ErrorEnvelope{Status: "forbidden", Message: "please authenticate before accessing wireguard gateway endpoints."})
		return false
	}
	return true
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
		_ = os.Remove(a.paths.configPath)
	}
	if strings.TrimSpace(a.paths.rulesPath) != "" {
		_ = os.Remove(a.paths.rulesPath)
	}
	if strings.TrimSpace(a.paths.peersPath) != "" {
		_ = os.Remove(a.paths.peersPath)
	}
	if strings.TrimSpace(a.paths.statusPath) != "" {
		_ = os.Remove(a.paths.statusPath)
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
		rulesBody = renderNftablesRules(a.nftTable, a.interfaceName, snapshot.ServerIP, activeWireGuardPeers(snapshot.Peers))
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
	cmd := exec.CommandContext(ctx, binary, args...)
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
	return os.WriteFile(path, payload, 0o600)
}

func ensureParentDir(path string) error {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return nil
	}
	return os.MkdirAll(filepath.Dir(trimmed), 0o755)
}

func buildWireGuardGatewaySnapshot(peers []apigateway.WireGuardGatewayPeer, now time.Time) wireGuardGatewaySnapshot {
	settings := resolveWireGuardServerSettings()
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
	}
}

func activeWireGuardPeers(peers []apigateway.WireGuardGatewayPeer) []apigateway.WireGuardGatewayPeer {
	active := make([]apigateway.WireGuardGatewayPeer, 0, len(peers))
	for _, peer := range peers {
		if strings.EqualFold(strings.TrimSpace(peer.Status), "revoked") {
			continue
		}
		active = append(active, peer)
	}
	return active
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
		builder.WriteString(fmt.Sprintf("# %s | %s | %s\n", peer.WireGuardPeer, peer.DisplayName, peer.TeamName))
		builder.WriteString(fmt.Sprintf("PublicKey = %s\n", peer.ClientPublicKey))
		builder.WriteString(fmt.Sprintf("PresharedKey = %s\n", peer.PresharedKey))
		builder.WriteString(fmt.Sprintf("AllowedIPs = %s/32\n", peer.Address))
	}
	return builder.String()
}

func renderNftablesRules(tableName, interfaceName, serverIP string, peers []apigateway.WireGuardGatewayPeer) string {
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
	builder.WriteString(fmt.Sprintf("    iifname \"%s\" ip saddr != @active_peers drop\n", interfaceName))
	builder.WriteString("  }\n")
	builder.WriteString("}\n")
	return builder.String()
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

func resolveWireGuardServerSettings() wireGuardServerSettings {
	privateKey := strings.TrimSpace(config.String("WIREGUARD_SERVER_PRIVATE_KEY", ""))
	if privateKey == "" {
		privateKey, _ = defaultWireGuardGatewayKeypair()
	}
	return wireGuardServerSettings{
		PrivateKey: privateKey,
		Address:    strings.TrimSpace(config.String("WIREGUARD_SERVER_ADDRESS", "10.70.0.1/16")),
		ListenPort: config.Int("WIREGUARD_SERVER_LISTEN_PORT", 51820),
	}
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
