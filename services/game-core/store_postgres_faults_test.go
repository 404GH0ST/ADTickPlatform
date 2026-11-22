package main

import (
	"context"
	"fmt"
	"sort"
	"testing"
	"time"

	"adplatform/internal/platform/database"
	"adplatform/internal/platform/database/dbtest"
	"adplatform/internal/services/apigateway"
)

// TestPostgresGameStoreSurvivesInjectedFaults fails each database round trip
// of each game-store operation in turn: scoring, ticks, captures, and match
// transitions must handle the failure and complete when retried.
func TestPostgresGameStoreSurvivesInjectedFaults(t *testing.T) {
	db := dbtest.IsolatedSchema(t, "SUBMISSION_TEST_POSTGRES_DSN", "game_faults_test")
	ctx := context.Background()
	if err := database.RunEmbeddedMigrationsWithOptions(ctx, db, database.MigrationOptions{IncludeSeeds: true}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if _, err := db.ExecContext(ctx, `UPDATE challenges SET published = TRUE`); err != nil {
		t.Fatal(err)
	}
	faultyDB, faults := dbtest.FaultInjectingDB(t, "SUBMISSION_TEST_POSTGRES_DSN", db)
	clean := newPostgresGameStore(db)
	store := newPostgresGameStore(faultyDB)
	now := time.Now().UTC()

	if _, err := clean.StartMatch(ctx, now); err != nil {
		t.Fatalf("start match: %v", err)
	}
	server := newGameCoreServer("dev-admin-token", store, testCheckerClient{}, newFlagCodec("faults-secret", "PLAYIT"), &testGameScheduler{}, []string{"put", "get", "check"}, 15).
		WithWarmupRequired(false).
		WithAutoTickOnMatchStart(false)
	cleanServer := newGameCoreServer("dev-admin-token", clean, testCheckerClient{}, newFlagCodec("faults-secret", "PLAYIT"), &testGameScheduler{}, []string{"put", "get", "check"}, 15).
		WithWarmupRequired(false).
		WithAutoTickOnMatchStart(false)
	if _, err := cleanServer.advanceTick(ctx); err != nil {
		t.Fatalf("seed tick: %v", err)
	}

	targets, err := clean.ListCheckerTargets(ctx)
	if err != nil || len(targets) < 2 {
		t.Fatalf("need at least two checker targets, got %d (%v)", len(targets), err)
	}
	victim, attacker := targets[0], targets[len(targets)-1]
	if victim.TeamID == attacker.TeamID {
		t.Fatal("fixture needs targets from two teams")
	}
	currentTick := func() int {
		t.Helper()
		status, err := clean.GameStatus(ctx)
		if err != nil || status.CurrentTick == nil {
			t.Fatalf("game status: %+v %v", status, err)
		}
		return status.CurrentTick.ID
	}
	issue := func(run int) issuedFlagRecord {
		t.Helper()
		tick := currentTick()
		record := issuedFlagRecord{Flag: fmt.Sprintf("PLAYIT{fault-%d-%d}", run, time.Now().UnixNano()), OwnerTeamID: victim.TeamID, OwnerTeamName: victim.TeamName, ChallengeID: victim.ChallengeID, ChallengeName: victim.ChallengeName, IssuedTick: tick, ExpiresTick: tick + 5, CreatedAt: now, NextCheckAt: now}
		if err := clean.IssueFlag(ctx, record); err != nil {
			t.Fatal(err)
		}
		return record
	}
	capture := func(record issuedFlagRecord) acceptedFlagSubmission {
		return acceptedFlagSubmission{Flag: record.Flag, SubmittingTeam: attacker.TeamID, AttackerName: attacker.TeamName, VictimName: victim.TeamName, ChallengeName: victim.ChallengeName, ChallengeID: victim.ChallengeID, ExpiresTick: record.ExpiresTick, SubmittedAt: now}
	}
	run := func(tick, team, challenge int, phase string) checkerRunRecord {
		return checkerRunRecord{TickID: tick, TeamID: team, TeamName: "t", ChallengeID: challenge, ChallengeName: "c", Phase: phase, Status: "success", ReportedServiceState: "ok", CheckedAt: now}
	}

	ops := map[string]func(run int) error{
		// One tick's own persistence. The full server tick fans out to every
		// checker target (hundreds of round trips), which would make an
		// exhaustive sweep quadratic; the lifecycle test covers that path.
		"TickRows": func(int) error {
			tick, err := store.StartNextTick(ctx, now)
			if err != nil {
				_, _ = clean.ReapRunningTicks(ctx)
				return err
			}
			tick.Status = "completed"
			tick.CompletedAt = now.Format(time.RFC3339)
			if _, err := store.CompleteTick(ctx, tick); err != nil {
				_, _ = clean.ReapRunningTicks(ctx)
				return err
			}
			return nil
		},
		"IssueAndLookupFlag": func(r int) error {
			record := issueFlagRecord(r, victim, currentTick(), now)
			if err := store.IssueFlag(ctx, record); err != nil {
				return err
			}
			if _, err := store.LookupIssuedFlag(ctx, record.Flag); err != nil {
				return err
			}
			_, err := store.LatestIssuedFlag(ctx, victim.TeamID, victim.ChallengeID)
			return err
		},
		"AcceptFlagSubmission": func(r int) error {
			_, err := store.AcceptFlagSubmission(ctx, capture(issue(r)))
			return err
		},
		"SubmitFlagsViaServer": func(r int) error {
			record := issue(r)
			_, err := server.submitFlags(ctx, attacker.TeamID, []string{record.Flag, "PLAYIT{not-a-flag}"})
			return err
		},
		"SubmissionHistory": func(r int) error {
			if err := store.RecordSubmissions(ctx, apigateway.AdminSubmission{TeamID: attacker.TeamID, Attacker: attacker.TeamName, Flag: fmt.Sprintf("bad-%d", r), Status: "invalid", Tick: 1, SubmittedAt: now}); err != nil {
				return err
			}
			_, err := store.ListSubmissions(ctx, apigateway.SubmissionQuery{AttackFeedQuery: apigateway.AttackFeedQuery{Limit: 5, Attacker: attacker.TeamName, TickFrom: 1, TickTo: 99}, Status: "invalid", Flag: "bad"})
			return err
		},
		"DeleteSubmission": func(r int) error {
			if accepted, err := clean.AcceptFlagSubmission(ctx, capture(issue(r))); err != nil || !accepted {
				return fmt.Errorf("seed capture: %v %v", accepted, err)
			}
			page, err := clean.ListSubmissions(ctx, apigateway.SubmissionQuery{AttackFeedQuery: apigateway.AttackFeedQuery{Limit: 1}, Status: "accepted"})
			if err != nil || len(page.Items) == 0 {
				return fmt.Errorf("seed listing: %v", err)
			}
			_, err = store.DeleteSubmission(ctx, page.Items[0].ID)
			return err
		},
		"CheckerRuns": func(int) error {
			tick := currentTick()
			if _, err := store.RecordCheckerRun(ctx, run(tick, victim.TeamID, victim.ChallengeID, "check")); err != nil {
				return err
			}
			if _, err := store.ListCheckerRuns(ctx, apigateway.GameCheckerRunQuery{Limit: 5, TickID: tick, TeamID: victim.TeamID, ChallengeID: victim.ChallengeID, Phase: "check", Status: "success"}); err != nil {
				return err
			}
			_, err := store.ListCheckerRuns(ctx, apigateway.GameCheckerRunQuery{Limit: 50})
			return err
		},
		"Retention": func(r int) error {
			record := issue(r)
			if err := store.ScheduleRetentionCheck(ctx, record.Flag, now.Add(time.Minute)); err != nil {
				return err
			}
			_, err := store.RecordRetentionCheck(ctx, record, run(currentTick(), victim.TeamID, victim.ChallengeID, "get"), r%2 == 0)
			return err
		},
		"CheckerTargets": func(int) error {
			if _, err := store.ListCheckerTargets(ctx); err != nil {
				return err
			}
			if _, err := store.CheckerTargetCurrent(ctx, victim); err != nil {
				return err
			}
			if _, err := store.IsChallengeInMaintenance(ctx, victim.ChallengeID); err != nil {
				return err
			}
			_, err := store.LookupTeamName(ctx, victim.TeamID)
			return err
		},
		"LockCheckerTarget": func(int) error {
			unlock, err := store.LockCheckerTarget(ctx, victim.TeamID, victim.ChallengeID)
			if err == nil && unlock != nil {
				unlock()
			}
			return err
		},
		"Scoreboard": func(int) error {
			if _, err := store.RecomputeScoreboard(ctx); err != nil {
				return err
			}
			if _, err := store.ListScoreboard(ctx); err != nil {
				return err
			}
			_, err := store.AuditScoreboard(ctx)
			return err
		},
		"AttackFeed": func(int) error {
			_, err := store.ListAttackFeed(ctx, apigateway.AttackFeedQuery{Limit: 10, Attacker: attacker.TeamName, Victim: victim.TeamName, Service: victim.ChallengeName, TickFrom: 1, TickTo: 999})
			return err
		},
		"GameStatus": func(int) error { _, err := store.GameStatus(ctx); return err },
		"Scheduler": func(r int) error {
			if _, err := store.LoadSchedulerState(ctx, 60); err != nil {
				return err
			}
			if err := store.SaveSchedulerState(ctx, apigateway.GameSchedulerStatus{State: "stopped", IntervalSeconds: 60}, now); err != nil {
				return err
			}
			if _, err := store.AppendSchedulerEvent(ctx, schedulerEventRecord{EventType: "fault_test", Source: "test", State: "stopped", Message: fmt.Sprint(r), CreatedAt: now}); err != nil {
				return err
			}
			_, err := store.ListSchedulerEvents(ctx, apigateway.GameSchedulerEventQuery{Limit: 5, EventType: "fault_test", Source: "test", State: "stopped"})
			return err
		},
		"MatchPauseResume": func(int) error {
			if _, err := store.PauseMatch(ctx, now); err != nil {
				return err
			}
			if _, err := store.MatchStatus(ctx); err != nil {
				return err
			}
			_, err := store.ResumeMatch(ctx, now)
			if err != nil {
				// Leave the match running for the next operation.
				_, _ = clean.ResumeMatch(ctx, now)
			}
			return err
		},
		"MatchSchedule": func(int) error {
			start, end := now.Add(-time.Hour), now.Add(24*time.Hour)
			_, err := store.UpdateMatchSchedule(ctx, &start, &end)
			return err
		},
		"ReapRunningTicks": func(int) error { _, err := store.ReapRunningTicks(ctx); return err },
		"CheckerRunsWithFallback": func(r int) error {
			// A run with no stored service state forces the summary fallback.
			if _, err := db.ExecContext(ctx, `INSERT INTO checker_runs(tick_id, team_id, team_name, challenge_id, challenge_name, phase, target, checker_image, status, exit_code, message, output, checked_at)
				VALUES ($1, $2, 't', $3, 'c', 'put', 't', 'i', 'success', 0, '', '', NOW())`, currentTick(), victim.TeamID, victim.ChallengeID); err != nil {
				return err
			}
			// Drop the stored state for this key so listing must derive it from runs.
			if _, err := db.ExecContext(ctx, `DELETE FROM checker_service_states WHERE tick_id = $1 AND team_id = $2 AND challenge_id = $3`, currentTick(), victim.TeamID, victim.ChallengeID); err != nil {
				return err
			}
			_, err := store.ListCheckerRuns(ctx, apigateway.GameCheckerRunQuery{Limit: 0, Offset: -1})
			return err
		},
		"SkippedRunKeepsState": func(r int) error {
			skipped := run(currentTick(), victim.TeamID, victim.ChallengeID, "check")
			skipped.Status, skipped.ReportedServiceState = "skipped", ""
			// With a stored state the skip keeps it; without one it derives it.
			if _, err := store.RecordCheckerRun(ctx, skipped); err != nil {
				return err
			}
			if _, err := db.ExecContext(ctx, `DELETE FROM checker_service_states WHERE tick_id = $1 AND team_id = $2 AND challenge_id = $3`, currentTick(), victim.TeamID, victim.ChallengeID); err != nil {
				return err
			}
			_, err := store.RecordCheckerRun(ctx, skipped)
			return err
		},
		"SchedulerStateFull": func(r int) error {
			if err := store.SaveSchedulerState(ctx, apigateway.GameSchedulerStatus{State: "running", IntervalSeconds: 30, LastRunAt: time.Now().UTC().Format(time.RFC3339), NextRunAt: time.Now().UTC().Format(time.RFC3339), LastTickID: currentTick()}, now); err != nil {
				return err
			}
			if _, err := store.LoadSchedulerState(ctx, 60); err != nil {
				return err
			}
			if _, err := store.AppendSchedulerEvent(ctx, schedulerEventRecord{EventType: "tick_completed", Source: "scheduler", State: "running", TickID: currentTick(), CreatedAt: now}); err != nil {
				return err
			}
			_, err := store.ListSchedulerEvents(ctx, apigateway.GameSchedulerEventQuery{})
			return err
		},
		"RetentionConfirmed": func(r int) error {
			// Other ops issue token-less flags for the victim, so issue an
			// eligible one: current tick, carrying the service's checker token.
			latest := issueFlagRecord(r, victim, currentTick(), now)
			latest.CheckerToken = victim.CheckerToken
			latest.CreatedAt = time.Now() // newer than the fixture-stamped flags
			if err := clean.IssueFlag(ctx, latest); err != nil {
				return err
			}
			// Without a stored state the record must refresh it from the runs.
			if _, err := db.ExecContext(ctx, `DELETE FROM checker_service_states WHERE tick_id = $1 AND team_id = $2 AND challenge_id = $3`, latest.IssuedTick, victim.TeamID, victim.ChallengeID); err != nil {
				return err
			}
			getRun := checkerRunRecord{TickID: latest.IssuedTick, TeamID: latest.OwnerTeamID, ChallengeID: latest.ChallengeID, Phase: "get", Status: "failed", ExitCode: flagMissingExitCode, CheckedAt: now}
			recorded, err := store.RecordRetentionCheck(ctx, latest, getRun, true)
			if err != nil {
				return err
			}
			if !recorded {
				return fmt.Errorf("the latest flag must be eligible for a retention record")
			}
			// With no failure row and no stored state, a passing re-check must
			// derive the state from the tick's runs.
			if _, err := db.ExecContext(ctx, `DELETE FROM flag_retention_failures WHERE flag = $1`, latest.Flag); err != nil {
				return err
			}
			if _, err := db.ExecContext(ctx, `DELETE FROM checker_service_states WHERE tick_id = $1 AND team_id = $2 AND challenge_id = $3`, latest.IssuedTick, victim.TeamID, victim.ChallengeID); err != nil {
				return err
			}
			getRun.Status, getRun.ExitCode = "success", 0
			_, err = store.RecordRetentionCheck(ctx, latest, getRun, false)
			return err
		},
	}
	names := make([]string, 0, len(ops))
	for name := range ops {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		t.Run(name, func(t *testing.T) {
			dbtest.SweepFaults(t, faults, name, ops[name])
			dbtest.SweepRowFaults(t, faults, name, ops[name])
			dbtest.SweepResultFaults(t, faults, name, ops[name])
		})
	}
}

func issueFlagRecord(run int, owner checkerTarget, tick int, now time.Time) issuedFlagRecord {
	return issuedFlagRecord{Flag: fmt.Sprintf("PLAYIT{direct-%d-%d}", run, time.Now().UnixNano()), OwnerTeamID: owner.TeamID, OwnerTeamName: owner.TeamName, ChallengeID: owner.ChallengeID, ChallengeName: owner.ChallengeName, IssuedTick: tick, ExpiresTick: tick + 5, CreatedAt: now, NextCheckAt: now}
}
