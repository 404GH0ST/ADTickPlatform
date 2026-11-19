package apigateway

import (
	"context"
	"testing"
	"time"

	"adplatform/internal/platform/database"
	"adplatform/internal/platform/database/dbtest"
)

func assertWireGuardOrganizerFlagFollowsRole(t *testing.T, store Store) {
	t.Helper()
	ctx := context.Background()
	now := time.Now().UTC()
	team, err := store.CreateAdminTeam(ctx, adminCreateTeamRequest{Name: "Spoof Team", ContactEmail: "spoof@example.test"})
	if err != nil {
		t.Fatal(err)
	}
	captain, err := store.CreateAdminPlayer(ctx, adminCreatePlayerRequest{TeamID: team.ID, DisplayName: "Spoofer", Email: "spoofer@example.test", Password: "spoofer-password", Role: "captain"}, now)
	if err != nil {
		t.Fatal(err)
	}
	// The captain renames the team to the label the gateway used to trust.
	if _, err := store.UpdateParticipantProfile(ctx, captain.ID, participantUpdateProfileRequest{DisplayName: "Spoofer", Email: "spoofer@example.test", TeamName: "Organizer", TeamContactEmail: "spoof@example.test"}); err != nil {
		t.Fatalf("rename team: %v", err)
	}
	bound, err := store.CreateAdminPlayer(ctx, adminCreatePlayerRequest{TeamID: team.ID, DisplayName: "Bound Org", Email: "bound-org@example.test", Password: "bound-password", Role: "organizer"}, now)
	if err != nil {
		t.Fatal(err)
	}
	peers, err := store.ListWireGuardGatewayPeers(ctx)
	if err != nil {
		t.Fatal(err)
	}
	seen := map[int]WireGuardGatewayPeer{}
	for _, peer := range peers {
		seen[peer.PlayerID] = peer
	}
	spoof, ok := seen[captain.ID]
	if !ok || spoof.TeamName != "Organizer" {
		t.Fatalf("fixture: captain peer should carry the spoofed team name, got %+v", spoof)
	}
	if spoof.Organizer {
		t.Fatal("a participant whose team is named \"Organizer\" must not be flagged as an organizer peer")
	}
	if peer, ok := seen[bound.ID]; !ok || !peer.Organizer {
		t.Fatalf("a team-bound organizer must be flagged as an organizer peer, got %+v", peer)
	}
}

func TestMemoryWireGuardOrganizerFlagFollowsRole(t *testing.T) {
	assertWireGuardOrganizerFlagFollowsRole(t, NewMemoryStore(101))
}

func TestPostgresWireGuardOrganizerFlagFollowsRole(t *testing.T) {
	db := dbtest.IsolatedSchema(t, "TEST_POSTGRES_DSN", "wg_organizer_flag_test")
	t.Setenv("WIREGUARD_SERVER_PUBLIC_KEY", "flag-server-public-key")
	if err := database.RunEmbeddedMigrationsWithOptions(context.Background(), db, database.MigrationOptions{IncludeSeeds: false}); err != nil {
		t.Fatal(err)
	}
	assertWireGuardOrganizerFlagFollowsRole(t, NewPostgresStore(db))
}
