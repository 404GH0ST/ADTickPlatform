package apigateway

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sort"
	"testing"
	"time"

	"adplatform/internal/platform/database"
	"adplatform/internal/platform/database/dbtest"
)

// TestPostgresStoreSurvivesInjectedFaults fails each database round trip of
// each store operation in turn. Every operation must handle the failure (no
// panic) and leave the store able to complete the same operation afterwards.
func TestPostgresStoreSurvivesInjectedFaults(t *testing.T) {
	db := dbtest.IsolatedSchema(t, "TEST_POSTGRES_DSN", "store_faults_test")
	t.Setenv("WIREGUARD_SERVER_PUBLIC_KEY", "faults-server-public-key")
	ctx := context.Background()
	if err := database.RunEmbeddedMigrationsWithOptions(ctx, db, database.MigrationOptions{IncludeSeeds: false}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	faultyDB, faults := dbtest.FaultInjectingDB(t, "TEST_POSTGRES_DSN", db)
	clean := NewPostgresStore(db)
	store := NewPostgresStore(faultyDB)
	now := time.Now().UTC()

	team, err := clean.CreateAdminTeam(ctx, adminCreateTeamRequest{Name: "Fault Team", ContactEmail: "fault-team@example.test"})
	if err != nil {
		t.Fatal(err)
	}
	challenge, err := clean.CreateAdminChallenge(ctx, adminCreateChallengeRequest{Name: "fault-web", BaselineImage: "img/web", CheckerImage: "img/checker"}, now)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := clean.DeployAdminChallenge(ctx, challenge.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := clean.ReconcileAdminDeployments(ctx, now); err != nil {
		t.Fatal(err)
	}
	// Operations that requeue, redeploy, or pause services act on their own
	// team and challenge so they cannot disturb the service-action fixtures.
	mutableTeam, err := clean.CreateAdminTeam(ctx, adminCreateTeamRequest{Name: "Mutable Team", ContactEmail: "mutable-team@example.test"})
	if err != nil {
		t.Fatal(err)
	}
	mutable, err := clean.CreateAdminChallenge(ctx, adminCreateChallengeRequest{Name: "fault-mutable", BaselineImage: "img/m", CheckerImage: "img/mc"}, now)
	if err != nil {
		t.Fatal(err)
	}
	captain, err := clean.CreateAdminPlayer(ctx, adminCreatePlayerRequest{TeamID: team.ID, DisplayName: "Cap", Email: "cap@example.test", Password: "cap-password"}, now)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `UPDATE challenges SET published = TRUE;
		INSERT INTO game_match_state(singleton, state) VALUES (TRUE, 'running') ON CONFLICT (singleton) DO UPDATE SET state = 'running';
		INSERT INTO game_ticks(id, status) VALUES (1, 'completed');
		UPDATE team_service_states SET unlocked = TRUE`); err != nil {
		t.Fatal(err)
	}

	// A member limit makes joins and adds count the team; a stored attack and
	// a flag issued against the current checker token give the feed and the
	// factory reset rows to read.
	t.Setenv("WIREGUARD_SERVER_DNS", "1.1.1.1")
	limit := 1_000_000
	if _, err := clean.UpdatePlatformSettings(ctx, adminUpdatePlatformSettingsRequest{FlagFormatPrefix: "PLAYIT", MaxTeamMembers: &limit}, "fixture", now); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO attack_events (id, attacker, victim, service, tick, verdict) VALUES (1, 'A', 'B', 'svc', 1, 'accepted')`); err != nil {
		t.Fatal(err)
	}
	issueFlag := func(run int) error {
		_, err := db.ExecContext(ctx, `
			INSERT INTO issued_flags (flag, owner_team_id, owner_team_name, challenge_id, challenge_name, issued_tick, expires_tick, checker_token_snapshot)
			SELECT $3, si.team_id, 'Fault Team', si.challenge_id, 'fault-web', 1, 5, si.checker_token
			FROM service_instances si WHERE si.team_id = $1 AND si.challenge_id = $2`, team.ID, challenge.ID, fmt.Sprintf("FLAG-%d-%d", run, time.Now().UnixNano()))
		return err
	}

	newPlayer := func(run int, teamID int) adminPlayer {
		t.Helper()
		player, err := clean.CreateAdminPlayer(ctx, adminCreatePlayerRequest{TeamID: teamID, DisplayName: fmt.Sprintf("P%d", run), Email: fmt.Sprintf("p-%d-%d@example.test", run, time.Now().UnixNano()), Password: "player-password"}, now)
		if err != nil {
			t.Fatal(err)
		}
		return player
	}
	newTeam := func(run int) adminTeam {
		t.Helper()
		created, err := clean.CreateAdminTeam(ctx, adminCreateTeamRequest{Name: fmt.Sprintf("T-%d-%d", run, time.Now().UnixNano()), ContactEmail: fmt.Sprintf("t-%d-%d@example.test", run, time.Now().UnixNano())})
		if err != nil {
			t.Fatal(err)
		}
		return created
	}
	unique := func(prefix string, run int) string {
		return fmt.Sprintf("%s-%d-%d", prefix, run, time.Now().UnixNano())
	}

	ops := map[string]func(run int) error{
		"AuthenticatePlayer": func(int) error {
			_, err := store.AuthenticatePlayer(ctx, "cap@example.test", "cap-password")
			return err
		},
		"RegisterPlayer": func(run int) error {
			_, err := store.RegisterPlayer(ctx, participantRegisterRequest{DisplayName: "Reg", Email: unique("reg", run) + "@example.test", Password: "register-password"}, now)
			return err
		},
		"ValidatePlayerSession": func(int) error {
			_, err := store.ValidatePlayerSession(ctx, captain.ID, team.ID, "captain", 1)
			return err
		},
		"UpdateParticipantProfile": func(run int) error {
			player := newPlayer(run, team.ID)
			_, err := store.UpdateParticipantProfile(ctx, player.ID, participantUpdateProfileRequest{DisplayName: "Renamed", Email: player.Email})
			return err
		},
		"CaptainProfileRename": func(run int) error {
			fresh := newTeam(run)
			lead := newPlayer(run, fresh.ID)
			_, err := store.UpdateParticipantProfile(ctx, lead.ID, participantUpdateProfileRequest{DisplayName: "Lead", Email: lead.Email, TeamName: unique("renamed", run), TeamContactEmail: unique("renamed", run) + "@example.test"})
			return err
		},
		"ImportAdminTeams": func(run int) error {
			result, err := store.ImportAdminTeams(ctx, []adminBulkImportTeam{{
				Name: unique("imported", run), ContactEmail: unique("imported", run) + "@example.test",
				Players: []adminBulkImportPlayer{{DisplayName: "Imp", Email: unique("imp", run) + "@example.test", Password: "import-password"}},
			}}, now)
			if err == nil && len(result.Errors) > 0 {
				err = errors.New(result.Errors[0])
			}
			return err
		},
		"ChangeParticipantPassword": func(run int) error {
			player := newPlayer(run, team.ID)
			_, err := store.ChangeParticipantPassword(ctx, player.ID, "player-password", "changed-password")
			return err
		},
		"JoinExistingPlayerTeam": func(run int) error {
			fresh := newTeam(run)
			registered, err := clean.RegisterPlayer(ctx, participantRegisterRequest{DisplayName: "Joiner", Email: unique("join", run) + "@example.test", Password: "joiner-password"}, now)
			if err != nil {
				return err
			}
			_, err = store.JoinExistingPlayerTeam(ctx, registered.PlayerID, fresh.JoinKey, now)
			return err
		},
		"TransferTeamCaptain": func(run int) error {
			fresh := newTeam(run)
			lead := newPlayer(run, fresh.ID)
			next := newPlayer(run, fresh.ID)
			_, err := store.TransferTeamCaptain(ctx, lead.ID, next.ID)
			return err
		},
		"ListTeamMembers": func(int) error { _, err := store.ListTeamMembers(ctx, team.ID); return err },
		"ListChallenges":  func(int) error { _, err := store.ListChallenges(ctx); return err },
		"ListPublicServices": func(int) error {
			_, err := store.ListPublicServices(ctx)
			return err
		},
		"ListTeamServices": func(int) error { _, err := store.ListTeamServices(ctx, team.ID); return err },
		"ListScoreboard":   func(int) error { _, err := store.ListScoreboard(ctx); return err },
		"ListAttackFeed":   func(int) error { _, err := store.ListAttackFeed(ctx); return err },
		"ScoreboardFreeze": func(int) error {
			freezeAt, unfreezeAt := now.Add(-time.Minute), now.Add(time.Hour)
			if _, err := store.SetScoreboardFreezeWindow(ctx, &freezeAt, &unfreezeAt, now); err != nil {
				return err
			}
			if _, err := store.SaveFrozenScoreboardSnapshot(ctx, []scoreRow{{Team: "x"}}, now); err != nil {
				return err
			}
			if _, err := store.GetScoreboardFreeze(ctx); err != nil {
				return err
			}
			_, err := store.ClearScoreboardFreeze(ctx, now)
			return err
		},
		"ValidateServiceAction": func(int) error { return store.ValidateServiceAction(ctx, team.ID, challenge.ID) },
		"UnlockService": func(int) error {
			if _, err := db.ExecContext(ctx, `UPDATE team_service_states SET unlocked = TRUE WHERE team_id = $1 AND challenge_id = $2`, team.ID, challenge.ID); err != nil {
				return err
			}
			_, err := store.UnlockService(ctx, team.ID, challenge.ID)
			return err
		},
		"CreateSSHSession": func(int) error {
			_, err := store.CreateSSHSession(ctx, team.ID, challenge.ID, now)
			return err
		},
		"MarkSSHSessionApplyFailure": func(int) error {
			return store.MarkSSHSessionApplyFailure(ctx, team.ID, challenge.ID)
		},
		"RestartService": func(int) error {
			if err := store.ValidateRestartService(ctx, team.ID, challenge.ID); err != nil {
				return err
			}
			_, err := store.RestartService(ctx, team.ID, challenge.ID)
			return err
		},
		"DeleteAdminDeployment": func(int) error {
			deployment, err := clean.DeployAdminChallenge(ctx, mutable.ID)
			if err != nil {
				return err
			}
			if _, err := clean.ReconcileAdminDeployments(ctx, now); err != nil {
				return err
			}
			return store.DeleteAdminDeployment(ctx, deployment.JobID)
		},
		"ReconcileStandaloneInstances": func(run int) error {
			// A team created after deployment gets queued instances with no job.
			newTeam(run)
			_, err := store.ReconcileAdminDeployments(ctx, now)
			return err
		},
		"RepairMissingPeer": func(run int) error {
			player := newPlayer(run, team.ID)
			if _, err := db.ExecContext(ctx, `DELETE FROM wireguard_peers WHERE player_id = $1`, player.ID); err != nil {
				return err
			}
			_, err := store.ListWireGuardGatewayPeers(ctx)
			return err
		},
		"LockServiceLifecycle": func(int) error {
			unlock, err := store.LockServiceLifecycle(ctx, team.ID, challenge.ID)
			if err != nil {
				return err
			}
			unlock()
			return nil
		},
		"DeferredValidation": func(int) error {
			if _, err := db.ExecContext(ctx, `UPDATE teams SET play_from_tick = 99 WHERE id = $1`, mutableTeam.ID); err != nil {
				return err
			}
			defer db.ExecContext(ctx, `UPDATE teams SET play_from_tick = NULL WHERE id = $1`, mutableTeam.ID)
			if err := store.ValidateServiceAction(ctx, mutableTeam.ID, challenge.ID); !errors.Is(err, ErrChallengeDeferred) {
				return fmt.Errorf("want deferred, got %w", err)
			}
			return nil
		},
		"AbortFactoryReset": func(int) error {
			// Start from a settled service so every run can prepare afresh.
			if _, err := db.ExecContext(ctx, `UPDATE team_service_states SET status = 'stable' WHERE team_id = $1 AND challenge_id = $2`, team.ID, challenge.ID); err != nil {
				return err
			}
			prepared, err := clean.PrepareFactoryResetService(ctx, team.ID, challenge.ID)
			if err != nil {
				return err
			}
			return store.AbortFactoryResetService(ctx, prepared)
		},
		"FactoryResetLifecycle": func(run int) error {
			if err := issueFlag(run); err != nil {
				return err
			}
			prepared, err := store.PrepareFactoryResetService(ctx, team.ID, challenge.ID)
			if err != nil {
				return err
			}
			if _, err := store.CompleteFactoryResetService(ctx, team.ID, challenge.ID); err != nil {
				_ = store.AbortFactoryResetService(ctx, prepared)
				return err
			}
			if err := store.MarkFactoryResetFailure(ctx, team.ID, challenge.ID); err != nil {
				return err
			}
			if err := store.MarkFactoryResetCompletionFailure(ctx, team.ID, challenge.ID); err != nil {
				return err
			}
			// Put the runtime back to ready so the next run can reset again.
			_, err = db.ExecContext(ctx, `UPDATE service_instances SET runtime_status = 'ready' WHERE team_id = $1 AND challenge_id = $2`, team.ID, challenge.ID)
			return err
		},
		"ListAdminTeams": func(int) error { _, err := store.ListAdminTeams(ctx); return err },
		"CreateAdminTeam": func(run int) error {
			_, err := store.CreateAdminTeam(ctx, adminCreateTeamRequest{Name: unique("team", run), ContactEmail: unique("team", run) + "@example.test"})
			return err
		},
		"UpdateAdminTeam": func(run int) error {
			fresh := newTeam(run)
			_, err := store.UpdateAdminTeam(ctx, fresh.ID, adminUpdateTeamRequest{Name: unique("upd", run), ContactEmail: unique("upd", run) + "@example.test"})
			return err
		},
		"SetTeamActive": func(run int) error {
			fresh := newTeam(run)
			if _, err := store.SetTeamActive(ctx, fresh.ID, false, now); err != nil {
				return err
			}
			_, err := store.SetTeamActive(ctx, fresh.ID, true, now)
			return err
		},
		"ListAdminPlayers": func(int) error { _, err := store.ListAdminPlayers(ctx); return err },
		"CreateAdminPlayer": func(run int) error {
			_, err := store.CreateAdminPlayer(ctx, adminCreatePlayerRequest{TeamID: team.ID, DisplayName: "New", Email: unique("new", run) + "@example.test", Password: "new-password"}, now)
			return err
		},
		"CreateOrganizer": func(run int) error {
			_, err := store.CreateAdminPlayer(ctx, adminCreatePlayerRequest{DisplayName: "Org", Email: unique("org", run) + "@example.test", Password: "org-password", Role: "organizer"}, now)
			return err
		},
		"UpdateAdminPlayer": func(run int) error {
			player := newPlayer(run, team.ID)
			_, err := store.UpdateAdminPlayer(ctx, player.ID, adminUpdatePlayerRequest{DisplayName: "Upd", Email: player.Email, Role: "member"})
			return err
		},
		"SetPlayerActive": func(run int) error {
			fresh := newTeam(run)
			lead := newPlayer(run, fresh.ID)
			if _, err := store.SetPlayerActive(ctx, lead.ID, false, now); err != nil {
				return err
			}
			newPlayer(run, fresh.ID) // promoted to captain while the lead is inactive
			_, err := store.SetPlayerActive(ctx, lead.ID, true, now)
			return err
		},
		"WireGuardPeer": func(run int) error {
			player := newPlayer(run, team.ID)
			// A changed server profile makes the read rewrite the issued peer.
			os.Setenv("WIREGUARD_SERVER_DNS", fmt.Sprintf("10.53.0.%d", run%250+1))
			if _, err := store.GetAdminPlayerWireGuardConfig(ctx, player.ID); err != nil {
				return err
			}
			if _, err := store.RotateAdminPlayerWireGuardConfig(ctx, player.ID, now); err != nil {
				return err
			}
			_, err := store.RevokeAdminPlayerWireGuardConfig(ctx, player.ID, now)
			return err
		},
		"ListWireGuardGatewayPeers": func(int) error { _, err := store.ListWireGuardGatewayPeers(ctx); return err },
		"ListAdminChallenges":       func(int) error { _, err := store.ListAdminChallenges(ctx); return err },
		"CreateAdminChallenge": func(run int) error {
			_, err := store.CreateAdminChallenge(ctx, adminCreateChallengeRequest{Name: unique("chal", run), BaselineImage: "img", CheckerImage: "chk"}, now)
			return err
		},
		"UpdateAdminChallenge": func(int) error {
			_, err := store.UpdateAdminChallenge(ctx, mutable.ID, adminUpdateChallengeRequest{Name: "fault-mutable", BaselineImage: "img/m2", CheckerImage: "img/mc"})
			return err
		},
		"DeployAndReconcile": func(int) error {
			if _, err := store.DeployAdminChallenge(ctx, mutable.ID); err != nil {
				return err
			}
			_, err := store.ReconcileAdminDeployments(ctx, now)
			return err
		},
		"ListAdminDeployments": func(int) error { _, err := store.ListAdminDeployments(ctx); return err },
		"ChallengeMaintenance": func(int) error {
			if _, err := store.SetChallengeMaintenance(ctx, mutable.ID, true, now); err != nil {
				return err
			}
			_, err := store.SetChallengeMaintenance(ctx, mutable.ID, false, now)
			return err
		},
		"ChallengePlayBlocked": func(int) error { _, err := store.ChallengePlayBlocked(ctx, challenge.ID); return err },
		"RequeueServices": func(int) error {
			if err := store.RequeueTeamServices(ctx, mutableTeam.ID); err != nil {
				return err
			}
			return store.RequeueChallengeServices(ctx, mutable.ID)
		},
		"MatchState": func(int) error {
			if _, err := store.IsMatchStarted(ctx); err != nil {
				return err
			}
			_, err := store.IsMatchPaused(ctx)
			return err
		},
		"AuditLogs": func(int) error {
			if err := store.AppendAdminAuditLog(ctx, adminAuditLogEntry{ActorType: "admin", Actor: "organizer", Action: "test", TargetType: "t", Target: "x", Status: "success", Message: "fault sweep", Metadata: "{}"}); err != nil {
				return err
			}
			_, err := store.ListAdminAuditLogs(ctx, adminAuditLogQuery{Limit: 5, Action: "test", ActorType: "admin"})
			return err
		},
		"PlatformSettings": func(int) error {
			if _, err := store.GetPlatformSettings(ctx); err != nil {
				return err
			}
			if _, err := store.UpdatePlatformSettings(ctx, adminUpdatePlatformSettingsRequest{FlagFormatPrefix: "CTF"}, "organizer", now); err != nil {
				return err
			}
			_, err := store.SetActiveFlagFormat(ctx, "CTF", now)
			return err
		},
		"PlatformAppearance": func(int) error {
			if _, err := store.GetPlatformAppearance(ctx); err != nil {
				return err
			}
			_, err := store.SetPlatformTheme(ctx, "neubrutalism", "organizer", now)
			return err
		},
		"ControllerViews": func(int) error {
			if _, err := store.ListControllerRuntimeTasks(ctx); err != nil {
				return err
			}
			if _, err := store.GetControllerRuntimeTask(ctx, team.ID, challenge.ID); err != nil {
				return err
			}
			if _, err := store.ListControllerServiceAccessPolicies(ctx); err != nil {
				return err
			}
			_, err := store.GetControllerServiceAccessPolicy(ctx, team.ID, challenge.ID)
			return err
		},
		"Announcements": func(run int) error {
			item, err := store.CreateAnnouncement(ctx, unique("note", run), "organizer", now)
			if err != nil {
				return err
			}
			if _, err := store.ListAnnouncements(ctx); err != nil {
				return err
			}
			return store.DeleteAnnouncement(ctx, item.ID)
		},
		"SaveChallengeValidation": func(int) error {
			return store.SaveAdminChallengeValidation(ctx, ChallengeValidationResult{ChallengeID: mutable.ID, Status: "valid", CheckedAt: now.Format(time.RFC3339)})
		},
		"DeleteResources": func(run int) error {
			player := newPlayer(run, 0)
			if err := store.DeleteAdminPlayer(ctx, player.ID); err != nil {
				return err
			}
			fresh := newTeam(run)
			if err := store.DeleteAdminTeam(ctx, fresh.ID); err != nil {
				return err
			}
			extra, err := clean.CreateAdminChallenge(ctx, adminCreateChallengeRequest{Name: unique("gone", run), BaselineImage: "img", CheckerImage: "chk"}, now)
			if err != nil {
				return err
			}
			return store.DeleteAdminChallenge(ctx, extra.ID)
		},
	}
	names := make([]string, 0, len(ops))
	for name := range ops {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		t.Run(name, func(t *testing.T) {
			// Ops create teams per run; prune them so the sweeps cannot exhaust
			// the team network address pool.
			pruned := func(run int) error {
				defer func() {
					_, _ = db.ExecContext(ctx, `DELETE FROM teams WHERE id NOT IN ($1, $2)`, team.ID, mutableTeam.ID)
				}()
				return ops[name](run)
			}
			dbtest.SweepFaults(t, faults, name, pruned)
			dbtest.SweepRowFaults(t, faults, name, pruned)
			dbtest.SweepResultFaults(t, faults, name, pruned)
		})
	}
}
