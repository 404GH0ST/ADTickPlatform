package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"adplatform/internal/services/apigateway"
)

func TestRenderWireGuardGatewayConfigSkipsRevokedPeers(t *testing.T) {
	settings := wireGuardServerSettings{PrivateKey: "server-private", Address: "10.70.0.1/24", ListenPort: 51820}
	configBody := renderWireGuardGatewayConfig(settings, activeWireGuardPeers([]apigateway.WireGuardGatewayPeer{
		{WireGuardPeer: "team-101-player-1", DisplayName: "Alpha", TeamName: "Team Alpha", Address: "10.70.11.20", Status: "active", ClientPublicKey: "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=", PresharedKey: "BBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB="},
		{WireGuardPeer: "team-102-player-2", DisplayName: "Delta", TeamName: "Team Delta", Address: "10.70.12.21", Status: "revoked", ClientPublicKey: "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=", PresharedKey: "BBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB="},
	}))

	if !strings.Contains(configBody, "team-101-player-1") {
		t.Fatalf("expected active peer to be rendered")
	}
	if strings.Contains(configBody, "team-102-player-2") {
		t.Fatalf("expected revoked peer to be omitted")
	}
	if !strings.Contains(configBody, "AllowedIPs = 10.70.11.20/32") {
		t.Fatalf("expected active peer address to be rendered")
	}
	if strings.Contains(configBody, "Address = ") {
		t.Fatalf("expected syncconf config to omit wg-quick-only Address directive")
	}
	if strings.Contains(configBody, "SaveConfig = ") {
		t.Fatalf("expected syncconf config to omit wg-quick-only SaveConfig directive")
	}
}

func TestRenderNftablesRulesSkipsRevokedPeers(t *testing.T) {
	rules := renderNftablesRules("adplatform_wireguard", "wg0", "10.70.0.1", activeWireGuardPeers([]apigateway.WireGuardGatewayPeer{
		{WireGuardPeer: "team-101-player-1", Address: "10.70.11.20", Status: "active", ClientPublicKey: "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA="},
		{WireGuardPeer: "team-102-player-2", Address: "10.70.12.21", Status: "revoked", ClientPublicKey: "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA="},
	}))

	if !strings.Contains(rules, `iifname "wg0" ip saddr != @active_peers drop`) {
		t.Fatalf("expected interface-scoped drop rule")
	}
	if !strings.Contains(rules, `iifname "wg0" ip saddr @active_peers ip daddr 10.70.0.1 tcp dport { 80, 443 } accept`) {
		t.Fatalf("expected host edge allow rule")
	}
	if !strings.Contains(rules, `iifname "wg0" ip saddr @active_peers ip daddr 10.70.0.1 icmp type echo-request accept`) {
		t.Fatalf("expected host icmp allow rule")
	}
	if !strings.Contains(rules, "10.70.11.20") {
		t.Fatalf("expected active peer ip in rules")
	}
	if strings.Contains(rules, "10.70.12.21") {
		t.Fatalf("expected revoked peer ip to be omitted")
	}
}

func TestResolveWireGuardServerSettingsRejectsMissingKeyOutsideDryRunMemory(t *testing.T) {
	t.Setenv("WIREGUARD_SERVER_PRIVATE_KEY", "")
	t.Setenv("API_GATEWAY_STATE_BACKEND", "postgres")
	t.Setenv("WIREGUARD_GATEWAY_STATE_BACKEND", "postgres")
	t.Setenv("WIREGUARD_GATEWAY_MODE", "host")

	if _, err := resolveWireGuardServerSettings(); err == nil {
		t.Fatal("expected missing WireGuard server key to fail outside dry-run memory mode")
	}
}

func TestResolveWireGuardServerSettingsRejectsInvalidPrivateKey(t *testing.T) {
	t.Setenv("WIREGUARD_SERVER_PRIVATE_KEY", "not-base64")
	t.Setenv("API_GATEWAY_STATE_BACKEND", "postgres")
	t.Setenv("WIREGUARD_GATEWAY_STATE_BACKEND", "postgres")
	t.Setenv("WIREGUARD_GATEWAY_MODE", "host")

	if _, err := resolveWireGuardServerSettings(); err == nil {
		t.Fatal("expected invalid WireGuard server key to fail")
	}
}

func TestResolveWireGuardServerSettingsAllowsDevFallbackInDryRunMemory(t *testing.T) {
	t.Setenv("WIREGUARD_SERVER_PRIVATE_KEY", "")
	t.Setenv("API_GATEWAY_STATE_BACKEND", "memory")
	t.Setenv("WIREGUARD_GATEWAY_STATE_BACKEND", "memory")
	t.Setenv("WIREGUARD_GATEWAY_MODE", "dry-run")

	settings, err := resolveWireGuardServerSettings()
	if err != nil {
		t.Fatalf("expected dry-run memory fallback, got %v", err)
	}
	if settings.PrivateKey == "" {
		t.Fatal("expected dev fallback private key")
	}
}

func TestFileWireGuardApplierWritesArtifacts(t *testing.T) {
	tmpDir := t.TempDir()
	applier := &fileWireGuardApplier{
		mode: "files",
		paths: artifactPaths{
			configPath: filepath.Join(tmpDir, "wg0.conf"),
			peersPath:  filepath.Join(tmpDir, "peers.json"),
			statusPath: filepath.Join(tmpDir, "status.json"),
		},
	}

	snapshot := wireGuardGatewaySnapshot{
		GeneratedAt:  time.Date(2026, time.March, 10, 8, 0, 0, 0, time.UTC).Format(time.RFC3339),
		Revision:     "deadbeefcafe",
		Config:       "[Interface]\nPrivateKey = server-private\n",
		Peers:        []apigateway.WireGuardGatewayPeer{{WireGuardPeer: "team-101-player-1", Status: "active"}},
		PeersTotal:   1,
		PeersActive:  1,
		PeersRevoked: 0,
	}

	status, err := applier.Apply(context.Background(), snapshot)
	if err != nil {
		t.Fatalf("expected apply to succeed, got %v", err)
	}
	if status.Mode != "files" || status.State != "applied" {
		t.Fatalf("unexpected status: %+v", status)
	}

	configBody, err := os.ReadFile(applier.paths.configPath)
	if err != nil {
		t.Fatalf("expected config file, got %v", err)
	}
	if string(configBody) != snapshot.Config {
		t.Fatalf("unexpected config body %q", string(configBody))
	}

	statusBody, err := os.ReadFile(applier.paths.statusPath)
	if err != nil {
		t.Fatalf("expected status file, got %v", err)
	}
	if !strings.Contains(string(statusBody), snapshot.Revision) {
		t.Fatalf("expected revision in status file")
	}
}

func TestHostWireGuardApplierRunsWgAndNft(t *testing.T) {
	tmpDir := t.TempDir()
	runner := &recordingCommandRunner{}
	applier := &hostWireGuardApplier{
		fileWireGuardApplier: fileWireGuardApplier{
			mode: "host",
			paths: artifactPaths{
				configPath: filepath.Join(tmpDir, "wg0.conf"),
				peersPath:  filepath.Join(tmpDir, "peers.json"),
				rulesPath:  filepath.Join(tmpDir, "nftables.conf"),
				statusPath: filepath.Join(tmpDir, "status.json"),
			},
		},
		interfaceName:   "wg0",
		wgBinary:        "wg",
		firewallBackend: "nftables",
		nftBinary:       "nft",
		nftTable:        "adplatform_wireguard",
		applyTimeout:    5 * time.Second,
		runner:          runner,
	}

	snapshot := wireGuardGatewaySnapshot{
		GeneratedAt: time.Date(2026, time.March, 10, 8, 0, 0, 0, time.UTC).Format(time.RFC3339),
		Revision:    "deadbeefcafe",
		Config:      "[Interface]\nPrivateKey = server-private\n",
		ServerIP:    "10.70.0.1",
		Peers: []apigateway.WireGuardGatewayPeer{
			{WireGuardPeer: "team-101-player-1", DisplayName: "Alpha", TeamName: "Team Alpha", Address: "10.70.11.20", Status: "active", ClientPublicKey: "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=", PresharedKey: "BBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB="},
			{WireGuardPeer: "team-102-player-2", DisplayName: "Delta", TeamName: "Team Delta", Address: "10.70.12.21", Status: "revoked", ClientPublicKey: "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=", PresharedKey: "BBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB="},
		},
		PeersTotal:   2,
		PeersActive:  1,
		PeersRevoked: 1,
	}

	status, err := applier.Apply(context.Background(), snapshot)
	if err != nil {
		t.Fatalf("expected apply to succeed, got %v", err)
	}
	if status.Mode != "host" || status.Interface != "wg0" || status.FirewallBackend != "nftables" {
		t.Fatalf("unexpected status: %+v", status)
	}
	if len(runner.commands) != 3 {
		t.Fatalf("expected 3 commands, got %d", len(runner.commands))
	}
	if runner.commands[0] != "wg syncconf wg0 "+applier.paths.configPath {
		t.Fatalf("unexpected wg command %q", runner.commands[0])
	}
	if runner.commands[1] != "nft delete table inet adplatform_wireguard" {
		t.Fatalf("unexpected nft delete command %q", runner.commands[1])
	}
	if runner.commands[2] != "nft -f "+applier.paths.rulesPath {
		t.Fatalf("unexpected nft apply command %q", runner.commands[2])
	}

	rulesBody, err := os.ReadFile(applier.paths.rulesPath)
	if err != nil {
		t.Fatalf("expected rules file, got %v", err)
	}
	if strings.Contains(string(rulesBody), "flush table inet") {
		t.Fatalf("expected rules body to omit flush table directive")
	}
	if !strings.Contains(string(rulesBody), "10.70.11.20") || strings.Contains(string(rulesBody), "10.70.12.21") {
		t.Fatalf("unexpected rules body %q", string(rulesBody))
	}
}

func TestHostWireGuardApplierSurfacesRunnerError(t *testing.T) {
	tmpDir := t.TempDir()
	applier := &hostWireGuardApplier{
		fileWireGuardApplier: fileWireGuardApplier{
			mode: "host",
			paths: artifactPaths{
				configPath: filepath.Join(tmpDir, "wg0.conf"),
				peersPath:  filepath.Join(tmpDir, "peers.json"),
				rulesPath:  filepath.Join(tmpDir, "nftables.conf"),
				statusPath: filepath.Join(tmpDir, "status.json"),
			},
		},
		interfaceName:   "wg0",
		wgBinary:        "wg",
		firewallBackend: "none",
		applyTimeout:    5 * time.Second,
		runner:          failingCommandRunner{err: errors.New("wg syncconf failed")},
	}

	status, err := applier.Apply(context.Background(), wireGuardGatewaySnapshot{
		GeneratedAt:  time.Date(2026, time.March, 10, 8, 0, 0, 0, time.UTC).Format(time.RFC3339),
		Revision:     "deadbeefcafe",
		Config:       "[Interface]\nPrivateKey = server-private\n",
		PeersTotal:   0,
		PeersActive:  0,
		PeersRevoked: 0,
	})
	if err == nil {
		t.Fatal("expected apply to fail")
	}
	if status.State != "error" || !strings.Contains(status.LastError, "wg syncconf failed") {
		t.Fatalf("unexpected status %+v", status)
	}
}

func TestWireGuardServerIPStripsCIDR(t *testing.T) {
	if serverIP := wireGuardServerIP("10.70.0.1/24"); serverIP != "10.70.0.1" {
		t.Fatalf("expected server ip without cidr, got %q", serverIP)
	}
}

func TestWireGuardGatewayRoutesRequireAdminAuth(t *testing.T) {
	server := newWireGuardGatewayServer("admin-token", apigateway.NewMemoryStore(101), dryRunWireGuardApplier{})
	mux := http.NewServeMux()
	server.RegisterRoutes(mux)
	cases := []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/internal/v1/wireguard/status"},
		{http.MethodPost, "/internal/v1/wireguard/reconcile"},
		{http.MethodPost, "/internal/v1/wireguard/teardown"},
	}

	for _, tc := range cases {
		t.Run(tc.method+" "+tc.path+" unauthenticated", func(t *testing.T) {
			request := httptest.NewRequest(tc.method, tc.path, nil)
			response := httptest.NewRecorder()

			mux.ServeHTTP(response, request)

			if response.Code != http.StatusForbidden {
				t.Fatalf("expected 403, got %d: %s", response.Code, response.Body.String())
			}
		})

		t.Run(tc.method+" "+tc.path+" wrong token", func(t *testing.T) {
			request := httptest.NewRequest(tc.method, tc.path, nil)
			request.Header.Set("Authorization", "Bearer wrong-token")
			response := httptest.NewRecorder()

			mux.ServeHTTP(response, request)

			if response.Code != http.StatusForbidden {
				t.Fatalf("expected 403, got %d: %s", response.Code, response.Body.String())
			}
		})
	}
}

func TestWireGuardGatewayServerReconcileReturnsCounts(t *testing.T) {
	store := apigateway.NewMemoryStore(101)
	if _, err := store.RevokeAdminPlayerWireGuardConfig(context.Background(), 2, time.Date(2026, time.March, 10, 8, 0, 0, 0, time.UTC)); err != nil {
		t.Fatalf("expected revoke to succeed, got %v", err)
	}
	server := newWireGuardGatewayServer("admin-token", store, dryRunWireGuardApplier{})

	mux := http.NewServeMux()
	server.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodPost, "/internal/v1/wireguard/reconcile", nil)
	req.Header.Set("Authorization", "Bearer admin-token")
	resp := httptest.NewRecorder()
	mux.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.Code)
	}

	var payload apigateway.WireGuardGatewayStatus
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if payload.PeersTotal != 4 || payload.PeersActive != 3 || payload.PeersRevoked != 1 {
		t.Fatalf("unexpected counts: %+v", payload)
	}
	if payload.Mode != "dry-run" || payload.State != "applied" {
		t.Fatalf("unexpected status: %+v", payload)
	}
}

type recordingCommandRunner struct {
	commands []string
}

func (r *recordingCommandRunner) Run(_ context.Context, binary string, args ...string) error {
	r.commands = append(r.commands, strings.TrimSpace(binary+" "+strings.Join(args, " ")))
	return nil
}

type failingCommandRunner struct {
	err error
}

func (f failingCommandRunner) Run(context.Context, string, ...string) error {
	return f.err
}
