package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"adplatform/internal/platform/httpapi"
	"adplatform/internal/services/apigateway"
)

const (
	testPeerKey = "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA="
	testPSK     = "BBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB="
)

func testPaths(dir string) artifactPaths {
	return artifactPaths{
		configPath: filepath.Join(dir, "wg0.conf"),
		peersPath:  filepath.Join(dir, "peers.json"),
		rulesPath:  filepath.Join(dir, "nftables.conf"),
		statusPath: filepath.Join(dir, "status.json"),
	}
}

func testSnapshot() wireGuardGatewaySnapshot {
	return wireGuardGatewaySnapshot{
		GeneratedAt: time.Now().UTC().Format(time.RFC3339), Revision: "rev1", Config: "[Interface]\n", ServerIP: "10.70.0.1",
		Peers: []apigateway.WireGuardGatewayPeer{{WireGuardPeer: "p1", TeamName: "A", Address: "10.70.1.2", Status: "active", ClientPublicKey: testPeerKey, PresharedKey: testPSK}},
	}
}

func TestNewWireGuardApplierSelectsMode(t *testing.T) {
	for mode, check := range map[string]func(wireGuardApplier) bool{
		"files": func(a wireGuardApplier) bool { f, ok := a.(*fileWireGuardApplier); return ok && f.mode == "files" },
		"host": func(a wireGuardApplier) bool {
			h, ok := a.(*hostWireGuardApplier)
			return ok && h.nftTable == "customtable" && h.wgBinary == "wg"
		},
		"dry-run": func(a wireGuardApplier) bool { _, ok := a.(dryRunWireGuardApplier); return ok },
		"bogus":   func(a wireGuardApplier) bool { _, ok := a.(dryRunWireGuardApplier); return ok },
	} {
		t.Setenv("WIREGUARD_GATEWAY_MODE", mode)
		t.Setenv("WIREGUARD_GATEWAY_FIREWALL_TABLE", "Custom-Table!")
		if !check(newWireGuardApplier()) {
			t.Fatalf("mode %q selected the wrong applier", mode)
		}
	}
}

func TestDryRunApplierIsInert(t *testing.T) {
	applier := dryRunWireGuardApplier{}
	if applier.Status().Mode == "" {
		t.Fatal("dry-run status must report its mode")
	}
	if status, err := applier.Apply(context.Background(), testSnapshot()); err != nil || status.State == "" {
		t.Fatalf("dry-run apply: %+v %v", status, err)
	}
	if err := applier.Teardown(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestFileApplierStatusReadsArtifact(t *testing.T) {
	dir := t.TempDir()
	if status := (&fileWireGuardApplier{mode: "files"}).Status(); status.State != "idle" {
		t.Fatalf("no status path: want idle, got %+v", status)
	}
	applier := &fileWireGuardApplier{mode: "files", paths: testPaths(dir)}
	if status := applier.Status(); status.State != "idle" {
		t.Fatalf("missing file: want idle, got %+v", status)
	}
	if err := os.WriteFile(applier.paths.statusPath, []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	if status := applier.Status(); status.State != "error" || status.LastError == "" {
		t.Fatalf("corrupt file: want error state, got %+v", status)
	}
	if err := os.WriteFile(applier.paths.statusPath, []byte(`{"state":"applied","revision":"r9"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	status := applier.Status()
	if status.State != "applied" || status.Revision != "r9" || status.Mode != "files" || status.ConfigPath == "" || status.PeersPath == "" || status.RulesPath == "" || status.StatusPath == "" {
		t.Fatalf("status must be filled from the artifact and defaults, got %+v", status)
	}
	host := &hostWireGuardApplier{fileWireGuardApplier: *applier, interfaceName: "wg9", firewallBackend: "nftables"}
	if hs := host.Status(); hs.Mode != "host" || hs.Interface != "wg9" || hs.FirewallBackend != "nftables" {
		t.Fatalf("host status: %+v", hs)
	}
}

func TestFileApplierTeardownRemovesArtifacts(t *testing.T) {
	dir := t.TempDir()
	applier := &fileWireGuardApplier{mode: "files", paths: testPaths(dir)}
	if _, err := applier.Apply(context.Background(), testSnapshot()); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(applier.paths.rulesPath, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := applier.Teardown(context.Background()); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{applier.paths.configPath, applier.paths.peersPath, applier.paths.rulesPath, applier.paths.statusPath} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("%s survived teardown", path)
		}
	}
	if err := (&fileWireGuardApplier{}).Teardown(context.Background()); err != nil {
		t.Fatal("teardown with no paths must be a no-op")
	}
}

// blockedPaths returns artifact paths whose parent "directory" is a regular
// file, so every write below it fails.
func blockedPaths(t *testing.T, which string) artifactPaths {
	t.Helper()
	dir := t.TempDir()
	blocker := filepath.Join(dir, "blocker")
	if err := os.WriteFile(blocker, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	paths := testPaths(dir)
	bad := filepath.Join(blocker, "nested", "file")
	switch which {
	case "config":
		paths.configPath = bad
	case "peers":
		paths.peersPath = bad
	case "rules":
		paths.rulesPath = bad
	case "status":
		paths.statusPath = bad
	}
	return paths
}

func TestWriteWireGuardArtifactsSurfacesEveryWriteFailure(t *testing.T) {
	for _, which := range []string{"config", "peers", "rules", "status"} {
		paths := blockedPaths(t, which)
		if err := writeWireGuardArtifacts(paths, testSnapshot(), apigateway.WireGuardGatewayStatus{}, "table inet x {}"); err == nil {
			t.Fatalf("blocked %s directory must fail the write", which)
		}
		applier := &fileWireGuardApplier{mode: "files", paths: paths}
		if status, err := applier.Apply(context.Background(), testSnapshot()); err == nil || status.State != "error" {
			t.Fatalf("file apply with blocked %s: want error status, got %+v %v", which, status, err)
		}
		host := &hostWireGuardApplier{fileWireGuardApplier: *applier, firewallBackend: "nftables", nftTable: "t", interfaceName: "wg0", applyTimeout: time.Second, runner: &recordingCommandRunner{}}
		if status, err := host.Apply(context.Background(), testSnapshot()); err == nil || status.State != "error" {
			t.Fatalf("host apply with blocked %s: want error status, got %+v %v", which, status, err)
		}
	}

	// A directory where a file is expected makes the file write itself fail.
	for _, which := range []string{"config", "peers", "rules", "status"} {
		dir := t.TempDir()
		paths := testPaths(dir)
		target := map[string]string{"config": paths.configPath, "peers": paths.peersPath, "rules": paths.rulesPath, "status": paths.statusPath}[which]
		if err := os.MkdirAll(target, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := writeWireGuardArtifacts(paths, testSnapshot(), apigateway.WireGuardGatewayStatus{}, "table inet x {}"); err == nil {
			t.Fatalf("directory at %s path must fail the write", which)
		}
	}
	if err := writeStatusArtifact("", apigateway.WireGuardGatewayStatus{}); err != nil {
		t.Fatal("an unset status path must be a no-op")
	}
}

type scriptedRunner struct {
	failOn string
	calls  []string
}

func (r *scriptedRunner) Run(_ context.Context, binary string, args ...string) error {
	call := strings.TrimSpace(binary + " " + strings.Join(args, " "))
	r.calls = append(r.calls, call)
	if r.failOn != "" && strings.HasPrefix(call, r.failOn) {
		return errors.New("simulated " + r.failOn + " failure")
	}
	return nil
}

func TestHostApplierFailuresAndTeardown(t *testing.T) {
	for _, failOn := range []string{"wg syncconf", "nft -f"} {
		runner := &scriptedRunner{failOn: failOn}
		applier := &hostWireGuardApplier{fileWireGuardApplier: fileWireGuardApplier{mode: "host", paths: testPaths(t.TempDir())}, interfaceName: "wg0", wgBinary: "wg", firewallBackend: "nftables", nftBinary: "nft", nftTable: "t", applyTimeout: time.Second, runner: runner}
		status, err := applier.Apply(context.Background(), testSnapshot())
		if err == nil || status.State != "error" || !strings.Contains(status.LastError, failOn) {
			t.Fatalf("failure in %q must surface, got %+v %v", failOn, status, err)
		}
		if failOn == "nft -f" && len(runner.calls) != 3 {
			t.Fatalf("a failed firewall update must not delete the installed table: %v", runner.calls)
		}
		if failOn == "wg syncconf" && len(runner.calls) != 2 {
			t.Fatalf("a failed peer sync must not apply a new firewall policy: %v", runner.calls)
		}
	}
	runnerOnNewHost := &scriptedRunner{failOn: "nft list"}
	newHost := &hostWireGuardApplier{fileWireGuardApplier: fileWireGuardApplier{mode: "host", paths: testPaths(t.TempDir())}, interfaceName: "wg0", wgBinary: "wg", firewallBackend: "nftables", nftBinary: "nft", nftTable: "t", applyTimeout: time.Second, runner: runnerOnNewHost}
	if _, err := newHost.Apply(context.Background(), testSnapshot()); err != nil {
		t.Fatal(err)
	}
	calls := runnerOnNewHost.calls
	if len(calls) != 4 || !strings.HasPrefix(calls[1], "nft create table inet t") || !strings.Contains(calls[1], `iifname "wg0" drop`) || !strings.HasPrefix(calls[2], "wg syncconf") || !strings.HasPrefix(calls[3], "nft -f") {
		t.Fatalf("initial apply must activate a deny policy before syncing peers: %v", calls)
	}

	// A status file that cannot be written after a successful apply is an error.
	dir := t.TempDir()
	paths := testPaths(dir)
	runner := &scriptedRunner{}
	applier := &hostWireGuardApplier{fileWireGuardApplier: fileWireGuardApplier{mode: "host", paths: paths}, interfaceName: "wg0", wgBinary: "wg", firewallBackend: "iptables", applyTimeout: time.Second, runner: &statusBreakingRunner{inner: runner, statusPath: paths.statusPath}}
	if status, err := applier.Apply(context.Background(), testSnapshot()); err == nil || status.State != "error" {
		t.Fatalf("final status write failure must surface, got %+v %v", status, err)
	}

	teardownRunner := &scriptedRunner{}
	host := &hostWireGuardApplier{fileWireGuardApplier: fileWireGuardApplier{mode: "host", paths: testPaths(t.TempDir())}, interfaceName: "wg0", firewallBackend: "nftables", nftBinary: "nft", nftTable: "t", applyTimeout: time.Second, runner: teardownRunner}
	if err := host.Teardown(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(teardownRunner.calls, ";"); got != "ip link delete wg0;nft delete table inet t" {
		t.Fatalf("unexpected teardown commands %q", got)
	}
}

// statusBreakingRunner replaces the status file with a directory during the
// wg step, so the final status write fails after a successful apply.
type statusBreakingRunner struct {
	inner      *scriptedRunner
	statusPath string
}

func (r *statusBreakingRunner) Run(ctx context.Context, binary string, args ...string) error {
	if binary == "wg" {
		_ = os.Remove(r.statusPath)
		_ = os.MkdirAll(r.statusPath, 0o700)
	}
	return r.inner.Run(ctx, binary, args...)
}

func TestExecCommandRunnerReportsFailures(t *testing.T) {
	runner := execCommandRunner{}
	if err := runner.Run(context.Background(), "true"); err != nil {
		t.Fatalf("true must succeed: %v", err)
	}
	err := runner.Run(context.Background(), "sh", "-c", "echo boom >&2; exit 3")
	if err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("failure must include output, got %v", err)
	}
}

func TestPeerAndConfigHelpers(t *testing.T) {
	peers := []apigateway.WireGuardGatewayPeer{
		{WireGuardPeer: "ok-1", Address: "10.70.1.2", Status: "active", ClientPublicKey: testPeerKey, PresharedKey: testPSK},
		{WireGuardPeer: "ok-2", Address: "10.70.1.3", Status: "active", ClientPublicKey: testPeerKey},
		{WireGuardPeer: "revoked", Address: "10.70.1.4", Status: "revoked", ClientPublicKey: testPeerKey},
		{WireGuardPeer: "bad-key", Address: "10.70.1.5", Status: "active", ClientPublicKey: "not-base64!"},
		{WireGuardPeer: "short-key", Address: "10.70.1.5", Status: "active", ClientPublicKey: base64.StdEncoding.EncodeToString([]byte("short"))},
		{WireGuardPeer: "bad-psk", Address: "10.70.1.6", Status: "active", ClientPublicKey: testPeerKey, PresharedKey: "nope"},
		{WireGuardPeer: "bad-ip", Address: "fd00::1", Status: "active", ClientPublicKey: testPeerKey},
	}
	active := activeWireGuardPeers(peers)
	if len(active) != 2 || active[0].WireGuardPeer != "ok-1" || active[1].WireGuardPeer != "ok-2" {
		t.Fatalf("unexpected active peers %+v", active)
	}
	rules := renderNftablesRules("t", "wg0", "10.70.0.1", active)
	if !strings.Contains(rules, "10.70.1.2, 10.70.1.3") {
		t.Fatalf("multiple peers must be comma-joined in the set:\n%s", rules)
	}
	if long := sanitizeConfigComment(strings.Repeat("x", 200) + "\nInjected"); len(long) != 96 || strings.Contains(long, "\n") {
		t.Fatalf("comments must be truncated and single-line, got %q", long)
	}
	if got := parseIPv4CIDRList(""); got != nil {
		t.Fatalf("empty list: %v", got)
	}
	if got := parseIPv4CIDRList("10.0.0.1, ,10.0.0.0/8,10.0.0.0/8,bogus"); strings.Join(got, ",") != "10.0.0.1/32,10.0.0.0/8" {
		t.Fatalf("unexpected CIDRs %v", got)
	}
	t.Setenv("WIREGUARD_FORWARD_ALLOWED_CIDRS", "")
	t.Setenv("WIREGUARD_SERVER_ALLOWED_IPS", "10.9.0.0/16")
	if got := wireGuardForwardAllowedCIDRs(); strings.Join(got, ",") != "10.9.0.0/16" {
		t.Fatalf("fallback to allowed IPs failed: %v", got)
	}
	if wireGuardServerIP("") != "" || wireGuardServerIP("10.70.0.1") != "10.70.0.1" {
		t.Fatal("server IP parsing")
	}
	for in, want := range map[string]string{"": "adplatform_wireguard", "!!!": "adplatform_wireguard", " Prod-WG_1 ": "prodwg_1"} {
		if got := sanitizeNftTableName(in); got != want {
			t.Fatalf("sanitizeNftTableName(%q) = %q, want %q", in, got, want)
		}
	}
	if err := validateWireGuardGatewayPrivateKey(base64.StdEncoding.EncodeToString([]byte("short"))); err == nil {
		t.Fatal("short private keys must be rejected")
	}
	private, public := defaultWireGuardGatewayKeypair()
	if err := validateWireGuardGatewayPrivateKey(private); err != nil || !isValidWireGuardPublicKey(public) {
		t.Fatalf("dev keypair invalid: %v", err)
	}
}

type gatewayStore struct {
	apigateway.Store
	peers    []apigateway.WireGuardGatewayPeer
	listErr  error
	pauseErr error
}

func (s gatewayStore) ListWireGuardGatewayPeers(context.Context) ([]apigateway.WireGuardGatewayPeer, error) {
	return s.peers, s.listErr
}

func (s gatewayStore) IsMatchPaused(context.Context) (bool, error) { return false, s.pauseErr }

type failingApplier struct{ dryRunWireGuardApplier }

func (failingApplier) Apply(context.Context, wireGuardGatewaySnapshot) (apigateway.WireGuardGatewayStatus, error) {
	return apigateway.WireGuardGatewayStatus{State: "error", LastError: "apply exploded"}, errors.New("apply exploded")
}

func (failingApplier) Teardown(context.Context) error { return errors.New("teardown exploded") }

func gatewayCall(t *testing.T, store apigateway.Store, applier wireGuardApplier, method, path string) *httptest.ResponseRecorder {
	t.Helper()
	mux := httpapi.NewBaseMux(httpapi.ServiceInfo{Name: "wireguard-gateway", Version: "test", Addr: ":0"})
	newWireGuardGatewayServer("wg-token", store, applier).RegisterRoutes(mux)
	request := httptest.NewRequest(method, path, &bytes.Buffer{})
	request.Header.Set("Authorization", "Bearer wg-token")
	recorder := httptest.NewRecorder()
	mux.ServeHTTP(recorder, request)
	return recorder
}

func TestGatewayHandlers(t *testing.T) {
	t.Setenv("WIREGUARD_GATEWAY_MODE", "dry-run")
	t.Setenv("API_GATEWAY_STATE_BACKEND", "memory")
	t.Setenv("WIREGUARD_GATEWAY_STATE_BACKEND", "memory")
	t.Setenv("WIREGUARD_SERVER_PRIVATE_KEY", "")

	if response := gatewayCall(t, gatewayStore{}, dryRunWireGuardApplier{}, http.MethodGet, "/internal/v1/wireguard/status"); response.Code != http.StatusOK {
		t.Fatalf("status: %d", response.Code)
	}
	if response := gatewayCall(t, gatewayStore{listErr: errors.New("db down")}, dryRunWireGuardApplier{}, http.MethodPost, "/internal/v1/wireguard/reconcile"); response.Code != http.StatusInternalServerError {
		t.Fatalf("store failure: %d", response.Code)
	}
	// An unknown pause state fails closed but the reconcile still applies.
	if response := gatewayCall(t, gatewayStore{pauseErr: errors.New("db down")}, dryRunWireGuardApplier{}, http.MethodPost, "/internal/v1/wireguard/reconcile"); response.Code != http.StatusOK {
		t.Fatalf("pause-state failure must still reconcile (paused): %d %s", response.Code, response.Body.String())
	}
	if response := gatewayCall(t, gatewayStore{}, failingApplier{}, http.MethodPost, "/internal/v1/wireguard/reconcile"); response.Code != http.StatusInternalServerError || !strings.Contains(response.Body.String(), "apply exploded") {
		t.Fatalf("apply failure: %d %s", response.Code, response.Body.String())
	}
	if response := gatewayCall(t, gatewayStore{}, failingApplier{}, http.MethodPost, "/internal/v1/wireguard/teardown"); response.Code != http.StatusInternalServerError {
		t.Fatalf("teardown failure: %d", response.Code)
	}
	if response := gatewayCall(t, gatewayStore{}, dryRunWireGuardApplier{}, http.MethodPost, "/internal/v1/wireguard/teardown"); response.Code != http.StatusNoContent {
		t.Fatalf("teardown: %d", response.Code)
	}

	t.Setenv("WIREGUARD_SERVER_PRIVATE_KEY", "invalid-key")
	if response := gatewayCall(t, gatewayStore{}, dryRunWireGuardApplier{}, http.MethodPost, "/internal/v1/wireguard/reconcile"); response.Code != http.StatusInternalServerError || !strings.Contains(response.Body.String(), "configuration invalid") {
		t.Fatalf("bad server key: %d %s", response.Code, response.Body.String())
	}
	if _, err := buildWireGuardGatewaySnapshot(nil, time.Now()); err == nil {
		t.Fatal("snapshot with an invalid server key must fail")
	}
}

func TestFileApplierStatusFillsBlankFields(t *testing.T) {
	applier := &fileWireGuardApplier{mode: "files", paths: testPaths(t.TempDir())}
	blank := `{"state":"applied","mode":"","config_path":"","peers_path":"","rules_path":"","status_path":""}`
	if err := os.WriteFile(applier.paths.statusPath, []byte(blank), 0o600); err != nil {
		t.Fatal(err)
	}
	status := applier.Status()
	if status.Mode != "files" || status.ConfigPath != applier.paths.configPath || status.PeersPath != applier.paths.peersPath || status.RulesPath != applier.paths.rulesPath || status.StatusPath != applier.paths.statusPath {
		t.Fatalf("blank fields must fall back to configuration, got %+v", status)
	}
}

func TestRestoreWireGuardStateBranches(t *testing.T) {
	server := newWireGuardGatewayServer("t", nil, &wireGuardApplierStub{})
	t.Setenv("WIREGUARD_GATEWAY_RECONCILE_ON_STARTUP", "false")
	if err := restoreWireGuardState(context.Background(), wireGuardStartupStoreStub{err: errors.New("unused")}, server); err != nil {
		t.Fatalf("disabled restore must be a no-op, got %v", err)
	}
	t.Setenv("WIREGUARD_GATEWAY_RECONCILE_ON_STARTUP", "true")
	if err := restoreWireGuardState(context.Background(), wireGuardStartupStoreStub{err: errors.New("db down")}, server); err == nil {
		t.Fatal("peer listing failure must abort startup")
	}
	t.Setenv("WIREGUARD_SERVER_PRIVATE_KEY", "invalid")
	if err := restoreWireGuardState(context.Background(), wireGuardStartupStoreStub{}, server); err == nil {
		t.Fatal("invalid server settings must abort startup")
	}
}

func TestMetricsTolerateZeroValues(t *testing.T) {
	metrics := &wireGuardGatewayMetrics{}
	metrics.recordOperation(wireGuardOperationStatus, time.Millisecond, true)
	server := newWireGuardGatewayServer("t", nil, dryRunWireGuardApplier{})
	server.metrics = &wireGuardGatewayMetrics{}
	var out bytes.Buffer
	server.WritePrometheusMetrics(&out)
	if out.Len() == 0 {
		t.Fatal("metrics output must not be empty")
	}
	if !metricParseTime("").IsZero() || !metricParseTime("yesterday").IsZero() || metricParseTime("2026-01-02T03:04:05Z").IsZero() {
		t.Fatal("metricParseTime")
	}
}

func TestArtifactEncodingFailureIsReported(t *testing.T) {
	original := marshalIndent
	t.Cleanup(func() { marshalIndent = original })
	marshalIndent = func(any, string, string) ([]byte, error) { return nil, errors.New("encode failed") }
	if err := writeWireGuardArtifacts(testPaths(t.TempDir()), testSnapshot(), apigateway.WireGuardGatewayStatus{}, ""); err == nil {
		t.Fatal("manifest encoding failure must surface")
	}
	if err := writeStatusArtifact(filepath.Join(t.TempDir(), "status.json"), apigateway.WireGuardGatewayStatus{}); err == nil {
		t.Fatal("status encoding failure must surface")
	}
}
