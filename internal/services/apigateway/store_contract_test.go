package apigateway

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"slices"
	"testing"
	"time"

	"adplatform/internal/platform/database"
	"adplatform/internal/platform/database/dbtest"
)

// contractStore is a store plus the hooks for state its API cannot set.
type contractStore struct {
	Store
	setMatch       func(started, paused bool)
	deferTeam      func(teamID int, deferred bool)
	deferChallenge func(challengeID int, deferred bool)
	setRuntime     func(teamID, challengeID int, status string)
	dropTeamState  func(teamID, challengeID int)
	publish        func(challengeID int)
	db             *sql.DB // Postgres only
}

func memoryContractStore(t *testing.T) contractStore {
	t.Helper()
	t.Setenv("WIREGUARD_SERVER_PUBLIC_KEY", "contract-server-public-key")
	store := NewMemoryStore(101).(*memoryStore)
	return contractStore{
		Store: store,
		setMatch: func(started, paused bool) {
			store.setMatchStartedForTest(started)
			store.setMatchPausedForTest(paused)
		},
		deferTeam: func(teamID int, deferred bool) {
			store.mu.Lock()
			defer store.mu.Unlock()
			store.teams[teamID].PlayFromTick = map[bool]int{true: 99}[deferred]
		},
		deferChallenge: func(challengeID int, deferred bool) {
			store.mu.Lock()
			defer store.mu.Unlock()
			store.challenges[challengeID].PlayFromTick = map[bool]int{true: 99}[deferred]
		},
		setRuntime: func(teamID, challengeID int, status string) {
			store.mu.Lock()
			defer store.mu.Unlock()
			store.instances[teamID][challengeID].RuntimeStatus = status
		},
		dropTeamState: func(teamID, challengeID int) {
			store.mu.Lock()
			defer store.mu.Unlock()
			delete(store.teamStates[teamID], challengeID)
		},
		publish: func(int) {},
	}
}

func postgresContractStore(t *testing.T) contractStore {
	t.Helper()
	db := dbtest.IsolatedSchema(t, "TEST_POSTGRES_DSN", "store_contract_test")
	t.Setenv("WIREGUARD_SERVER_PUBLIC_KEY", "contract-server-public-key")
	if err := database.RunEmbeddedMigrationsWithOptions(context.Background(), db, database.MigrationOptions{IncludeSeeds: false}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	exec := func(statement string, args ...any) {
		t.Helper()
		if _, err := db.Exec(statement, args...); err != nil {
			t.Fatal(err)
		}
	}
	deferral := func(deferred bool) sql.NullInt64 { return sql.NullInt64{Int64: 99, Valid: deferred} }
	return contractStore{
		db:    db,
		Store: NewPostgresStore(db),
		setMatch: func(started, paused bool) {
			state := "not_started"
			if started {
				state = "running"
			}
			if paused {
				state = "paused"
			}
			exec(`INSERT INTO game_match_state(singleton, state) VALUES (TRUE, $1) ON CONFLICT (singleton) DO UPDATE SET state = EXCLUDED.state`, state)
		},
		deferTeam: func(teamID int, deferred bool) {
			exec(`UPDATE teams SET play_from_tick = $2 WHERE id = $1`, teamID, deferral(deferred))
		},
		deferChallenge: func(challengeID int, deferred bool) {
			exec(`UPDATE challenges SET play_from_tick = $2 WHERE id = $1`, challengeID, deferral(deferred))
		},
		setRuntime: func(teamID, challengeID int, status string) {
			exec(`UPDATE service_instances SET runtime_status = $3 WHERE team_id = $1 AND challenge_id = $2`, teamID, challengeID, status)
		},
		dropTeamState: func(teamID, challengeID int) {
			exec(`DELETE FROM team_service_states WHERE team_id = $1 AND challenge_id = $2`, teamID, challengeID)
		},
		publish: func(challengeID int) { exec(`UPDATE challenges SET published = TRUE WHERE id = $1`, challengeID) },
	}
}

func forEachStore(t *testing.T, run func(t *testing.T, s contractStore)) {
	for name, build := range map[string]func(*testing.T) contractStore{"memory": memoryContractStore, "postgres": postgresContractStore} {
		t.Run(name, func(t *testing.T) { run(t, build(t)) })
	}
}

func expectErr(t *testing.T, label string, err, want error) {
	t.Helper()
	if !errors.Is(err, want) {
		t.Errorf("%s: want %v, got %v", label, want, err)
	}
}

type contractWorld struct {
	teamA, teamB    adminTeam
	challenge       adminChallenge
	captain, member adminPlayer
	other           adminPlayer
	organizer       adminPlayer
}

// buildWorld creates two teams, a deployed and published challenge, and
// players on team A, then starts the match.
func buildWorld(t *testing.T, s contractStore) contractWorld {
	t.Helper()
	ctx := context.Background()
	now := time.Now()
	var w contractWorld
	var err error
	must := func(e error) {
		t.Helper()
		if e != nil {
			t.Fatal(e)
		}
	}
	w.teamA, err = s.CreateAdminTeam(ctx, adminCreateTeamRequest{Name: "Contract A", ContactEmail: "contract-a@example.test"})
	must(err)
	w.teamB, err = s.CreateAdminTeam(ctx, adminCreateTeamRequest{Name: "Contract B", ContactEmail: "contract-b@example.test"})
	must(err)
	w.challenge, err = s.CreateAdminChallenge(ctx, adminCreateChallengeRequest{Name: "contract-web", BaselineImage: "img/web", CheckerImage: "img/checker", ServiceSubnetOctet: 77}, now)
	must(err)
	_, err = s.DeployAdminChallenge(ctx, w.challenge.ID)
	must(err)
	s.publish(w.challenge.ID)
	_, err = s.ReconcileAdminDeployments(ctx, now)
	must(err)
	w.captain, err = s.CreateAdminPlayer(ctx, adminCreatePlayerRequest{TeamID: w.teamA.ID, DisplayName: "Cap", Email: "contract-cap@example.test", Password: "contract-password", Role: "captain"}, now)
	must(err)
	w.member, err = s.CreateAdminPlayer(ctx, adminCreatePlayerRequest{TeamID: w.teamA.ID, DisplayName: "Mem", Email: "contract-mem@example.test", Password: "contract-password", Role: "member"}, now)
	must(err)
	w.other, err = s.CreateAdminPlayer(ctx, adminCreatePlayerRequest{TeamID: w.teamB.ID, DisplayName: "Oth", Email: "contract-oth@example.test", Password: "contract-password", Role: "captain"}, now)
	must(err)
	w.organizer, err = s.CreateAdminPlayer(ctx, adminCreatePlayerRequest{DisplayName: "Org", Email: "contract-org@example.test", Password: "contract-password", Role: "organizer"}, now)
	must(err)
	s.setMatch(true, false)
	return w
}

func TestStoreContractDeferredTeamPeers(t *testing.T) {
	forEachStore(t, func(t *testing.T, s contractStore) {
		ctx := context.Background()
		w := buildWorld(t, s)
		s.deferTeam(w.teamA.ID, true)
		hasCaptainPeer := func() bool {
			peers, err := s.ListWireGuardGatewayPeers(ctx)
			if err != nil {
				t.Fatal(err)
			}
			return slices.ContainsFunc(peers, func(peer WireGuardGatewayPeer) bool {
				return peer.PlayerID == w.captain.ID
			})
		}
		if hasCaptainPeer() {
			t.Fatal("a deferred team has an active participant peer")
		}
		if s.db == nil {
			s.deferTeam(w.teamA.ID, false)
		} else if _, err := s.db.ExecContext(ctx, `INSERT INTO game_ticks(id,status) VALUES(99,'running')`); err != nil {
			t.Fatal(err)
		}
		if !hasCaptainPeer() {
			t.Fatal("the peer should be active after deferral ends")
		}
	})
}

func TestStoreContractTeamJoinKeyRotation(t *testing.T) {
	forEachStore(t, func(t *testing.T, s contractStore) {
		ctx := context.Background()
		now := time.Now()
		w := buildWorld(t, s)
		player, err := s.RegisterPlayer(ctx, participantRegisterRequest{DisplayName: "Solo", Email: "rotation-solo@example.test", Password: "contract-password"}, now)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := s.SetTeamActive(ctx, w.teamA.ID, false, now); err != nil {
			t.Fatal(err)
		}
		_, err = s.JoinExistingPlayerTeam(ctx, player.PlayerID, w.teamA.JoinKey, now)
		expectErr(t, "join an inactive team", err, ErrInvalidCredentials)

		newKey, err := s.RotateAdminTeamJoinKey(ctx, w.teamA.ID)
		if err != nil {
			t.Fatal(err)
		}
		if newKey == "" || newKey == w.teamA.JoinKey {
			t.Fatal("rotation did not issue a new key")
		}
		_, err = s.JoinExistingPlayerTeam(ctx, player.PlayerID, newKey, now)
		expectErr(t, "join an inactive team after rotation", err, ErrInvalidCredentials)
		if _, err := s.SetTeamActive(ctx, w.teamA.ID, true, now); err != nil {
			t.Fatal(err)
		}
		_, err = s.JoinExistingPlayerTeam(ctx, player.PlayerID, w.teamA.JoinKey, now)
		expectErr(t, "join with a rotated key", err, ErrInvalidCredentials)
		joined, err := s.JoinExistingPlayerTeam(ctx, player.PlayerID, newKey, now)
		if err != nil || joined.TeamID != w.teamA.ID || joined.Role != "member" {
			t.Fatalf("join with the replacement key: team=%d role=%q err=%v", joined.TeamID, joined.Role, err)
		}
	})
}

func TestStoreContractParticipantRules(t *testing.T) {
	forEachStore(t, func(t *testing.T, s contractStore) {
		ctx := context.Background()
		now := time.Now()
		w := buildWorld(t, s)

		_, err := s.RegisterPlayer(ctx, participantRegisterRequest{DisplayName: "Dup", Email: "CONTRACT-CAP@example.test", Password: "contract-password"}, now)
		expectErr(t, "register a taken email", err, ErrDuplicateResource)

		_, err = s.ValidatePlayerSession(ctx, 999999, w.teamA.ID, "captain", 1)
		expectErr(t, "session for an unknown player", err, ErrInvalidCredentials)
		_, err = s.ValidatePlayerSession(ctx, w.captain.ID, w.teamA.ID, "member", 1)
		expectErr(t, "session with a stale role", err, ErrInvalidCredentials)
		_, err = s.ValidatePlayerSession(ctx, w.captain.ID, w.teamA.ID, "captain", 7)
		expectErr(t, "session with a stale version", err, ErrInvalidCredentials)
		_, err = s.ValidatePlayerSession(ctx, w.captain.ID, w.teamB.ID, "captain", 1)
		expectErr(t, "session on another team", err, ErrInvalidCredentials)
		_, err = s.ValidatePlayerSession(ctx, w.organizer.ID, w.teamA.ID, "organizer", 1)
		expectErr(t, "organizer session with a team", err, ErrInvalidCredentials)
		if player, err := s.ValidatePlayerSession(ctx, w.organizer.ID, 0, "organizer", 1); err != nil || player.TeamName != "Organizer" {
			t.Errorf("organizer session: %+v %v", player, err)
		}

		_, err = s.UpdateParticipantProfile(ctx, 999999, participantUpdateProfileRequest{DisplayName: "X", Email: "x@example.test"})
		expectErr(t, "profile of an unknown player", err, ErrInvalidCredentials)
		_, err = s.UpdateParticipantProfile(ctx, w.organizer.ID, participantUpdateProfileRequest{DisplayName: "X", Email: "x@example.test"})
		expectErr(t, "organizer profile", err, ErrInvalidCredentials)
		_, err = s.UpdateParticipantProfile(ctx, w.member.ID, participantUpdateProfileRequest{DisplayName: " ", Email: "x@example.test"})
		expectErr(t, "blank display name", err, ErrDuplicateResource)
		_, err = s.UpdateParticipantProfile(ctx, w.member.ID, participantUpdateProfileRequest{DisplayName: "Mem", Email: "contract-cap@example.test"})
		expectErr(t, "taken email", err, ErrDuplicateResource)
		_, err = s.UpdateParticipantProfile(ctx, w.captain.ID, participantUpdateProfileRequest{DisplayName: "Cap", Email: "contract-cap@example.test"})
		expectErr(t, "captain without team fields", err, ErrDuplicateResource)
		_, err = s.UpdateParticipantProfile(ctx, w.captain.ID, participantUpdateProfileRequest{DisplayName: "Cap", Email: "contract-cap@example.test", TeamName: "Contract B", TeamContactEmail: "new-a@example.test"})
		expectErr(t, "captain taking a team name", err, ErrDuplicateResource)
		_, err = s.UpdateParticipantProfile(ctx, w.captain.ID, participantUpdateProfileRequest{DisplayName: "Cap", Email: "contract-cap@example.test", TeamName: "Contract A2", TeamContactEmail: "contract-b@example.test"})
		expectErr(t, "captain taking a team email", err, ErrDuplicateResource)
		solo, err := s.RegisterPlayer(ctx, participantRegisterRequest{DisplayName: "Solo", Email: "contract-solo@example.test", Password: "contract-password"}, now)
		if err != nil {
			t.Fatal(err)
		}
		if player, err := s.UpdateParticipantProfile(ctx, solo.PlayerID, participantUpdateProfileRequest{DisplayName: "Solo2", Email: "contract-solo@example.test"}); err != nil || player.TeamName != "" {
			t.Errorf("a teamless profile edit: %+v %v", player, err)
		}

		_, err = s.JoinExistingPlayerTeam(ctx, 999999, w.teamA.JoinKey, now)
		expectErr(t, "join by an unknown player", err, ErrInvalidCredentials)
		_, err = s.JoinExistingPlayerTeam(ctx, w.member.ID, w.teamB.JoinKey, now)
		expectErr(t, "join while on a team", err, ErrDuplicateResource)
		_, err = s.JoinExistingPlayerTeam(ctx, solo.PlayerID, "TEAM-NOPE", now)
		expectErr(t, "join with a bad key", err, ErrInvalidCredentials)
		_, err = s.JoinExistingPlayerTeam(ctx, solo.PlayerID, " ", now)
		expectErr(t, "join with no key", err, ErrInvalidCredentials)
		limit := 2
		if _, err := s.UpdatePlatformSettings(ctx, adminUpdatePlatformSettingsRequest{FlagFormatPrefix: "PLAYIT", MaxTeamMembers: &limit}, "test", now); err != nil {
			t.Fatal(err)
		}
		_, err = s.JoinExistingPlayerTeam(ctx, solo.PlayerID, w.teamA.JoinKey, now)
		expectErr(t, "join a full team", err, ErrTeamMemberLimit)
		_, err = s.CreateAdminPlayer(ctx, adminCreatePlayerRequest{TeamID: w.teamA.ID, DisplayName: "Late", Email: "contract-late@example.test", Password: "contract-password"}, now)
		expectErr(t, "admin add to a full team", err, ErrTeamMemberLimit)
		limit = 0
		if _, err := s.UpdatePlatformSettings(ctx, adminUpdatePlatformSettingsRequest{FlagFormatPrefix: "PLAYIT", MaxTeamMembers: &limit}, "test", now); err != nil {
			t.Fatal(err)
		}

		_, err = s.ListTeamMembers(ctx, 0)
		expectErr(t, "members of team 0", err, ErrTeamNotFound)
		_, err = s.ListTeamMembers(ctx, 999999)
		expectErr(t, "members of an unknown team", err, ErrTeamNotFound)

		_, err = s.TransferTeamCaptain(ctx, 999999, w.member.ID)
		expectErr(t, "transfer by an unknown player", err, ErrPlayerNotFound)
		_, err = s.TransferTeamCaptain(ctx, w.member.ID, w.captain.ID)
		expectErr(t, "transfer by a non-captain", err, ErrNotTeamCaptain)
		_, err = s.TransferTeamCaptain(ctx, w.captain.ID, w.captain.ID)
		expectErr(t, "transfer to self", err, ErrInvalidCaptainTransfer)
		_, err = s.TransferTeamCaptain(ctx, w.captain.ID, 999999)
		expectErr(t, "transfer to an unknown player", err, ErrPlayerNotFound)
		_, err = s.TransferTeamCaptain(ctx, w.captain.ID, w.other.ID)
		expectErr(t, "transfer across teams", err, ErrInvalidCaptainTransfer)
		if _, err := s.SetPlayerActive(ctx, w.member.ID, false, now); err != nil {
			t.Fatal(err)
		}
		_, err = s.TransferTeamCaptain(ctx, w.captain.ID, w.member.ID)
		expectErr(t, "transfer to a deactivated teammate", err, ErrInvalidCaptainTransfer)
		if _, err := s.SetPlayerActive(ctx, w.member.ID, true, now); err != nil {
			t.Fatal(err)
		}
		if _, err := s.SetPlayerActive(ctx, w.captain.ID, false, now); err != nil {
			t.Fatal(err)
		}
		_, err = s.TransferTeamCaptain(ctx, w.captain.ID, w.member.ID)
		expectErr(t, "transfer by a deactivated captain", err, ErrAccountDeactivated)
		_, err = s.ValidatePlayerSession(ctx, w.captain.ID, w.teamA.ID, "captain", 1)
		expectErr(t, "session of a deactivated player", err, ErrAccountDeactivated)
		_, err = s.AuthenticatePlayer(ctx, "contract-cap@example.test", "contract-password")
		expectErr(t, "login of a deactivated player", err, ErrAccountDeactivated)
		if _, err := s.SetPlayerActive(ctx, w.captain.ID, true, now); err != nil {
			t.Fatal(err)
		}
		_, err = s.ValidatePlayerSession(ctx, w.captain.ID, w.teamA.ID, "captain", 1)
		expectErr(t, "old session after player reactivation", err, ErrInvalidCredentials)
		captain, err := s.AuthenticatePlayer(ctx, "contract-cap@example.test", "contract-password")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := s.ValidatePlayerSession(ctx, captain.PlayerID, captain.TeamID, captain.Role, captain.SessionVersion); err != nil {
			t.Fatalf("fresh session after player reactivation: %v", err)
		}
		if _, err := s.SetTeamActive(ctx, w.teamB.ID, false, now); err != nil {
			t.Fatal(err)
		}
		_, err = s.ValidatePlayerSession(ctx, w.other.ID, w.teamB.ID, "captain", 1)
		expectErr(t, "session on a deactivated team", err, ErrAccountDeactivated)
		if _, err := s.SetTeamActive(ctx, w.teamB.ID, true, now); err != nil {
			t.Fatal(err)
		}
		_, err = s.ValidatePlayerSession(ctx, w.other.ID, w.teamB.ID, "captain", 1)
		expectErr(t, "old session after team reactivation", err, ErrInvalidCredentials)

		for _, role := range []string{"member", "organizer"} {
			if _, err := s.UpdateAdminPlayer(ctx, w.organizer.ID, adminUpdatePlayerRequest{
				DisplayName: w.organizer.DisplayName, Email: w.organizer.Email, Role: role,
			}); err != nil {
				t.Fatal(err)
			}
		}
		_, err = s.ValidatePlayerSession(ctx, w.organizer.ID, 0, "organizer", 1)
		expectErr(t, "old session after organizer role restored", err, ErrInvalidCredentials)
	})
}

func TestStoreContractServiceRules(t *testing.T) {
	forEachStore(t, func(t *testing.T, s contractStore) {
		ctx := context.Background()
		now := time.Now()
		w := buildWorld(t, s)
		team, chal := w.teamA.ID, w.challenge.ID
		check := func(label string, want error) {
			t.Helper()
			expectErr(t, label+": validate", s.ValidateServiceAction(ctx, team, chal), want)
			_, err := s.UnlockService(ctx, team, chal)
			expectErr(t, label+": unlock", err, want)
			_, err = s.CreateSSHSession(ctx, team, chal, now)
			expectErr(t, label+": ssh", err, want)
			_, err = s.PrepareFactoryResetService(ctx, team, chal)
			expectErr(t, label+": reset", err, want)
			expectErr(t, label+": restart", s.ValidateRestartService(ctx, team, chal), want)
		}

		s.setMatch(false, false)
		check("before the match", ErrMatchNotStarted)
		s.setMatch(true, true)
		check("while paused", ErrMatchPaused)
		s.setMatch(true, false)

		s.deferTeam(team, true)
		check("deferred team", ErrChallengeDeferred)
		s.deferTeam(team, false)
		s.deferChallenge(chal, true)
		check("deferred challenge", ErrChallengeDeferred)
		s.deferChallenge(chal, false)

		if _, err := s.SetChallengeMaintenance(ctx, chal, true, now); err != nil {
			t.Fatal(err)
		}
		check("maintenance", ErrChallengeMaintenance)
		if _, err := s.SetChallengeMaintenance(ctx, chal, false, now); err != nil {
			t.Fatal(err)
		}
		s.deferChallenge(chal, false)

		s.setRuntime(team, chal, "queued")
		check("runtime not ready", ErrServiceUnavailable)
		s.setRuntime(team, chal, "ready")

		_, err := s.UnlockService(ctx, team, 999999)
		expectErr(t, "unlock an unknown challenge", err, ErrChallengeNotFound)
		expectErr(t, "validate an unknown team", s.ValidateServiceAction(ctx, 999999, chal), ErrChallengeNotFound)

		if _, err := s.SetTeamActive(ctx, w.teamB.ID, false, now); err != nil {
			t.Fatal(err)
		}
		expectErr(t, "validate a deactivated team", s.ValidateServiceAction(ctx, w.teamB.ID, chal), ErrTeamNotFound)

		_, err = s.CompleteFactoryResetService(ctx, team, 999999)
		expectErr(t, "complete an unknown reset", err, ErrChallengeNotFound)
		_, err = s.RestartService(ctx, team, 999999)
		expectErr(t, "restart an unknown service", err, ErrChallengeNotFound)

		s.dropTeamState(team, chal)
		expectErr(t, "validate a service with no state", s.ValidateServiceAction(ctx, team, chal), ErrChallengeNotFound)
	})
}

func TestStoreContractAdminRules(t *testing.T) {
	forEachStore(t, func(t *testing.T, s contractStore) {
		ctx := context.Background()
		now := time.Now()
		w := buildWorld(t, s)
		yes := true

		_, err := s.CreateAdminTeam(ctx, adminCreateTeamRequest{Name: "Contract A", ContactEmail: "fresh@example.test"})
		expectErr(t, "duplicate team name", err, ErrDuplicateResource)
		_, err = s.CreateAdminTeam(ctx, adminCreateTeamRequest{Name: " ", ContactEmail: "fresh@example.test"})
		expectErr(t, "blank team name", err, ErrDuplicateResource)
		_, err = s.UpdateAdminTeam(ctx, 999999, adminUpdateTeamRequest{Name: "N", ContactEmail: "n@example.test"})
		expectErr(t, "update an unknown team", err, ErrTeamNotFound)
		_, err = s.UpdateAdminTeam(ctx, w.teamA.ID, adminUpdateTeamRequest{Name: "Contract B", ContactEmail: "contract-a@example.test"})
		expectErr(t, "rename onto another team", err, ErrDuplicateResource)
		_, err = s.UpdateAdminTeam(ctx, w.teamA.ID, adminUpdateTeamRequest{Name: "Contract A", ContactEmail: " "})
		expectErr(t, "blank team email", err, ErrDuplicateResource)
		if renamed, err := s.UpdateAdminTeam(ctx, w.teamA.ID, adminUpdateTeamRequest{Name: "Contract A Renamed", ContactEmail: "CONTRACT-A@example.test"}); err != nil || renamed.Name != "Contract A Renamed" || renamed.ContactEmail != "contract-a@example.test" {
			t.Errorf("a valid rename: %+v %v", renamed, err)
		}
		rows, err := s.ListScoreboard(ctx)
		if err != nil {
			t.Fatal(err)
		}
		for _, row := range rows {
			if row.Team == "Contract A" {
				t.Error("the scoreboard must follow a team rename")
			}
		}
		_, err = s.SetTeamActive(ctx, 999999, false, now)
		expectErr(t, "deactivate an unknown team", err, ErrTeamNotFound)
		expectErr(t, "delete an unknown team", s.DeleteAdminTeam(ctx, 999999), ErrTeamNotFound)

		_, err = s.CreateAdminPlayer(ctx, adminCreatePlayerRequest{TeamID: w.teamA.ID, DisplayName: "P", Email: "p@example.test", Password: "short"}, now)
		expectErr(t, "player with a short password", err, ErrInvalidCredentials)
		_, err = s.CreateAdminPlayer(ctx, adminCreatePlayerRequest{TeamID: 999999, DisplayName: "P", Email: "p@example.test", Password: "contract-password"}, now)
		expectErr(t, "player on an unknown team", err, ErrTeamNotFound)
		_, err = s.UpdateAdminPlayer(ctx, 999999, adminUpdatePlayerRequest{DisplayName: "P", Email: "p@example.test", Role: "member"})
		expectErr(t, "update an unknown player", err, ErrPlayerNotFound)
		_, err = s.UpdateAdminPlayer(ctx, w.member.ID, adminUpdatePlayerRequest{DisplayName: " ", Email: "p@example.test", Role: "member"})
		expectErr(t, "blank player name", err, ErrDuplicateResource)
		_, err = s.UpdateAdminPlayer(ctx, w.member.ID, adminUpdatePlayerRequest{DisplayName: "Mem", Email: "contract-cap@example.test", Role: "member"})
		expectErr(t, "player taking an email", err, ErrDuplicateResource)
		expectErr(t, "delete an unknown player", s.DeleteAdminPlayer(ctx, 999999), ErrPlayerNotFound)
		_, err = s.SetPlayerActive(ctx, 999999, false, now)
		expectErr(t, "deactivate an unknown player", err, ErrPlayerNotFound)
		_, err = s.SetPlayerActive(ctx, 999999, true, now)
		expectErr(t, "reactivate an unknown player", err, ErrPlayerNotFound)

		for label, call := range map[string]func() (adminWireGuardPeer, error){
			"get":    func() (adminWireGuardPeer, error) { return s.GetAdminPlayerWireGuardConfig(ctx, 999999) },
			"rotate": func() (adminWireGuardPeer, error) { return s.RotateAdminPlayerWireGuardConfig(ctx, 999999, now) },
			"revoke": func() (adminWireGuardPeer, error) { return s.RevokeAdminPlayerWireGuardConfig(ctx, 999999, now) },
		} {
			_, err := call()
			expectErr(t, label+" an unknown player's peer", err, ErrPlayerNotFound)
		}
		if peer, err := s.RotateAdminPlayerWireGuardConfig(ctx, w.organizer.ID, now); err != nil || peer.TeamName != "Organizer" {
			t.Errorf("an organizer's peer belongs to the organizer team: %+v %v", peer, err)
		}

		_, err = s.CreateAdminChallenge(ctx, adminCreateChallengeRequest{Name: "contract-web", BaselineImage: "i", CheckerImage: "c"}, now)
		expectErr(t, "duplicate challenge name", err, ErrDuplicateResource)
		_, err = s.CreateAdminChallenge(ctx, adminCreateChallengeRequest{Name: " "}, now)
		expectErr(t, "blank challenge name", err, ErrDuplicateResource)
		_, err = s.CreateAdminChallenge(ctx, adminCreateChallengeRequest{Name: "contract-clash", ServiceSubnetOctet: 77}, now)
		expectErr(t, "subnet already assigned", err, ErrInvalidRuntimeConfig)
		created, err := s.CreateAdminChallenge(ctx, adminCreateChallengeRequest{Name: "Contract Defaults", EgressEnabled: &yes}, now)
		if err != nil || created.BaselineImage == "" || !created.EgressEnabled {
			t.Errorf("default images and egress: %+v %v", created, err)
		}
		if updated, err := s.UpdateAdminChallenge(ctx, created.ID, adminUpdateChallengeRequest{Name: "Contract Defaults", BaselineImage: "b", CheckerImage: "c", EgressEnabled: &yes}); err != nil || !updated.EgressEnabled {
			t.Errorf("update egress: %+v %v", updated, err)
		}
		_, err = s.UpdateAdminChallenge(ctx, 999999, adminUpdateChallengeRequest{Name: "x", BaselineImage: "b", CheckerImage: "c"})
		expectErr(t, "update an unknown challenge", err, ErrChallengeNotFound)
		expectErr(t, "delete an unknown challenge", s.DeleteAdminChallenge(ctx, 999999), ErrChallengeNotFound)
		_, err = s.DeployAdminChallenge(ctx, 999999)
		expectErr(t, "deploy an unknown challenge", err, ErrChallengeNotFound)
		expectErr(t, "delete an unknown deployment", s.DeleteAdminDeployment(ctx, 999999), ErrDeploymentNotFound)
		_, err = s.SetChallengeMaintenance(ctx, 999999, true, now)
		expectErr(t, "maintain an unknown challenge", err, ErrChallengeNotFound)
		_, err = s.ChallengePlayBlocked(ctx, 999999)
		expectErr(t, "play gate of an unknown challenge", err, ErrChallengeNotFound)
		s.deferChallenge(w.challenge.ID, true)
		if blocked, err := s.ChallengePlayBlocked(ctx, w.challenge.ID); err != nil || !blocked {
			t.Errorf("a deferred challenge is blocked: %v %v", blocked, err)
		}
		s.deferChallenge(w.challenge.ID, false)
		// Requeueing nothing changes nothing; handlers validate the ID first.
		_ = s.RequeueChallengeServices(ctx, 999999)
		_ = s.RequeueTeamServices(ctx, 999999)

		_, err = s.GetControllerRuntimeTask(ctx, 999999, w.challenge.ID)
		expectErr(t, "runtime task of an unknown team", err, ErrChallengeNotFound)
		_, err = s.GetControllerServiceAccessPolicy(ctx, 999999, w.challenge.ID)
		expectErr(t, "access policy of an unknown team", err, ErrChallengeNotFound)
		if _, err := s.GetControllerServiceAccessPolicy(ctx, w.teamA.ID, w.challenge.ID); err != nil {
			t.Errorf("access policy: %v", err)
		}

		negative := -1
		_, err = s.UpdatePlatformSettings(ctx, adminUpdatePlatformSettingsRequest{FlagFormatPrefix: " "}, "test", now)
		if err == nil {
			t.Error("an empty flag prefix is refused")
		}
		_, err = s.UpdatePlatformSettings(ctx, adminUpdatePlatformSettingsRequest{FlagFormatPrefix: "CTF", MaxTeamMembers: &negative}, "test", now)
		if err == nil {
			t.Error("a negative member limit is refused")
		}
		if _, err := s.SetPlatformTheme(ctx, " ", "test", now); err == nil {
			t.Error("an empty theme is refused")
		}

		for i := 0; i < 3; i++ {
			if err := s.AppendAdminAuditLog(ctx, adminAuditLogEntry{ActorType: "admin", Actor: "a", Action: fmt.Sprintf("act.%d", i), TargetType: "team", Target: "t", Status: "success"}); err != nil {
				t.Fatal(err)
			}
		}
		for label, query := range map[string]adminAuditLogQuery{
			"defaults":    {Limit: 0, Offset: -1},
			"actor":       {ActorType: "nobody"},
			"action":      {Action: "nothing"},
			"target type": {TargetType: "nothing"},
			"status":      {Status: "nothing"},
			"past end":    {Limit: 5, Offset: 100},
		} {
			page, err := s.ListAdminAuditLogs(ctx, query)
			if err != nil || page.Items == nil {
				t.Errorf("audit query %s: %+v %v", label, page, err)
			}
		}
	})
}

func TestStoreContractVisibilityAndResetLifecycle(t *testing.T) {
	forEachStore(t, func(t *testing.T, s contractStore) {
		ctx := context.Background()
		now := time.Now()
		w := buildWorld(t, s)
		team, chal := w.teamA.ID, w.challenge.ID
		reasonFor := func(label string) string {
			t.Helper()
			states, err := s.ListTeamServices(ctx, team)
			if err != nil {
				t.Fatalf("%s: %v", label, err)
			}
			for _, state := range states {
				if state.ChallengeID == chal {
					if state.LockReason != "" && (!state.Maintenance || (state.LockReason != "maintenance" && state.LockReason != "deferred" && state.Endpoint != "")) {
						t.Errorf("%s: a locked service must be closed and hide its endpoint: %+v", label, state)
					}
					return state.LockReason
				}
			}
			t.Fatalf("%s: service missing from %+v", label, states)
			return ""
		}

		s.setMatch(false, false)
		if reason := reasonFor("before the match"); reason != "match_not_started" {
			t.Errorf("before the match: %q", reason)
		}
		if services, err := s.ListPublicServices(ctx); err != nil || len(services) != 0 {
			t.Errorf("no public services before the match: %v %v", services, err)
		}
		if blocked, err := s.ChallengePlayBlocked(ctx, chal); err != nil || !blocked {
			t.Errorf("play is blocked before the match: %v %v", blocked, err)
		}

		s.setMatch(true, true)
		if reason := reasonFor("paused"); reason != "match_paused" {
			t.Errorf("paused: %q", reason)
		}
		if services, err := s.ListPublicServices(ctx); err != nil || len(services) != 0 {
			t.Errorf("no public services while paused: %v %v", services, err)
		}
		challenges, err := s.ListChallenges(ctx)
		if err != nil {
			t.Fatal(err)
		}
		for _, item := range challenges {
			if !item.Maintenance || item.HasSourceDownload {
				t.Errorf("a paused match closes every challenge and its source: %+v", item)
			}
		}

		s.setMatch(true, false)
		s.deferChallenge(chal, true)
		if reason := reasonFor("deferred"); reason != "deferred" {
			t.Errorf("deferred: %q", reason)
		}
		s.deferChallenge(chal, false)
		if services, err := s.ListTeamServices(ctx, 999999); err != nil || len(services) != 0 {
			t.Errorf("an unknown team has no services: %v %v", services, err)
		}

		_, err = s.RegisterPlayer(ctx, participantRegisterRequest{DisplayName: " ", Email: "blank@example.test", Password: "contract-password"}, now)
		expectErr(t, "register without a name", err, ErrDuplicateResource)
		if organizer, err := s.AuthenticatePlayer(ctx, "contract-org@example.test", "contract-password"); err != nil || organizer.TeamName != "Organizer" {
			t.Errorf("an organizer signs in to the organizer team: %+v %v", organizer, err)
		}

		if _, err := s.SetPlayerActive(ctx, w.member.ID, false, now); err != nil {
			t.Fatal(err)
		}
		members, err := s.ListTeamMembers(ctx, team)
		if err != nil {
			t.Fatal(err)
		}
		for _, member := range members {
			if member.PlayerID == w.member.ID {
				t.Error("a deactivated player is not listed as a member")
			}
		}

		prepared, err := s.PrepareFactoryResetService(ctx, team, chal)
		if err != nil {
			t.Fatal(err)
		}
		_, err = s.PrepareFactoryResetService(ctx, team, chal)
		expectErr(t, "a second reset while one runs", err, ErrServiceUnavailable)
		stale := prepared
		stale.resetToken = "not-the-token"
		expectErr(t, "aborting with a stale token", s.AbortFactoryResetService(ctx, stale), ErrServiceUnavailable)
		if err := s.AbortFactoryResetService(ctx, prepared); err != nil {
			t.Fatalf("abort: %v", err)
		}

		_, err = s.GetControllerRuntimeTask(ctx, team, 999999)
		expectErr(t, "runtime task of an unknown challenge", err, ErrChallengeNotFound)
		_, err = s.GetControllerServiceAccessPolicy(ctx, team, 999999)
		expectErr(t, "access policy of an unknown challenge", err, ErrChallengeNotFound)
		expectErr(t, "validation of an unknown challenge", s.SaveAdminChallengeValidation(ctx, ChallengeValidationResult{ChallengeID: 999999, Status: "valid"}), ErrChallengeNotFound)

		// Two pending deployments reconcile in job order.
		for _, name := range []string{"contract-one", "contract-two"} {
			created, err := s.CreateAdminChallenge(ctx, adminCreateChallengeRequest{Name: name, BaselineImage: "i", CheckerImage: "c"}, now)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := s.DeployAdminChallenge(ctx, created.ID); err != nil {
				t.Fatal(err)
			}
		}
		if result, err := s.ReconcileAdminDeployments(ctx, now); err != nil || result.CompletedJobs < 2 {
			t.Errorf("both deployments complete: %+v %v", result, err)
		}
	})
}

func TestStoreContractClosedNetworksAndEdits(t *testing.T) {
	forEachStore(t, func(t *testing.T, s contractStore) {
		ctx := context.Background()
		now := time.Now()
		w := buildWorld(t, s)

		solo, err := s.RegisterPlayer(ctx, participantRegisterRequest{DisplayName: "Solo", Email: "closed-solo@example.test", Password: "contract-password"}, now)
		if err != nil {
			t.Fatal(err)
		}
		_, err = s.ValidatePlayerSession(ctx, solo.PlayerID, w.teamA.ID, solo.Role, 1)
		expectErr(t, "a teamless player claiming a team", err, ErrInvalidCredentials)

		_, err = s.UpdateAdminChallenge(ctx, w.challenge.ID, adminUpdateChallengeRequest{Name: " ", BaselineImage: "b", CheckerImage: "c"})
		expectErr(t, "a blank challenge name", err, ErrDuplicateResource)
		other, err := s.CreateAdminChallenge(ctx, adminCreateChallengeRequest{Name: "closed-other", BaselineImage: "b", CheckerImage: "c"}, now)
		if err != nil {
			t.Fatal(err)
		}
		_, err = s.UpdateAdminChallenge(ctx, other.ID, adminUpdateChallengeRequest{Name: "CONTRACT-WEB", BaselineImage: "b", CheckerImage: "c"})
		expectErr(t, "renaming onto another challenge", err, ErrDuplicateResource)

		// With the network closed, only organizers stay allowlisted.
		if s.db != nil {
			if _, err := s.db.ExecContext(ctx, `UPDATE team_service_states SET unlocked = TRUE WHERE team_id = $1 AND challenge_id = $2`, w.teamA.ID, w.challenge.ID); err != nil {
				t.Fatal(err)
			}
		}
		if _, err := s.UnlockService(ctx, w.teamA.ID, w.challenge.ID); err != nil {
			t.Fatal(err)
		}
		organizerPeer, err := s.GetAdminPlayerWireGuardConfig(ctx, w.organizer.ID)
		if err != nil {
			t.Fatal(err)
		}
		s.setMatch(false, false)
		policies, err := s.ListControllerServiceAccessPolicies(ctx)
		if err != nil {
			t.Fatal(err)
		}
		checked := false
		for _, policy := range policies {
			if policy.TeamID != w.teamA.ID || policy.ChallengeID != w.challenge.ID {
				continue
			}
			checked = true
			if !policy.NetworkClosed || policy.MatchPaused || len(policy.AllowedPeerAddresses) != 1 || policy.AllowedPeerAddresses[0] != organizerPeer.Address {
				t.Errorf("a closed network admits only organizers: %+v (organizer %s)", policy, organizerPeer.Address)
			}
		}
		if !checked {
			t.Fatal("the unlocked service has no policy")
		}
		s.setMatch(true, true)
		policy, err := s.GetControllerServiceAccessPolicy(ctx, w.teamA.ID, w.challenge.ID)
		if err != nil || !policy.NetworkClosed || !policy.MatchPaused {
			t.Fatalf("a paused match must enable bridge isolation: %+v %v", policy, err)
		}
		policies, err = s.ListControllerServiceAccessPolicies(ctx)
		if err != nil {
			t.Fatal(err)
		}
		for _, policy := range policies {
			if !policy.NetworkClosed || !policy.MatchPaused {
				t.Errorf("global pause reconciliation must close this service: %+v", policy)
			}
		}
		s.setMatch(true, false)

		deployment, err := s.DeployAdminChallenge(ctx, other.ID)
		if err != nil {
			t.Fatal(err)
		}
		expectErr(t, "deleting a deployment with queued instances", s.DeleteAdminDeployment(ctx, deployment.JobID), ErrDeploymentActive)
	})
}

func TestStoreContractVolumeFollowsInstanceRatherThanNetworkSlot(t *testing.T) {
	forEachStore(t, func(t *testing.T, s contractStore) {
		ctx := context.Background()
		world := buildWorld(t, s)
		original, err := s.GetControllerRuntimeTask(ctx, world.teamA.ID, world.challenge.ID)
		if err != nil || original.StateVolume == "" {
			t.Fatalf("missing initial volume: %+v %v", original, err)
		}
		if _, err := s.DeployAdminChallenge(ctx, world.challenge.ID); err != nil {
			t.Fatal(err)
		}
		redeployed, err := s.GetControllerRuntimeTask(ctx, world.teamA.ID, world.challenge.ID)
		if err != nil || redeployed.StateVolume != original.StateVolume {
			t.Fatalf("redeploy changed existing storage: %+v %v", redeployed, err)
		}
		if err := s.DeleteAdminTeam(ctx, world.teamA.ID); err != nil {
			t.Fatal(err)
		}
		replacement, err := s.CreateAdminTeam(ctx, adminCreateTeamRequest{Name: "Replacement", ContactEmail: "replacement@example.test"})
		if err != nil || replacement.ID != world.teamA.ID {
			t.Fatalf("expected a reused network slot: %+v %v", replacement, err)
		}
		task, err := s.GetControllerRuntimeTask(ctx, replacement.ID, world.challenge.ID)
		if err != nil || task.StateVolume == "" || task.StateVolume == original.StateVolume {
			t.Fatalf("replacement inherited storage: %+v %v", task, err)
		}
		volumes, err := s.ListControllerTeamStateVolumes(ctx, replacement.ID)
		if err != nil || !slices.Contains(volumes, task.StateVolume) || slices.Contains(volumes, original.StateVolume) {
			t.Fatalf("unexpected authoritative volume list: %v %v", volumes, err)
		}
	})
}
