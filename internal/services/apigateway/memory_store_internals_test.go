package apigateway

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"
)

func newInternalMemoryStore(t *testing.T) *memoryStore {
	t.Helper()
	clearWireGuardEnv(t)
	return NewMemoryStore(101).(*memoryStore)
}

func TestMemoryStoreSeedsAndInconsistentRecords(t *testing.T) {
	ctx := context.Background()
	now := time.Now()
	if seeded := newInternalMemoryStore(t); seeded.teams[101] == nil {
		t.Fatal("team 101 is seeded")
	}
	store := NewMemoryStore(150).(*memoryStore)
	if store.teams[150] == nil || len(store.instances[150]) == 0 {
		t.Fatal("an unseeded team ID gets a team with ready services")
	}

	// Seeding skips unpublished challenges.
	store.challenges[3].Published = false
	store.seedReadyServicesLocked(150, now)
	if _, ok := store.instances[150][3]; ok {
		t.Error("an unpublished challenge gets no instance")
	}
	store.teamStates[150][3] = defaultServiceStateForConfig(3, 150, "storage", DefaultServicePort(3), 3)
	if states, _ := store.ListTeamServices(ctx, 150); len(states) != 2 {
		t.Errorf("an unpublished challenge's state is hidden: %+v", states)
	}
	expectErr(t, "an unpublished challenge", store.ValidateServiceAction(ctx, 150, 3), ErrChallengeNotFound)
	store.challenges[3].Published = true
	expectErr(t, "a state with no instance", store.ValidateServiceAction(ctx, 150, 3), ErrChallengeNotFound)

	// A legacy unsalted hash is upgraded on login.
	store.players[1].PasswordHash = hashSecret("alpha-secret")
	if _, err := store.AuthenticatePlayer(ctx, "alpha.captain@example.com", "alpha-secret"); err != nil {
		t.Fatal(err)
	}
	if passwordHashNeedsUpgrade(store.players[1].PasswordHash) {
		t.Error("a verified legacy hash is replaced")
	}

	// A player whose team record vanished cannot edit the team profile.
	delete(store.teams, 102)
	_, err := store.UpdateParticipantProfile(ctx, 2, participantUpdateProfileRequest{DisplayName: "D", Email: "delta.captain@example.com"})
	expectErr(t, "profile with a missing team", err, ErrTeamNotFound)

	for label, call := range map[string]func() error{
		"ssh apply failure":      func() error { return store.MarkSSHSessionApplyFailure(ctx, 999, 1) },
		"reset failure":          func() error { return store.MarkFactoryResetFailure(ctx, 999, 1) },
		"reset completion":       func() error { return store.MarkFactoryResetCompletionFailure(ctx, 999, 1) },
		"abort an unknown reset": func() error { return store.AbortFactoryResetService(ctx, resetData{TeamID: 999, ChallengeID: 1}) },
	} {
		expectErr(t, label, call(), ErrChallengeNotFound)
	}

	if _, err := store.PrepareFactoryResetService(ctx, 101, 1); err != nil {
		t.Fatal(err)
	}
	expectErr(t, "restart during a reset", store.ValidateRestartService(ctx, 101, 1), ErrServiceUnavailable)
	store.instances[101][1] = nil
	delete(store.instances[101], 1)
	expectErr(t, "abort with no instance", store.AbortFactoryResetService(ctx, resetData{TeamID: 101, ChallengeID: 1}), ErrServiceUnavailable)

	// An organizer can never be handed captaincy, even when filed on a team.
	store.players[4].Player.TeamID = 101
	store.players[4].Player.Role = "organizer"
	_, err = store.TransferTeamCaptain(ctx, 1, 4)
	expectErr(t, "captaincy to an organizer", err, ErrInvalidCaptainTransfer)
}

func TestMemoryStoreListingsSkipInactivePeers(t *testing.T) {
	ctx := context.Background()
	now := time.Now()
	store := newInternalMemoryStore(t)
	organizer, err := store.CreateAdminPlayer(ctx, adminCreatePlayerRequest{DisplayName: "Org", Email: "org@example.test", Password: "organizer-password", Role: "organizer"}, now)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.RegisterPlayer(ctx, participantRegisterRequest{DisplayName: "Solo", Email: "solo@example.test", Password: "solo-password"}, now); err != nil {
		t.Fatal(err)
	}
	if _, err := store.SetPlayerActive(ctx, 2, false, now); err != nil {
		t.Fatal(err)
	}
	if _, err := store.SetTeamActive(ctx, 103, false, now); err != nil {
		t.Fatal(err)
	}
	if _, err := store.RevokeAdminPlayerWireGuardConfig(ctx, 4, now); err != nil {
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
	if _, ok := seen[2]; ok {
		t.Error("a deactivated player has no gateway peer")
	}
	if _, ok := seen[3]; ok {
		t.Error("a deactivated team's member has no gateway peer")
	}
	if peer, ok := seen[organizer.ID]; !ok || peer.TeamName != "Organizer" {
		t.Errorf("the organizer keeps a peer on the organizer team: %+v", peer)
	}

	if services, err := store.ListPublicServices(ctx); err != nil {
		t.Fatal(err)
	} else {
		for name := range services {
			if name == "Team Sigma" {
				t.Error("a deactivated team's services are not public")
			}
		}
	}

	if _, err := store.UnlockService(ctx, 102, 1); err != nil {
		t.Fatal(err)
	}
	policies, err := store.ListControllerServiceAccessPolicies(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, policy := range policies {
		if policy.TeamID == 102 && policy.ChallengeID == 1 && len(policy.AllowedPeerAddresses) != 1 {
			t.Errorf("only the organizer may reach a deactivated captain's service: %+v", policy)
		}
	}

	// Queued instances sort by deployment job, then challenge, then team.
	for teamID := range store.instances {
		for _, instance := range store.instances[teamID] {
			instance.RuntimeStatus = "queued"
		}
	}
	store.instances[101][2].DeploymentJobID = 7
	tasks, err := store.ListControllerRuntimeTasks(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for i := 1; i < len(tasks); i++ {
		a, b := tasks[i-1], tasks[i]
		if a.DeploymentJobID > b.DeploymentJobID || (a.DeploymentJobID == b.DeploymentJobID && (a.ChallengeID > b.ChallengeID || (a.ChallengeID == b.ChallengeID && a.TeamID > b.TeamID))) {
			t.Fatalf("tasks out of order at %d: %+v then %+v", i, a, b)
		}
	}
}

func TestMemoryStoreDeploymentCountsNeverGoNegative(t *testing.T) {
	ctx := context.Background()
	store := newInternalMemoryStore(t)
	created, err := store.CreateAdminChallenge(ctx, adminCreateChallengeRequest{Name: "counted", BaselineImage: "i", CheckerImage: "c"}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := store.DeployAdminChallenge(ctx, created.ID)
	if err != nil {
		t.Fatal(err)
	}
	store.deployments[deployment.JobID].QueuedTeamCount = 0
	if _, err := store.ReconcileAdminDeployments(ctx, time.Now()); err != nil {
		t.Fatal(err)
	}
	if count := store.deployments[deployment.JobID].QueuedTeamCount; count != 0 {
		t.Errorf("queued count clamps at zero, got %d", count)
	}
}

func TestMemoryStoreAddressPoolsAndServerKey(t *testing.T) {
	ctx := context.Background()
	now := time.Now()
	store := newInternalMemoryStore(t)
	joiner, err := store.RegisterPlayer(ctx, participantRegisterRequest{DisplayName: "Joiner", Email: "joiner@example.test", Password: "joiner-password"}, now)
	if err != nil {
		t.Fatal(err)
	}

	// Without a server key outside dry-run memory mode, no peer can be issued.
	t.Setenv("API_GATEWAY_STATE_BACKEND", "postgres")
	_, err = store.CreateAdminPlayer(ctx, adminCreatePlayerRequest{TeamID: 101, DisplayName: "K", Email: "k@example.test", Password: "keyless-password"}, now)
	if err == nil {
		t.Error("creating a team player needs a server key")
	}
	if _, err := store.JoinExistingPlayerTeam(ctx, joiner.PlayerID, "TEAM-DELTA-JOIN", now); err == nil {
		t.Error("joining a team needs a server key")
	}
	if _, err := store.GetAdminPlayerWireGuardConfig(ctx, 1); err == nil {
		t.Error("refreshing a peer needs a server key")
	}
	if _, err := store.RotateAdminPlayerWireGuardConfig(ctx, 1, now); err == nil {
		t.Error("rotating a peer needs a server key")
	}
	func() {
		defer func() {
			if recover() == nil {
				t.Error("seeding players without a server key is a startup error")
			}
		}()
		NewMemoryStore(101)
	}()
	t.Setenv("API_GATEWAY_STATE_BACKEND", "")

	// Every team network ID in use: no more teams.
	for id := minimumTeamNetworkID; id <= maximumTeamNetworkID; id++ {
		if store.teams[id] == nil {
			store.teams[id] = &adminTeam{ID: id, Name: fmt.Sprintf("Filler %d", id), ContactEmail: fmt.Sprintf("filler-%d@example.test", id)}
		}
	}
	_, err = store.CreateAdminTeam(ctx, adminCreateTeamRequest{Name: "One Too Many", ContactEmail: "too-many@example.test"})
	expectErr(t, "team network pool", err, ErrTeamAddressPool)

	// Every peer address in use: no more peers.
	for index := 0; index < wireGuardPeerAddressPoolSize; index++ {
		store.players[100000+index] = &adminPlayerRecord{Player: adminPlayer{ID: 100000 + index}, WireGuard: wireGuardPeerState{Address: wireGuardPeerAddressAt(index)}}
	}
	_, err = store.CreateAdminPlayer(ctx, adminCreatePlayerRequest{TeamID: 101, DisplayName: "Full", Email: "full@example.test", Password: "full-password"}, now)
	expectErr(t, "peer address pool on create", err, ErrPlayerAddressPool)
	_, err = store.JoinExistingPlayerTeam(ctx, joiner.PlayerID, "TEAM-DELTA-JOIN", now)
	if !errors.Is(err, ErrPlayerAddressPool) && !errors.Is(err, ErrTeamMemberLimit) {
		t.Errorf("peer address pool on join: %v", err)
	}
}

func TestMemoryStoreUnlimitedTeamsAndUnpublishedServices(t *testing.T) {
	ctx := context.Background()
	store := newInternalMemoryStore(t)
	store.platformSettings.MaxTeamMembers = 0
	if store.teamMemberLimitReachedLocked(101) {
		t.Error("a zero limit means unlimited")
	}
	store.challenges[2].Published = false
	services, err := store.ListPublicServices(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, perChallenge := range services {
		if _, ok := perChallenge["chat"]; ok {
			t.Error("an unpublished challenge is not public")
		}
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
}
