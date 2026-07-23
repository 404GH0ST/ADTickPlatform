package apigateway

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestMatchNotStartedHidesChallengesAndClosesNetwork(t *testing.T) {
	store := NewMemoryStore(101).(*memoryStore)
	ctx := context.Background()
	store.setMatchStartedForTest(false)

	challenges, err := store.ListChallenges(ctx)
	if err != nil {
		t.Fatalf("list challenges: %v", err)
	}
	if len(challenges) != 0 {
		t.Fatalf("expected empty challenge catalog before match start, got %+v", challenges)
	}
	public, err := store.ListPublicServices(ctx)
	if err != nil {
		t.Fatalf("list public: %v", err)
	}
	if len(public) != 0 {
		t.Fatalf("expected no public services before match start, got %+v", public)
	}
	if err := store.ValidateServiceAction(ctx, 101, 1); !errors.Is(err, ErrMatchNotStarted) {
		t.Fatalf("expected ErrMatchNotStarted, got %v", err)
	}
	policies, err := store.ListControllerServiceAccessPolicies(ctx)
	if err != nil {
		t.Fatalf("list policies: %v", err)
	}
	for _, policy := range policies {
		if !policy.NetworkClosed {
			t.Fatalf("expected network closed before match start, got %+v", policy)
		}
	}

	store.setMatchStartedForTest(true)
	challenges, err = store.ListChallenges(ctx)
	if err != nil {
		t.Fatalf("list challenges after start: %v", err)
	}
	if len(challenges) == 0 {
		t.Fatal("expected challenges after match start")
	}
}

func TestDeactivatedPlayerIsRemovedFromWireGuardGatewayPeers(t *testing.T) {
	store := NewMemoryStore(101)
	ctx := context.Background()
	if _, err := store.SetPlayerActive(ctx, 1, false, time.Now()); err != nil {
		t.Fatalf("deactivate player: %v", err)
	}
	peers, err := store.ListWireGuardGatewayPeers(ctx)
	if err != nil {
		t.Fatalf("list peers: %v", err)
	}
	for _, peer := range peers {
		if peer.PlayerID == 1 {
			t.Fatalf("deactivated player remained in gateway peers: %+v", peer)
		}
	}
}

func TestDeactivatedPlayerStillRequiresTheCorrectPassword(t *testing.T) {
	store := NewMemoryStore(101)
	ctx := context.Background()
	if _, err := store.SetPlayerActive(ctx, 1, false, time.Now()); err != nil {
		t.Fatalf("deactivate player: %v", err)
	}
	if _, err := store.AuthenticatePlayer(ctx, "alpha.captain@example.com", "wrong-password"); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("expected wrong password to hide account state, got %v", err)
	}
	if _, err := store.AuthenticatePlayer(ctx, "alpha.captain@example.com", "alpha-secret"); !errors.Is(err, ErrAccountDeactivated) {
		t.Fatalf("expected correct password to report deactivation, got %v", err)
	}
}

func TestReactivatedCaptainIsDemotedWhenTeamAlreadyHasCaptain(t *testing.T) {
	store := NewMemoryStore(101).(*memoryStore)
	ctx := context.Background()

	// Deactivate the seeded captain of team 101, leaving it captain-less.
	if _, err := store.SetPlayerActive(ctx, 1, false, time.Now()); err != nil {
		t.Fatalf("deactivate captain: %v", err)
	}

	// Adding a plain member to a captain-less team backfills the captaincy.
	promoted, err := store.CreateAdminPlayer(ctx, adminCreatePlayerRequest{
		TeamID:      101,
		DisplayName: "Backfill Captain",
		Email:       "backfill@example.com",
		Password:    "backfill-secret",
		Role:        "member",
	}, time.Now())
	if err != nil {
		t.Fatalf("create backfill player: %v", err)
	}
	if promoted.Role != "captain" {
		t.Fatalf("expected backfill member promoted to captain, got %q", promoted.Role)
	}

	// Reactivating the original captain must not create a second captain.
	reactivated, err := store.SetPlayerActive(ctx, 1, true, time.Now())
	if err != nil {
		t.Fatalf("reactivate original captain: %v", err)
	}
	if reactivated.Role != "member" {
		t.Fatalf("expected reactivated captain demoted to member, got %q", reactivated.Role)
	}

	members, err := store.ListTeamMembers(ctx, 101)
	if err != nil {
		t.Fatalf("list team members: %v", err)
	}
	captains := 0
	for _, m := range members {
		if strings.EqualFold(strings.TrimSpace(m.Role), "captain") {
			captains++
		}
	}
	if captains != 1 {
		t.Fatalf("expected exactly one captain on team 101, got %d (%+v)", captains, members)
	}
}

func TestReactivatedCaptainKeepsRoleWhenNoOtherCaptain(t *testing.T) {
	store := NewMemoryStore(101).(*memoryStore)
	ctx := context.Background()

	if _, err := store.SetPlayerActive(ctx, 1, false, time.Now()); err != nil {
		t.Fatalf("deactivate captain: %v", err)
	}
	reactivated, err := store.SetPlayerActive(ctx, 1, true, time.Now())
	if err != nil {
		t.Fatalf("reactivate captain: %v", err)
	}
	if reactivated.Role != "captain" {
		t.Fatalf("expected sole captain to remain captain after reactivation, got %q", reactivated.Role)
	}
}

func TestMemoryStoreReusesDeletedTeamNetworkID(t *testing.T) {
	store := NewMemoryStore(101)
	ctx := context.Background()
	created, err := store.CreateAdminTeam(ctx, adminCreateTeamRequest{Name: "Temporary Team", ContactEmail: "temporary@example.com"})
	if err != nil {
		t.Fatalf("create temporary team: %v", err)
	}
	if created.ID != 105 {
		t.Fatalf("expected first free team network id 105, got %d", created.ID)
	}
	if err := store.DeleteAdminTeam(ctx, created.ID); err != nil {
		t.Fatalf("delete temporary team: %v", err)
	}
	replacement, err := store.CreateAdminTeam(ctx, adminCreateTeamRequest{Name: "Replacement Team", ContactEmail: "replacement@example.com"})
	if err != nil {
		t.Fatalf("create replacement team: %v", err)
	}
	if replacement.ID != created.ID {
		t.Fatalf("expected deleted network id %d to be reused, got %d", created.ID, replacement.ID)
	}
}

func TestMemoryStorePlayerIDDoesNotLimitWireGuardAddressAllocation(t *testing.T) {
	store := NewMemoryStore(101).(*memoryStore)
	store.nextPlayerID = wireGuardPeerAddressPoolSize + 1
	player, err := store.CreateAdminPlayer(context.Background(), adminCreatePlayerRequest{
		TeamID:      101,
		DisplayName: "High ID Player",
		Email:       "high.id@example.com",
		Password:    "high-id-secret",
		Role:        "member",
	}, time.Now())
	if err != nil {
		t.Fatalf("create high-id player: %v", err)
	}
	if player.ID != wireGuardPeerAddressPoolSize+1 {
		t.Fatalf("unexpected player id %d", player.ID)
	}
	if !strings.HasPrefix(player.WireGuardAddress, "10.70.") {
		t.Fatalf("expected independently allocated WireGuard address, got %q", player.WireGuardAddress)
	}
}

func TestTeamReactivateDeferredClosesNetworkAndHidesPublicServices(t *testing.T) {
	store := NewMemoryStore(101).(*memoryStore)
	ctx := context.Background()

	// Deactivate then reactivate with a deferred play_from_tick (memory has no
	// tick clock, so set the gate explicitly after reactivate).
	if _, err := store.SetTeamActive(ctx, 101, false, time.Now().UTC()); err != nil {
		t.Fatalf("deactivate: %v", err)
	}
	if _, err := store.SetTeamActive(ctx, 101, true, time.Now().UTC()); err != nil {
		t.Fatalf("reactivate: %v", err)
	}
	store.mu.Lock()
	store.teams[101].PlayFromTick = 7
	store.mu.Unlock()

	public, err := store.ListPublicServices(ctx)
	if err != nil {
		t.Fatalf("list public: %v", err)
	}
	for _, byTeam := range public {
		if _, ok := byTeam["101"]; ok {
			t.Fatalf("expected team 101 hidden from public services while deferred, got %+v", public)
		}
	}

	policies, err := store.ListControllerServiceAccessPolicies(ctx)
	if err != nil {
		t.Fatalf("list policies: %v", err)
	}
	found := false
	for _, policy := range policies {
		if policy.TeamID != 101 {
			continue
		}
		found = true
		if !policy.NetworkClosed {
			t.Fatalf("expected network closed for reactivated deferred team, got %+v", policy)
		}
	}
	if !found {
		t.Fatal("expected access policies for team 101")
	}

	store.mu.Lock()
	store.teams[101].PlayFromTick = 0
	store.mu.Unlock()
	public, err = store.ListPublicServices(ctx)
	if err != nil {
		t.Fatalf("list public after open: %v", err)
	}
	seen := false
	for _, byTeam := range public {
		if _, ok := byTeam["101"]; ok {
			seen = true
		}
	}
	if !seen {
		t.Fatalf("expected team 101 public after gate cleared, got %+v", public)
	}
}

func TestMaintenanceRotatesUnlockProofEpoch(t *testing.T) {
	store := NewMemoryStore(101).(*memoryStore)
	ctx := context.Background()

	before, err := store.GetChallengeUnlockProofEpoch(ctx, 1)
	if err != nil || before != 1 {
		t.Fatalf("expected initial epoch 1, got %d err=%v", before, err)
	}
	oldTask, err := store.GetControllerRuntimeTask(ctx, 101, 1)
	if err != nil {
		t.Fatalf("runtime task: %v", err)
	}
	if oldTask.UnlockProofEpoch != 1 {
		t.Fatalf("expected runtime task epoch 1, got %d", oldTask.UnlockProofEpoch)
	}

	// Simulate unlock before maintain.
	if _, err := store.UnlockService(ctx, 101, 1); err != nil {
		t.Fatalf("unlock: %v", err)
	}

	challenge, err := store.SetChallengeMaintenance(ctx, 1, true, time.Now().UTC())
	if err != nil {
		t.Fatalf("maintain: %v", err)
	}
	if challenge.UnlockProofEpoch != 2 {
		t.Fatalf("expected epoch 2 after maintain, got %d", challenge.UnlockProofEpoch)
	}
	services, err := store.ListTeamServices(ctx, 101)
	if err != nil {
		t.Fatalf("list team services: %v", err)
	}
	for _, svc := range services {
		if svc.ChallengeID == 1 && svc.Unlocked {
			t.Fatalf("expected unlock cleared after maintain, got %+v", svc)
		}
	}
	afterTask, err := store.GetControllerRuntimeTask(ctx, 101, 1)
	if err != nil {
		t.Fatalf("runtime task after: %v", err)
	}
	if afterTask.UnlockProofEpoch != 2 {
		t.Fatalf("expected runtime task epoch 2, got %d", afterTask.UnlockProofEpoch)
	}

	rotated, err := store.RotateChallengeUnlockProof(ctx, 1, time.Now().UTC())
	if err != nil {
		t.Fatalf("rotate: %v", err)
	}
	if rotated.UnlockProofEpoch != 3 {
		t.Fatalf("expected epoch 3 after explicit rotate, got %d", rotated.UnlockProofEpoch)
	}
}

func TestMatchPausedBlocksInteractionsAndClosesNetwork(t *testing.T) {
	store := NewMemoryStore(101).(*memoryStore)
	ctx := context.Background()
	store.setMatchPausedForTest(true)

	if err := store.ValidateServiceAction(ctx, 101, 1); !errors.Is(err, ErrMatchPaused) {
		t.Fatalf("expected ErrMatchPaused, got %v", err)
	}
	if _, err := store.UnlockService(ctx, 101, 1); !errors.Is(err, ErrMatchPaused) {
		t.Fatalf("expected unlock blocked while paused, got %v", err)
	}
	if _, err := store.PrepareFactoryResetService(ctx, 101, 1); !errors.Is(err, ErrMatchPaused) {
		t.Fatalf("expected factory reset blocked while paused, got %v", err)
	}
	if _, err := store.RestartService(ctx, 101, 1); !errors.Is(err, ErrMatchPaused) {
		t.Fatalf("expected restart blocked while paused, got %v", err)
	}

	public, err := store.ListPublicServices(ctx)
	if err != nil {
		t.Fatalf("list public: %v", err)
	}
	if len(public) != 0 {
		t.Fatalf("expected no public targets while paused, got %+v", public)
	}

	services, err := store.ListTeamServices(ctx, 101)
	if err != nil {
		t.Fatalf("list team services: %v", err)
	}
	if len(services) == 0 {
		t.Fatal("expected team service cards while paused")
	}
	for _, svc := range services {
		if !svc.Maintenance || svc.Endpoint != "" || svc.LockReason != "match_paused" {
			t.Fatalf("expected locked/stripped team service while paused, got %+v", svc)
		}
	}

	policies, err := store.ListControllerServiceAccessPolicies(ctx)
	if err != nil {
		t.Fatalf("list policies: %v", err)
	}
	for _, policy := range policies {
		if !policy.NetworkClosed {
			t.Fatalf("expected network closed while paused, got %+v", policy)
		}
	}

	store.setMatchPausedForTest(false)
	if err := store.ValidateServiceAction(ctx, 101, 1); err != nil {
		t.Fatalf("expected actions allowed after unpause, got %v", err)
	}
}

func TestChallengeMaintenanceBlocksServiceMutations(t *testing.T) {
	// Direct store/API mutations must refuse while maintained — not only the UI.
	store := NewMemoryStore(101).(*memoryStore)
	ctx := context.Background()

	if _, err := store.SetChallengeMaintenance(ctx, 1, true, time.Now().UTC()); err != nil {
		t.Fatalf("set maintenance: %v", err)
	}

	if err := store.ValidateServiceAction(ctx, 101, 1); !errors.Is(err, ErrChallengeMaintenance) {
		t.Fatalf("expected ErrChallengeMaintenance from validate, got %v", err)
	}
	if _, err := store.UnlockService(ctx, 101, 1); !errors.Is(err, ErrChallengeMaintenance) {
		t.Fatalf("expected unlock blocked, got %v", err)
	}
	if _, err := store.CreateSSHSession(ctx, 101, 1, time.Now().UTC()); !errors.Is(err, ErrChallengeMaintenance) {
		t.Fatalf("expected ssh session blocked, got %v", err)
	}
	if _, err := store.PrepareFactoryResetService(ctx, 101, 1); !errors.Is(err, ErrChallengeMaintenance) {
		t.Fatalf("expected factory reset blocked, got %v", err)
	}
	if _, err := store.RestartService(ctx, 101, 1); !errors.Is(err, ErrChallengeMaintenance) {
		t.Fatalf("expected restart blocked, got %v", err)
	}

	// Deferred resume (play_from_tick) also blocks.
	if _, err := store.SetChallengeMaintenance(ctx, 1, false, time.Now().UTC()); err != nil {
		t.Fatalf("resume: %v", err)
	}
	store.mu.Lock()
	store.challenges[1].PlayFromTick = 99
	store.mu.Unlock()
	if _, err := store.UnlockService(ctx, 101, 1); !errors.Is(err, ErrChallengeDeferred) {
		t.Fatalf("expected unlock deferred, got %v", err)
	}
}

func TestChallengeMaintenanceClosesNetworkAfterWarmRedeployPolicy(t *testing.T) {
	// Organizer workflow: Maintain → redeploy newer image → reconcile.
	// NetworkClosed stays true (participants dark); organizer WG remains allowlisted.
	store := NewMemoryStore(101).(*memoryStore)
	ctx := context.Background()

	challenge, err := store.CreateAdminChallenge(ctx, adminCreateChallengeRequest{
		Name:          "sealbroker",
		BaselineImage: "registry.local/seal:baseline",
		CheckerImage:  "registry.local/seal-checker:latest",
	}, time.Now().UTC())
	if err != nil {
		t.Fatalf("create challenge: %v", err)
	}
	store.mu.Lock()
	store.challenges[challenge.ID].Published = true
	store.teamStates[101] = map[int]*serviceState{
		challenge.ID: {
			ChallengeID: challenge.ID,
			TeamID:      101,
			Name:        challenge.Name,
			Endpoint:    "10.80.1.12:8160",
			Status:      "stable",
			Checker:     "passing",
			Unlocked:    true,
		},
	}
	// Seed organizer + team player WireGuard peers.
	store.players[900] = mustNewAdminPlayerRecord(900, 0, "Organizer", "Admin", "admin@example.com", "organizer", "admin-secret", time.Now().UTC())
	store.players[900].WireGuard.Address = "10.70.0.2"
	store.players[900].WireGuard.Status = "active"
	store.players[1].WireGuard.Address = "10.70.11.20"
	store.players[1].WireGuard.Status = "active"
	store.mu.Unlock()

	if _, err := store.SetChallengeMaintenance(ctx, challenge.ID, true, time.Now().UTC()); err != nil {
		t.Fatalf("set maintenance: %v", err)
	}

	// Simulate warm redeploy: instance endpoint still present under maintenance.
	policies, err := store.ListControllerServiceAccessPolicies(ctx)
	if err != nil {
		t.Fatalf("list policies: %v", err)
	}
	var found *ControllerServiceAccessPolicy
	for i := range policies {
		if policies[i].ChallengeID == challenge.ID && policies[i].TeamID == 101 {
			found = &policies[i]
			break
		}
	}
	if found == nil {
		t.Fatal("expected access policy for maintained challenge")
	}
	if !found.NetworkClosed {
		t.Fatalf("expected NetworkClosed during maintenance warm redeploy, got %+v", found)
	}
	if !found.SSHUnlocked {
		t.Fatalf("expected SSH unlocked for organizer allowlist while maintained, got %+v", found)
	}
	if len(found.AllowedPeerAddresses) != 1 || found.AllowedPeerAddresses[0] != "10.70.0.2" {
		t.Fatalf("expected only organizer peer while maintained, got %+v", found.AllowedPeerAddresses)
	}
	if found.ServiceIP != "10.80.1.12" || found.ServicePort != 8160 {
		t.Fatalf("expected endpoint preserved for controller rules, got %+v", found)
	}

	public, err := store.ListPublicServices(ctx)
	if err != nil {
		t.Fatalf("list public: %v", err)
	}
	if _, ok := public[fmt.Sprintf("%d", challenge.ID)]; ok {
		t.Fatalf("expected no public endpoints while maintained, got %+v", public)
	}
}

func TestDeferredResumeClosesNetworkAndHidesPublicServices(t *testing.T) {
	store := NewMemoryStore(101).(*memoryStore)
	ctx := context.Background()

	challenge, err := store.CreateAdminChallenge(ctx, adminCreateChallengeRequest{
		Name:          "deferred-svc",
		BaselineImage: "registry.local/def:baseline",
		CheckerImage:  "registry.local/def-checker:latest",
	}, time.Now().UTC())
	if err != nil {
		t.Fatalf("create challenge: %v", err)
	}

	// Seed one published team service endpoint with deferred play_from_tick.
	store.mu.Lock()
	store.challenges[challenge.ID].Published = true
	store.challenges[challenge.ID].PlayFromTick = 42
	store.teamStates[1] = map[int]*serviceState{
		challenge.ID: {
			ChallengeID: challenge.ID,
			TeamID:      1,
			Name:        challenge.Name,
			Endpoint:    "10.80.1.11:10001",
			Status:      "stable",
			Checker:     "passing",
		},
	}
	store.mu.Unlock()

	blocked, err := store.ChallengePlayBlocked(ctx, challenge.ID)
	if err != nil || !blocked {
		t.Fatalf("expected play blocked during deferred resume, blocked=%v err=%v", blocked, err)
	}

	policies, err := store.ListControllerServiceAccessPolicies(ctx)
	if err != nil {
		t.Fatalf("list policies: %v", err)
	}
	var found *ControllerServiceAccessPolicy
	for i := range policies {
		if policies[i].ChallengeID == challenge.ID {
			found = &policies[i]
			break
		}
	}
	if found == nil {
		t.Fatalf("expected access policy for deferred challenge")
	}
	if !found.NetworkClosed {
		t.Fatalf("expected network closed during deferred resume, got %+v", found)
	}
	// No organizers seeded → no allowlisted peers (participants stay dark).
	if len(found.AllowedPeerAddresses) > 0 {
		for _, peer := range found.AllowedPeerAddresses {
			// Only organizers are allowed while closed; default store has none.
			_ = peer
		}
		// Default memory players are captains, not organizers.
		t.Fatalf("expected no participant peers while closed, got %+v", found.AllowedPeerAddresses)
	}

	public, err := store.ListPublicServices(ctx)
	if err != nil {
		t.Fatalf("list public services: %v", err)
	}
	if _, ok := public[fmt.Sprintf("%d", challenge.ID)]; ok {
		t.Fatalf("expected no public endpoints while deferred, got %+v", public)
	}

	// Clear deferred gate → play open.
	store.mu.Lock()
	store.challenges[challenge.ID].PlayFromTick = 0
	store.mu.Unlock()

	blocked, err = store.ChallengePlayBlocked(ctx, challenge.ID)
	if err != nil || blocked {
		t.Fatalf("expected play open after gate cleared, blocked=%v err=%v", blocked, err)
	}
	public, err = store.ListPublicServices(ctx)
	if err != nil {
		t.Fatalf("list public services after open: %v", err)
	}
	if _, ok := public[fmt.Sprintf("%d", challenge.ID)]; !ok {
		t.Fatalf("expected public endpoints after open, got %+v", public)
	}
}
