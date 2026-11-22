package apigateway

import (
	"context"
	"errors"
	"slices"
	"strconv"
	"testing"
	"time"

	"adplatform/internal/platform/database"
	"adplatform/internal/platform/database/dbtest"
)

func TestPostgresChallengeDeploymentLifecycle(t *testing.T) {
	db := dbtest.IsolatedSchema(t, "TEST_POSTGRES_DSN", "challenge_lifecycle_test")
	ctx := context.Background()
	if err := database.RunEmbeddedMigrationsWithOptions(ctx, db, database.MigrationOptions{}); err != nil {
		t.Fatal(err)
	}
	store := NewPostgresStore(db)
	now := time.Now().UTC().Truncate(time.Second)
	challenge, err := store.CreateAdminChallenge(ctx, adminCreateChallengeRequest{
		Name: "Deployment Test", ServicePort: 8010, ServiceSubnetOctet: 50,
	}, now)
	if err != nil || challenge.Published || challenge.RuntimeStatus != "draft" || challenge.BaselineImage == "" || challenge.CheckerImage == "" {
		t.Fatalf("create draft challenge: %+v, %v", challenge, err)
	}
	if _, err := store.CreateAdminChallenge(ctx, adminCreateChallengeRequest{Name: "deployment test"}, now); !errors.Is(err, ErrDuplicateResource) {
		t.Fatalf("duplicate challenge returned %v", err)
	}
	if _, err := store.CreateAdminChallenge(ctx, adminCreateChallengeRequest{Name: "Duplicate Subnet", ServiceSubnetOctet: 50}, now); !errors.Is(err, ErrInvalidRuntimeConfig) {
		t.Fatalf("duplicate subnet returned %v", err)
	}
	if err := store.SaveAdminChallengeValidation(ctx, ChallengeValidationResult{
		ChallengeID: challenge.ID, Status: "passed", CheckedAt: now.Format(time.RFC3339),
		BaselineSSHContractOK: true, CheckerContractOK: true, ServiceStateContractOK: true,
	}); err != nil {
		t.Fatal(err)
	}
	challenges, err := store.ListAdminChallenges(ctx)
	if err != nil || len(challenges) != 1 || challenges[0].LastValidation == nil || !challenges[0].LastValidation.CheckerContractOK {
		t.Fatalf("stored validation was missing: %+v, %v", challenges, err)
	}
	updated, err := store.UpdateAdminChallenge(ctx, challenge.ID, adminUpdateChallengeRequest{
		Name: challenge.Name, BaselineImage: "example.test/runtime:v2", CheckerImage: "example.test/checker:v2", EgressEnabled: new(false),
	})
	if err != nil || updated.LastValidation != nil || updated.EgressEnabled || updated.BaselineImage != "example.test/runtime:v2" {
		t.Fatalf("update challenge: %+v, %v", updated, err)
	}
	emptyDeployment, err := store.DeployAdminChallenge(ctx, challenge.ID)
	if err != nil || !emptyDeployment.Published || emptyDeployment.JobID != 0 || emptyDeployment.Status != "completed" {
		t.Fatalf("publish before teams are registered: %+v, %v", emptyDeployment, err)
	}
	team, err := store.CreateAdminTeam(ctx, adminCreateTeamRequest{Name: "First Team", ContactEmail: "first@example.test"})
	if err != nil || team.DeployedChallenges != 1 {
		t.Fatalf("create team with published challenge: %+v, %v", team, err)
	}
	initialTask, err := store.GetControllerRuntimeTask(ctx, team.ID, challenge.ID)
	if err != nil || initialTask.DeploymentJobID != 0 || initialTask.CheckerToken == "" || initialTask.ServicePort != 8010 {
		t.Fatalf("expected a standalone runtime task on port 8010, got error %v and port %d", err, initialTask.ServicePort)
	}
	if result, err := store.ReconcileAdminDeployments(ctx, now); err != nil || result.ProcessedJobs != 0 || result.ProcessedInstances != 1 {
		t.Fatalf("reconcile standalone runtime: %+v, %v", result, err)
	}
	first, err := store.DeployAdminChallenge(ctx, challenge.ID)
	if err != nil || first.JobID == 0 || first.QueuedTeamCount != 1 {
		t.Fatalf("queue first redeploy: %+v, %v", first, err)
	}
	if err := store.DeleteAdminDeployment(ctx, first.JobID); !errors.Is(err, ErrDeploymentActive) {
		t.Fatalf("deleting a queued deployment returned %v", err)
	}
	second, err := store.DeployAdminChallenge(ctx, challenge.ID)
	if err != nil || second.JobID <= first.JobID || second.QueuedTeamCount != 1 {
		t.Fatalf("queue replacement deploy: %+v, %v", second, err)
	}
	tasks, err := store.ListControllerRuntimeTasks(ctx)
	if err != nil || len(tasks) != 1 || tasks[0].TeamID != team.ID || tasks[0].DeploymentJobID != second.JobID || tasks[0].CheckerToken == initialTask.CheckerToken {
		t.Fatalf("expected one replacement task with a new checker credential, got %d tasks and error %v", len(tasks), err)
	}
	if result, err := store.ReconcileAdminDeployments(ctx, now); err != nil || result.CompletedJobs != 1 || result.ProcessedInstances != 1 {
		t.Fatalf("reconcile replacement deployment: %+v, %v", result, err)
	}
	jobs, err := store.ListAdminDeployments(ctx)
	if err != nil || len(jobs) != 2 {
		t.Fatalf("list deployment jobs: %+v, %v", jobs, err)
	}
	states := map[int]string{jobs[0].ID: jobs[0].Status, jobs[1].ID: jobs[1].Status}
	if states[first.JobID] != "superseded" || states[second.JobID] != "completed" {
		t.Fatalf("unexpected job states %v", states)
	}
	if err := store.DeleteAdminDeployment(ctx, second.JobID); err != nil {
		t.Fatalf("delete completed deployment: %v", err)
	}
	if err := store.DeleteAdminDeployment(ctx, second.JobID); !errors.Is(err, ErrDeploymentNotFound) {
		t.Fatalf("delete a missing deployment returned %v", err)
	}

	if public, err := store.ListChallenges(ctx); err != nil || len(public) != 0 {
		t.Fatalf("prematch challenge catalog: %+v, %v", public, err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO game_match_state(singleton,state) VALUES(TRUE,'running'); INSERT INTO game_ticks(id,status) VALUES(1,'completed')`); err != nil {
		t.Fatal(err)
	}
	assertEndpoint := func(want bool) {
		t.Helper()
		services, err := store.ListPublicServices(ctx)
		endpoints := services[strconv.Itoa(challenge.ID)][strconv.Itoa(team.ID)]
		if err != nil || slices.Contains(endpoints, initialTask.Endpoint) != want {
			t.Fatalf("endpoint visibility, want %t: %v, %v", want, services, err)
		}
	}
	assertEndpoint(true)
	if _, err := store.CreateSSHSession(ctx, team.ID, challenge.ID, now); !errors.Is(err, ErrServiceLocked) {
		t.Fatalf("locked service credential request returned %v", err)
	}
	if _, err := store.UnlockService(ctx, team.ID, challenge.ID); !errors.Is(err, ErrServiceLocked) {
		t.Fatalf("gateway must not create a grant without a committed capture: %v", err)
	}
	if _, err := db.ExecContext(ctx, `UPDATE team_service_states SET unlocked = TRUE WHERE team_id = $1 AND challenge_id = $2`, team.ID, challenge.ID); err != nil {
		t.Fatal(err)
	}
	if grant, err := store.UnlockService(ctx, team.ID, challenge.ID); err != nil || !grant.Unlocked {
		t.Fatalf("record a valid service unlock: %+v, %v", grant, err)
	}
	if session, err := store.CreateSSHSession(ctx, team.ID, challenge.ID, now); err != nil || session.Host != initialTask.SSHHost || session.Port != 22 {
		t.Fatalf("read unlocked service SSH connection: %v", err)
	}
	maintenance, err := store.SetChallengeMaintenance(ctx, challenge.ID, true, now)
	if err != nil || !maintenance.Maintenance {
		t.Fatalf("enter maintenance: %+v, %v", maintenance, err)
	}
	assertEndpoint(false)
	services, err := store.ListTeamServices(ctx, team.ID)
	if err != nil || len(services) != 1 || services[0].Unlocked || services[0].LockReason != "maintenance" {
		t.Fatalf("maintenance participant view: %+v, %v", services, err)
	}
	if policy, err := store.GetControllerServiceAccessPolicy(ctx, team.ID, challenge.ID); err != nil || !policy.NetworkClosed || policy.SSHUnlocked {
		t.Fatalf("maintenance access policy: %+v, %v", policy, err)
	}
	if resumed, err := store.SetChallengeMaintenance(ctx, challenge.ID, false, now); err != nil || resumed.Maintenance || resumed.PlayFromTick != 2 {
		t.Fatalf("resume at the next tick: %+v, %v", resumed, err)
	}
	assertEndpoint(false)
	if _, err := db.ExecContext(ctx, `INSERT INTO game_ticks(id,status) VALUES(2,'completed')`); err != nil {
		t.Fatal(err)
	}
	assertEndpoint(true)
	if blocked, err := store.ChallengePlayBlocked(ctx, challenge.ID); err != nil || blocked {
		t.Fatalf("challenge should be playable at the resume tick, blocked = %t, error = %v", blocked, err)
	}
	policies, err := store.ListControllerServiceAccessPolicies(ctx)
	if err != nil || len(policies) != 1 || policies[0].NetworkClosed || policies[0].SSHUnlocked {
		t.Fatalf("resumed challenge policy before a fresh grant: %+v, %v", policies, err)
	}
	if _, err := db.ExecContext(ctx, `UPDATE team_service_states SET unlocked = TRUE WHERE team_id = $1 AND challenge_id = $2`, team.ID, challenge.ID); err != nil {
		t.Fatal(err)
	}
	if grant, err := store.UnlockService(ctx, team.ID, challenge.ID); err != nil || !grant.Unlocked {
		t.Fatalf("grant service access after resumption: %+v, %v", grant, err)
	}
	participant, err := store.CreateAdminPlayer(ctx, adminCreatePlayerRequest{
		TeamID: team.ID, DisplayName: "Finished Match Participant", Email: "finished-participant@example.test",
		Password: "participant-password", Role: "member",
	}, now)
	if err != nil {
		t.Fatal(err)
	}
	organizer, err := store.CreateAdminPlayer(ctx, adminCreatePlayerRequest{
		DisplayName: "Finished Match Organizer", Email: "finished-organizer@example.test",
		Password: "organizer-password", Role: "organizer",
	}, now)
	if err != nil {
		t.Fatal(err)
	}
	peers, err := store.ListWireGuardGatewayPeers(ctx)
	if err != nil || !slices.ContainsFunc(peers, func(peer WireGuardGatewayPeer) bool { return peer.PlayerID == participant.ID }) {
		t.Fatalf("active match participant peer missing: %+v, %v", peers, err)
	}
	if _, err := db.ExecContext(ctx, `UPDATE game_match_state SET state = 'finished' WHERE singleton = TRUE`); err != nil {
		t.Fatal(err)
	}
	peers, err = store.ListWireGuardGatewayPeers(ctx)
	if err != nil || slices.ContainsFunc(peers, func(peer WireGuardGatewayPeer) bool { return peer.PlayerID == participant.ID }) ||
		!slices.ContainsFunc(peers, func(peer WireGuardGatewayPeer) bool { return peer.PlayerID == organizer.ID }) {
		t.Fatalf("finished match should retain only organizer peers: %+v, %v", peers, err)
	}
	finishedPolicy, err := store.GetControllerServiceAccessPolicy(ctx, team.ID, challenge.ID)
	if err != nil || !finishedPolicy.NetworkClosed || finishedPolicy.MatchPaused || !finishedPolicy.MatchFinished {
		t.Fatalf("finished match must close service access: %+v, %v", finishedPolicy, err)
	}
	finishedPolicies, err := store.ListControllerServiceAccessPolicies(ctx)
	if err != nil || len(finishedPolicies) != 1 || !finishedPolicies[0].NetworkClosed || !finishedPolicies[0].MatchFinished {
		t.Fatalf("finished match must close global service access: %+v, %v", finishedPolicies, err)
	}
	if _, err := db.ExecContext(ctx, `UPDATE game_match_state SET state = 'running', scheduled_end_at = NOW() - INTERVAL '1 second' WHERE singleton = TRUE`); err != nil {
		t.Fatal(err)
	}
	endedBySchedule, err := store.GetControllerServiceAccessPolicy(ctx, team.ID, challenge.ID)
	if err != nil || !endedBySchedule.NetworkClosed || endedBySchedule.MatchFinished {
		t.Fatalf("elapsed match schedule must close service access: %+v, %v", endedBySchedule, err)
	}
	peers, err = store.ListWireGuardGatewayPeers(ctx)
	if err != nil || slices.ContainsFunc(peers, func(peer WireGuardGatewayPeer) bool { return peer.PlayerID == participant.ID }) {
		t.Fatalf("elapsed match schedule must remove participant peer: %+v, %v", peers, err)
	}
}
