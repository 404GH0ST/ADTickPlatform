package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"adplatform/internal/services/apigateway"
)

// During a match pause only organizer peers keep game-network access. A
// participant team must not keep it by renaming itself "Organizer"; a
// team-bound organizer must keep it even though its peer carries a team name.
func TestPausedMatchKeepsOnlyRoleOrganizersOnline(t *testing.T) {
	tmpDir := t.TempDir()
	applier := &hostWireGuardApplier{
		fileWireGuardApplier: fileWireGuardApplier{mode: "host", paths: artifactPaths{
			configPath: filepath.Join(tmpDir, "wg0.conf"),
			peersPath:  filepath.Join(tmpDir, "peers.json"),
			rulesPath:  filepath.Join(tmpDir, "nftables.conf"),
			statusPath: filepath.Join(tmpDir, "status.json"),
		}},
		interfaceName: "wg0", wgBinary: "wg", firewallBackend: "nftables", nftBinary: "nft",
		nftTable: "adplatform_wireguard", applyTimeout: 5 * time.Second, runner: &recordingCommandRunner{},
	}
	key := "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA="
	psk := "BBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB="
	snapshot := wireGuardGatewaySnapshot{
		GeneratedAt: time.Now().UTC().Format(time.RFC3339), Revision: "rev", Config: "[Interface]\n", ServerIP: "10.70.0.1",
		MatchPaused: true,
		Peers: []apigateway.WireGuardGatewayPeer{
			{WireGuardPeer: "team-101-player-1", TeamID: 101, TeamName: "Organizer", Address: "10.70.11.20", Status: "active", ClientPublicKey: key, PresharedKey: psk},
			{WireGuardPeer: "team-102-player-2", TeamID: 102, TeamName: "Team Delta", Address: "10.70.12.21", Status: "active", ClientPublicKey: key, PresharedKey: psk, Organizer: true},
			{WireGuardPeer: "organizer-player-3", TeamName: "Organizer", Address: "10.70.0.30", Status: "active", ClientPublicKey: key, PresharedKey: psk, Organizer: true},
		},
	}
	if _, err := applier.Apply(context.Background(), snapshot); err != nil {
		t.Fatal(err)
	}
	rules, err := os.ReadFile(applier.paths.rulesPath)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(rules), "10.70.11.20") {
		t.Fatalf("a participant team renamed to \"Organizer\" kept access during the pause:\n%s", rules)
	}
	for _, address := range []string{"10.70.12.21", "10.70.0.30"} {
		if !strings.Contains(string(rules), address) {
			t.Fatalf("organizer peer %s lost access during the pause:\n%s", address, rules)
		}
	}
}
