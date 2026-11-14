package main

import (
	"context"
	"sync"
	"testing"
	"time"

	"adplatform/internal/platform/database"
)

func TestPostgresCheckerRecoversStaleResetGate(t *testing.T) {
	db := isolatedSubmissionDB(t)
	ctx := context.Background()
	if err := database.RunEmbeddedMigrationsWithOptions(ctx, db, database.MigrationOptions{IncludeSeeds: false}); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `
		INSERT INTO teams(id,name,email,join_key) VALUES(101,'Team Alpha','a@test.local','a');
		INSERT INTO challenges(id,name,published) VALUES(1,'stale-reset',TRUE);
		INSERT INTO service_instances(team_id,challenge_id,runtime_kind,runtime_status,container_name,baseline_image,checker_image,endpoint,ssh_host,checker_token)
		VALUES(101,1,'docker','ready','stale-reset','test','checker','10.80.1.11:8080','10.80.1.11','new-token');
		INSERT INTO team_service_states(team_id,challenge_id,endpoint,status,checker,unlocked,ssh_hint,last_event,reset_cooldown)
		VALUES(101,1,'10.80.1.11:8080','resetting','warning',FALSE,'','factory reset started via participant API','resetting');
	`); err != nil {
		t.Fatal(err)
	}
	store := &postgresGameStore{db: db}
	target := checkerTarget{TeamID: 101, ChallengeID: 1, CheckerToken: "new-token"}
	if current, err := store.CheckerTargetCurrent(ctx, target); err != nil || current {
		t.Fatalf("active reset gate opened early: current=%v err=%v", current, err)
	}
	if _, err := db.ExecContext(ctx, `UPDATE service_instances SET updated_at=NOW()-INTERVAL '6 minutes' WHERE team_id=101 AND challenge_id=1`); err != nil {
		t.Fatal(err)
	}
	if current, err := store.CheckerTargetCurrent(ctx, target); err != nil || !current {
		t.Fatalf("stale reset gate remained closed: current=%v err=%v", current, err)
	}
	var status string
	if err := db.QueryRowContext(ctx, `SELECT status FROM team_service_states WHERE team_id=101 AND challenge_id=1`).Scan(&status); err != nil || status != "degraded" {
		t.Fatalf("stale reset state not recovered: %q %v", status, err)
	}
}

func TestPostgresRetentionPenaltySurvivesRecoveryAndReload(t *testing.T) {
	db := isolatedSubmissionDB(t)
	ctx := context.Background()
	if err := database.RunEmbeddedMigrationsWithOptions(ctx, db, database.MigrationOptions{IncludeSeeds: false}); err != nil {
		t.Fatal(err)
	}
	_, err := db.ExecContext(ctx, `
 INSERT INTO teams(id,name,email,join_key) VALUES(101,'Team Alpha','a@test.local','a'),(102,'Team Delta','b@test.local','b');
 INSERT INTO challenges(id,name,published) VALUES(1,'retention',TRUE);
 INSERT INTO game_match_state(singleton,state) VALUES(TRUE,'running');
 INSERT INTO game_ticks(id,status) VALUES(1,'completed');
 INSERT INTO service_instances(team_id,challenge_id,runtime_kind,runtime_status,container_name,baseline_image,checker_image,endpoint,ssh_host,checker_token)
 VALUES(101,1,'docker','ready','retention-test','test','checker','10.80.1.11:8080','10.80.1.11','token-before-reset');
 `)
	if err != nil {
		t.Fatal(err)
	}
	store := &postgresGameStore{db: db}
	now := time.Now().UTC()
	flag := issuedFlagRecord{Flag: "retained-flag", OwnerTeamID: 101, OwnerTeamName: "Team Alpha", ChallengeID: 1, ChallengeName: "retention", IssuedTick: 1, ExpiresTick: 1, CreatedAt: now, Metadata: `{"path":"flag.txt","opaque":"slot"}`, CheckerToken: "token-before-reset", NextCheckAt: now.Add(time.Minute)}
	if err = store.IssueFlag(ctx, flag); err != nil {
		t.Fatal(err)
	}
	restored, err := store.LatestIssuedFlag(ctx, 101, 1)
	if err != nil {
		t.Fatal(err)
	}
	if restored.Metadata != flag.Metadata || restored.CheckerToken != flag.CheckerToken || restored.NextCheckAt.IsZero() {
		t.Fatalf("lost persistent retention snapshot: %+v", restored)
	}
	run := checkerRunRecord{TickID: 1, TeamID: 101, TeamName: "Team Alpha", ChallengeID: 1, ChallengeName: "retention", Target: "10.80.1.11:8080", CheckerImage: "checker", CheckedAt: now, Status: "success"}
	for _, phase := range []string{"put", "get", "check"} {
		run.Phase = phase
		if _, err = store.RecordCheckerRun(ctx, run); err != nil {
			t.Fatal(err)
		}
	}
	baseline := retentionTeamScore(t, store)
	if baseline.SLA <= 0 {
		t.Fatalf("expected healthy baseline: %+v", baseline)
	}
	run.Phase = "get"
	run.Status = "failed"
	run.ExitCode = flagMissingExitCode
	for i := 0; i < 3; i++ {
		if applied, err := store.RecordRetentionCheck(ctx, restored, run, true); err != nil || !applied {
			t.Fatalf("record retention: %v %v", applied, err)
		}
	}
	var count int
	if err = db.QueryRowContext(ctx, `SELECT count(*) FROM flag_retention_failures`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("duplicate penalty count=%d", count)
	}
	// New process/store and a later successful checker report cannot erase loss.
	store = &postgresGameStore{db: db}
	run.Status = "success"
	run.ExitCode = 0
	run.Phase = "check"
	run.ReportedServiceState = "ok"
	if _, err = store.RecordCheckerRun(ctx, run); err != nil {
		t.Fatal(err)
	}
	row := retentionTeamScore(t, store)
	if row.Defense != -10 || row.SLA != 0 || row.Total != -10 || len(row.Services) != 1 || row.Services[0].Defense != -10 || row.Services[0].SLA != 0 {
		t.Fatalf("wrong persistent penalty: %+v", row)
	}
	audit, err := store.AuditScoreboard(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if audit.MismatchCount != 0 {
		t.Fatalf("audit inconsistent: %+v", audit)
	}
	// A reset rotating the runtime credential invalidates outstanding snapshots.
	if _, err = db.ExecContext(ctx, `UPDATE service_instances SET checker_token='after-reset'; DELETE FROM flag_retention_failures;`); err != nil {
		t.Fatal(err)
	}
	run.Phase = "get"
	run.Status = "failed"
	run.ExitCode = 42
	if applied, err := store.RecordRetentionCheck(ctx, restored, run, true); err != nil || applied {
		t.Fatalf("stale reset snapshot penalized: %v %v", applied, err)
	}
	if _, err = db.ExecContext(ctx, `INSERT INTO teams(id,name,email,join_key) SELECT id,'Extra-'||id,id||'@test.local','key-'||id FROM generate_series(103,108) id; INSERT INTO game_ticks(id,status) VALUES(2,'completed')`); err != nil {
		t.Fatal(err)
	}
	newer := restored
	newer.Flag = "new-tick-flag"
	newer.IssuedTick = 2
	newer.ExpiresTick = 2
	newer.CheckerToken = "after-reset"
	if err = store.IssueFlag(ctx, newer); err != nil {
		t.Fatal(err)
	}
	run.TickID = 2
	run.Status = "failed"
	run.ExitCode = 42
	run.ReportedServiceState = ""
	if applied, err := store.RecordRetentionCheck(ctx, newer, run, true); err != nil || !applied {
		t.Fatalf("scaled retention penalty: %v %v", applied, err)
	}
	row = retentionTeamScore(t, store)
	if row.Defense != -20 || row.Services[0].Defense != -20 {
		t.Fatalf("SQL penalty did not scale for 8 teams: %+v", row)
	}
	if _, err = db.ExecContext(ctx, `INSERT INTO teams(id,name,email,join_key) VALUES(500,'Late','late@test.local','late')`); err != nil {
		t.Fatal(err)
	}
	if _, err = store.RecordRetentionCheck(ctx, newer, run, true); err != nil {
		t.Fatal(err)
	}
	if row = retentionTeamScore(t, store); row.Defense != -20 {
		t.Fatalf("persisted penalty changed on retry: %+v", row)
	}

	// Retention recording must not invert acceptance's tick/match lock order.
	concurrentCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	var wg sync.WaitGroup
	errorsCh := make(chan error, 16)
	run.Status = "success"
	run.ExitCode = 0
	for i := 0; i < 8; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			_, err := store.RecordRetentionCheck(concurrentCtx, newer, run, false)
			errorsCh <- err
		}()
		go func() {
			defer wg.Done()
			_, err := store.AcceptFlagSubmission(concurrentCtx, acceptedFlagSubmission{Flag: newer.Flag, SubmittingTeam: 102, AttackerName: "Team Delta", VictimName: "Team Alpha", ChallengeID: 1, ChallengeName: "retention", ExpiresTick: 2, SubmittedAt: now})
			errorsCh <- err
		}()
	}
	wg.Wait()
	close(errorsCh)
	for err := range errorsCh {
		if err != nil {
			t.Fatalf("concurrent capture/retention failed: %v", err)
		}
	}

}

// On Postgres every store call honours its context. A tick that hits its
// timeout must still persist the runs it covered and complete.
func TestPostgresTickTimeoutRecordsRunsAndCompletes(t *testing.T) {
	db := isolatedSubmissionDB(t)
	ctx := context.Background()
	if err := database.RunEmbeddedMigrationsWithOptions(ctx, db, database.MigrationOptions{IncludeSeeds: false}); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `
 INSERT INTO teams(id,name,email,join_key) VALUES(101,'Team Alpha','a@test.local','a'),(102,'Team Delta','b@test.local','b');
 INSERT INTO challenges(id,name,published,checker_image) VALUES(1,'timeout',TRUE,'checker');
 INSERT INTO game_match_state(singleton,state) VALUES(TRUE,'running');
 INSERT INTO service_instances(team_id,challenge_id,runtime_kind,runtime_status,container_name,baseline_image,checker_image,endpoint,ssh_host,checker_token)
 VALUES(101,1,'docker','ready','timeout-a','test','checker','10.80.1.11:8080','10.80.1.11','token-a'),
       (102,1,'docker','ready','timeout-b','test','checker','10.80.1.12:8080','10.80.1.12','token-b');
 INSERT INTO team_service_states(team_id,challenge_id,endpoint,status,checker,unlocked,ssh_hint,last_event,reset_cooldown)
 VALUES(101,1,'10.80.1.11:8080','healthy','ok',FALSE,'','','ready'),(102,1,'10.80.1.12:8080','healthy','ok',FALSE,'','','ready');
 `); err != nil {
		t.Fatal(err)
	}
	srv := newGameCoreServer("test", newPostgresGameStore(db), blockingCheckerClient{}, newFlagCodec("test-secret", "PLAYIT"), &testGameScheduler{}, []string{"put", "get", "check"}, 15).
		WithWarmupRequired(false).WithAutoTickOnMatchStart(false).WithTickTimeout(200 * time.Millisecond)
	srv.scoringDebounce = time.Hour
	tick, err := srv.advanceTick(ctx)
	if err != nil {
		t.Fatalf("tick timeout failed the tick: %v", err)
	}
	var status string
	var runs int
	if err := db.QueryRowContext(ctx, `SELECT status FROM game_ticks WHERE id=$1`, tick.ID).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM checker_runs WHERE tick_id=$1`, tick.ID).Scan(&runs); err != nil {
		t.Fatal(err)
	}
	if status != "completed" || runs == 0 {
		t.Fatalf("expected a completed tick with persisted runs, got status=%q runs=%d", status, runs)
	}
}
