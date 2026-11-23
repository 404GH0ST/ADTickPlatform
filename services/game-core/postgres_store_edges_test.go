package main

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"testing"
	"time"

	"adplatform/internal/platform/database"
	"adplatform/internal/platform/database/dbtest"
	"adplatform/internal/services/apigateway"
)

func migratedGameDB(t *testing.T, prefix string, seeds bool) *sql.DB {
	t.Helper()
	db := dbtest.IsolatedSchema(t, "SUBMISSION_TEST_POSTGRES_DSN", prefix)
	if err := database.RunEmbeddedMigrationsWithOptions(context.Background(), db, database.MigrationOptions{IncludeSeeds: seeds}); err != nil {
		t.Fatal(err)
	}
	return db
}

func mustExec(t *testing.T, db *sql.DB, statement string, args ...any) {
	t.Helper()
	if _, err := db.Exec(statement, args...); err != nil {
		t.Fatalf("%s: %v", statement, err)
	}
}

func TestPostgresAcceptRejectsEveryInvalidState(t *testing.T) {
	ctx := context.Background()
	db := migratedGameDB(t, "gc_accept_states", false)
	store := newPostgresGameStore(db)
	capture := acceptedFlagSubmission{Flag: "PLAYIT{x}", SubmittingTeam: 101, ChallengeID: 1, ExpiresTick: 5, SubmittedAt: time.Now()}
	expectInvalid := func(stage string, submission acceptedFlagSubmission) {
		t.Helper()
		if accepted, err := store.AcceptFlagSubmission(ctx, submission); accepted || !errors.Is(err, errFlagNoLongerValid) {
			t.Fatalf("%s: want errFlagNoLongerValid, got %v %v", stage, accepted, err)
		}
	}
	expectInvalid("no match row", capture)
	mustExec(t, db, `INSERT INTO game_match_state(singleton, state) VALUES (TRUE, 'paused')`)
	expectInvalid("paused match", capture)
	mustExec(t, db, `UPDATE game_match_state SET state = 'running'`)
	expectInvalid("no tick yet", capture)
	mustExec(t, db, `INSERT INTO game_ticks(id, status) VALUES (7, 'running')`)
	expectInvalid("expired", capture)
	capture.ExpiresTick = 9
	expectInvalid("unknown submitting team", capture)
	mustExec(t, db, `INSERT INTO teams(id,name,email,join_key) VALUES(101,'attacker','a@test.local','a')`)
	expectInvalid("unknown challenge", capture)
	mustExec(t, db, `INSERT INTO challenges(id, name, maintenance) VALUES (1, 'web', TRUE)`)
	expectInvalid("maintenance", capture)

	if _, err := store.StartNextTick(ctx, time.Now()); !errors.Is(err, errTickInProgress) {
		t.Fatalf("a running tick must block the next one: %v", err)
	}
	if current, err := store.CheckerTargetCurrent(ctx, checkerTarget{TeamID: 999, ChallengeID: 999}); err != nil || current {
		t.Fatalf("an unknown target is not current: %v %v", current, err)
	}
	if blocked, err := store.IsChallengeInMaintenance(ctx, 999); err != nil || blocked {
		t.Fatalf("an unknown challenge is not blocked: %v %v", blocked, err)
	}
}

func TestPostgresCheckerConcurrencyLimitFollowsPool(t *testing.T) {
	unbounded, _ := sql.Open("pgx", "postgres://nobody@127.0.0.1:1/none")
	defer unbounded.Close()
	if got := newPostgresGameStore(unbounded).CheckerTargetConcurrencyLimit(); got != 12 {
		t.Fatalf("an unbounded pool: %d", got)
	}
	unbounded.SetMaxOpenConns(2)
	if got := newPostgresGameStore(unbounded).CheckerTargetConcurrencyLimit(); got != 1 {
		t.Fatalf("a tiny pool: %d", got)
	}
	unbounded.SetMaxOpenConns(25)
	if got := newPostgresGameStore(unbounded).CheckerTargetConcurrencyLimit(); got != 12 {
		t.Fatalf("half the pool is reserved: %d", got)
	}
}

func TestPostgresMatchTransitionsGuardState(t *testing.T) {
	ctx := context.Background()
	now := time.Now()
	store := newPostgresGameStore(migratedGameDB(t, "gc_match_states", false))
	if _, err := store.PauseMatch(ctx, now); !errors.Is(err, errContestNotStarted) {
		t.Fatalf("pause before start: %v", err)
	}
	if status, err := store.ResumeMatch(ctx, now); err != nil || status.State == "running" {
		t.Fatalf("resume before start is a no-op: %+v %v", status, err)
	}
	if _, err := store.StartMatch(ctx, now); err != nil {
		t.Fatal(err)
	}
	if status, err := store.StartMatch(ctx, now); err != nil || status.State != "running" {
		t.Fatalf("restart is idempotent: %+v %v", status, err)
	}
	if _, err := store.StopMatch(ctx, now); err != nil {
		t.Fatal(err)
	}
	if status, err := store.StopMatch(ctx, now); err != nil || status.State != "finished" {
		t.Fatalf("stop is idempotent: %+v %v", status, err)
	}
	for name, op := range map[string]func() error{
		"start":  func() error { _, err := store.StartMatch(ctx, now); return err },
		"pause":  func() error { _, err := store.PauseMatch(ctx, now); return err },
		"resume": func() error { _, err := store.ResumeMatch(ctx, now); return err },
	} {
		if err := op(); !errors.Is(err, errContestOver) {
			t.Errorf("%s after finish: %v", name, err)
		}
	}
	later := now.Add(time.Minute)
	reopened, err := store.RestartMatch(ctx, later)
	if err != nil || reopened.State != "running" || reopened.EndedAt != "" || !reopened.AcceptingSubmissions {
		t.Fatalf("restarting a finished match reopens it: %+v %v", reopened, err)
	}
	if reopened.StartedAt != later.UTC().Format(time.RFC3339) {
		t.Fatalf("a restarted match starts at the restart time: %q", reopened.StartedAt)
	}
	if status, err := store.RestartMatch(ctx, later.Add(time.Minute)); err != nil || status.StartedAt != reopened.StartedAt {
		t.Fatalf("restarting a running match is a no-op: %+v %v", status, err)
	}
}

func TestPostgresSchedulerPersistence(t *testing.T) {
	ctx := context.Background()
	db := migratedGameDB(t, "gc_scheduler", false)
	mustExec(t, db, `INSERT INTO game_ticks(id, status) VALUES (4, 'completed')`)
	store := newPostgresGameStore(db)
	if status, err := store.LoadSchedulerState(ctx, 45); err != nil || status.IntervalSeconds != 45 {
		t.Fatalf("no saved state uses the default interval: %+v %v", status, err)
	}
	if err := store.SaveSchedulerState(ctx, apigateway.GameSchedulerStatus{State: "running", LastRunAt: "yesterday"}, time.Now()); err == nil {
		t.Fatal("an unparseable last run must be rejected")
	}
	saved := apigateway.GameSchedulerStatus{State: "running", IntervalSeconds: 30, LastRunAt: "2026-11-19T10:00:00Z", NextRunAt: "2026-11-19T10:00:30Z", LastTickID: 4}
	if err := store.SaveSchedulerState(ctx, saved, time.Now()); err != nil {
		t.Fatal(err)
	}
	loaded, err := store.LoadSchedulerState(ctx, 60)
	if err != nil || loaded.LastTickID != 4 || loaded.LastRunAt == "" || loaded.NextRunAt == "" {
		t.Fatalf("saved scheduler state must round-trip: %+v %v", loaded, err)
	}
	if _, err := store.AppendSchedulerEvent(ctx, schedulerEventRecord{EventType: "tick_completed", Source: "scheduler", State: "running", TickID: 4, CreatedAt: time.Now()}); err != nil {
		t.Fatal(err)
	}
	page, err := store.ListSchedulerEvents(ctx, apigateway.GameSchedulerEventQuery{Limit: 0, Offset: -2})
	if err != nil || page.Limit != 25 || page.Offset != 0 || len(page.Items) != 1 || page.Items[0].TickID != 4 {
		t.Fatalf("event paging defaults and tick ids: %+v %v", page, err)
	}
}

func TestPostgresCheckerRunSummariesFallBackToRuns(t *testing.T) {
	ctx := context.Background()
	db := migratedGameDB(t, "gc_run_summaries", false)
	store := newPostgresGameStore(db)
	mustExec(t, db, `INSERT INTO game_ticks(id, status) VALUES (1, 'completed')`)
	mustExec(t, db, `INSERT INTO teams(id, name, email, join_key) VALUES (101, 'A', 'a@x.test', 'k')`)
	mustExec(t, db, `INSERT INTO challenges(id, name) VALUES (1, 'web')`)
	now := time.Now().UTC()
	for _, phase := range []string{"put", "get", "check"} {
		status := "success"
		if phase == "check" {
			status = "failed"
		}
		mustExec(t, db, `INSERT INTO checker_runs(tick_id, team_id, team_name, challenge_id, challenge_name, phase, target, checker_image, status, exit_code, message, output, checked_at)
			VALUES (1, 101, 'A', 1, 'web', $1, 't', 'img', $2, 0, '', '', $3)`, phase, status, now)
	}
	page, err := store.ListCheckerRuns(ctx, apigateway.GameCheckerRunQuery{Limit: 0, Offset: -1})
	if err != nil || page.Limit != 25 || page.Offset != 0 || len(page.Items) != 3 {
		t.Fatalf("run paging defaults: %+v %v", page, err)
	}
	for _, run := range page.Items {
		if run.ServiceState == "" {
			t.Fatalf("runs without a stored state must get a summary derived from the runs: %+v", run)
		}
	}
	feed, err := store.ListAttackFeed(ctx, apigateway.AttackFeedQuery{Limit: -1, Offset: -1})
	if err != nil || feed.Limit != 12 || feed.Offset != 0 {
		t.Fatalf("attack feed defaults: %+v %v", feed, err)
	}
}

func TestPostgresRetentionCheckOutcomes(t *testing.T) {
	ctx := context.Background()
	db := migratedGameDB(t, "gc_retention_outcomes", true)
	mustExec(t, db, `UPDATE challenges SET published = TRUE`)
	store := newPostgresGameStore(db)
	server := newTickServer(store)
	if _, err := store.StartMatch(ctx, time.Now()); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if _, err := server.advanceTick(ctx); err != nil {
			t.Fatal(err)
		}
	}
	targets, err := store.ListCheckerTargets(ctx)
	if err != nil || len(targets) == 0 {
		t.Fatal(err)
	}
	target := targets[0]
	latest, err := store.LatestIssuedFlag(ctx, target.TeamID, target.ChallengeID)
	if err != nil {
		t.Fatal(err)
	}
	get := func(f issuedFlagRecord, status string, exit int) checkerRunRecord {
		return checkerRunRecord{TickID: f.IssuedTick, TeamID: f.OwnerTeamID, ChallengeID: f.ChallengeID, Phase: "get", Status: status, ExitCode: exit, CheckedAt: time.Now()}
	}

	if _, err := store.RecordRetentionCheck(ctx, latest, checkerRunRecord{Phase: "put"}, false); err == nil {
		t.Fatal("a run for another flag must be rejected")
	}
	older := latest
	older.IssuedTick--
	older.Flag = "PLAYIT{older}"
	if recorded, err := store.RecordRetentionCheck(ctx, older, get(older, "failed", 42), true); err != nil || recorded {
		t.Fatalf("a superseded flag is not penalized: %v %v", recorded, err)
	}
	stale := latest
	stale.CheckerToken = "rotated"
	if recorded, err := store.RecordRetentionCheck(ctx, stale, get(stale, "failed", 42), true); err != nil || recorded {
		t.Fatalf("a rotated checker token is not penalized: %v %v", recorded, err)
	}
	missingTeam := latest
	missingTeam.OwnerTeamID = 9999
	if recorded, err := store.RecordRetentionCheck(ctx, missingTeam, get(missingTeam, "failed", 42), true); err != nil || recorded {
		t.Fatalf("an unknown service is not penalized: %v %v", recorded, err)
	}
	if recorded, err := store.RecordRetentionCheck(ctx, latest, get(latest, "success", 0), false); err != nil || !recorded {
		t.Fatalf("a passing re-check is recorded: %v %v", recorded, err)
	}
	if recorded, err := store.RecordRetentionCheck(ctx, latest, get(latest, "failed", 42), true); err != nil || !recorded {
		t.Fatalf("a confirmed missing flag is penalized: %v %v", recorded, err)
	}
	var penalty float64
	if err := db.QueryRow(`SELECT penalty_points FROM flag_retention_failures WHERE flag = $1`, latest.Flag).Scan(&penalty); err != nil || penalty < 10 {
		t.Fatalf("the defense penalty must be recorded: %v %v", penalty, err)
	}
	// An unconfirmed failure afterwards must not lower a confirmed penalty.
	if _, err := store.RecordRetentionCheck(ctx, latest, get(latest, "failed", 1), false); err != nil {
		t.Fatal(err)
	}
	var after float64
	if err := db.QueryRow(`SELECT penalty_points FROM flag_retention_failures WHERE flag = $1`, latest.Flag).Scan(&after); err != nil || after != penalty {
		t.Fatalf("a confirmed penalty must not be downgraded: %v -> %v (%v)", penalty, after, err)
	}
}

func TestGameCorePureHelpers(t *testing.T) {
	for status, phase := range map[string]string{"recovering": "put", "flag_not_found": "get", "faulty": "check", "ok": "check", "down": "check", "weird": ""} {
		if got := inferReportedStatePhase(status); got != phase {
			t.Errorf("inferReportedStatePhase(%q) = %q", status, got)
		}
	}
	for _, status := range []string{"ok", "recovering", "flag_not_found", "faulty", "down", "weird"} {
		if fallbackReportedStateMessage(status, "") == "" {
			t.Errorf("status %q needs a fallback message", status)
		}
	}
	if fallbackReportedStateMessage("ok", " custom ") != "custom" {
		t.Fatal("an explicit message wins")
	}
	if retentionPenalty(-3) != retentionPenalty(0) || retentionPenalty(0) < 10 {
		t.Fatal("negative opponent counts clamp to zero and the floor is 10")
	}

	at := time.Now()
	submissions := []acceptedFlagSubmission{{SubmittedAt: at, SubmittingTeam: 2, Flag: "b"}, {SubmittedAt: at, SubmittingTeam: 1, Flag: "z"}, {SubmittedAt: at, SubmittingTeam: 1, Flag: "a"}, {SubmittedAt: at.Add(-time.Second), SubmittingTeam: 9}}
	sortAcceptedSubmissions(submissions)
	order := []string{}
	for _, s := range submissions {
		order = append(order, s.Flag)
	}
	if strings.Join(order, ",") != ",a,z,b" {
		t.Fatalf("submissions sort by time, team, then flag: %v", order)
	}

	stored := []apigateway.ScoreRowAlias{{Team: "A", Rank: 1, Total: 10}, {Team: "Gone", Rank: 2}}
	replayed := []apigateway.ScoreRowAlias{{Team: "A", Rank: 1, Total: 12, Attack: 2}, {Team: "New", Rank: 2}}
	report := buildScoringAuditReport(stored, replayed)
	if report.Status != "mismatch" || report.MismatchCount != 4 {
		t.Fatalf("audit must flag changed, missing, and extra rows: %+v", report)
	}
	if clean := buildScoringAuditReport(stored, stored); clean.Status != "ok" || clean.MismatchCount != 0 {
		t.Fatalf("identical boards agree: %+v", clean)
	}
}

func TestPostgresGameStatusAndTickParsing(t *testing.T) {
	ctx := context.Background()
	store := newPostgresGameStore(migratedGameDB(t, "gc_status_empty", false))
	if status, err := store.GameStatus(ctx); err != nil || status.CurrentTick != nil {
		t.Fatalf("an empty game has no current tick: %+v %v", status, err)
	}
	if _, err := store.CompleteTick(ctx, apigateway.GameTickStatus{ID: 1, CompletedAt: "not-a-time"}); err == nil {
		t.Fatal("an unparseable completion time must be rejected")
	}
}

func TestPostgresSchedulerRejectsBadNextRun(t *testing.T) {
	store := newPostgresGameStore(migratedGameDB(t, "gc_sched_next", false))
	if err := store.SaveSchedulerState(context.Background(), apigateway.GameSchedulerStatus{State: "running", NextRunAt: "soon"}, time.Now()); err == nil {
		t.Fatal("an unparseable next run must be rejected")
	}
}

func TestPostgresAttackFeedFiltersTreatWildcardsLiterally(t *testing.T) {
	ctx := context.Background()
	db := migratedGameDB(t, "gc_attack_filter", true)
	for _, row := range [][2]string{{"filter-1", "Alpha_Team"}, {"filter-2", "AlphaXTeam"}} {
		mustExec(t, db, `INSERT INTO attack_events (id, attacker, victim, service, tick, verdict, created_at, challenge_id)
			VALUES ($1, $2, 'Victim', 'svc', 1, 'ok', NOW(), (SELECT MIN(id) FROM challenges))`, row[0], row[1])
	}
	store := newPostgresGameStore(db)
	for filter, want := range map[string]int{"alpha": 2, "Alpha_Team": 1, "%": 0, "A%a": 0, `\`: 0} {
		feed, err := store.ListAttackFeed(ctx, apigateway.AttackFeedQuery{Limit: 10, Attacker: filter})
		if err != nil || feed.TotalCount != want {
			t.Fatalf("filter %q: got %d rows, want %d: %v", filter, feed.TotalCount, want, err)
		}
	}
}
