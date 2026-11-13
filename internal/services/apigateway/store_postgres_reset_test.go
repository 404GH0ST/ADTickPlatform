package apigateway

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"testing"
	"time"

	"adplatform/internal/platform/database"
)

func TestPostgresParticipantResetChargesOutstandingFlagBeforeRuntime(t *testing.T) {
	dsn := os.Getenv("TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("TEST_POSTGRES_DSN is not set")
	}
	ctx := context.Background()
	base, err := database.OpenPostgres(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	schema := fmt.Sprintf("reset_loss_test_%d", time.Now().UnixNano())
	if _, err = base.ExecContext(ctx, "CREATE SCHEMA "+schema); err != nil {
		base.Close()
		t.Fatal(err)
	}
	parsed, err := url.Parse(dsn)
	if err != nil {
		t.Fatal(err)
	}
	query := parsed.Query()
	query.Set("search_path", schema)
	parsed.RawQuery = query.Encode()
	db, err := database.OpenPostgres(ctx, parsed.String())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		db.Close()
		base.ExecContext(context.Background(), "DROP SCHEMA "+schema+" CASCADE")
		base.Close()
	})
	if err = database.RunEmbeddedMigrationsWithOptions(ctx, db, database.MigrationOptions{IncludeSeeds: false}); err != nil {
		t.Fatal(err)
	}
	_, err = db.ExecContext(ctx, `
		INSERT INTO teams(id,name,email,join_key) SELECT id,'Team-'||id,id||'@test.local','key-'||id FROM generate_series(101,108) id;
		INSERT INTO challenges(id,name,published) VALUES(1,'reset-test',TRUE);
		INSERT INTO game_match_state(singleton,state) VALUES(TRUE,'running');
		INSERT INTO game_ticks(id,status) VALUES(1,'completed');
		INSERT INTO service_instances(team_id,challenge_id,runtime_kind,runtime_status,container_name,baseline_image,checker_image,endpoint,ssh_host,checker_token)
		VALUES(101,1,'docker','ready','reset-test','test','checker','10.80.1.11:8080','10.80.1.11','before-reset');
		INSERT INTO team_service_states(team_id,challenge_id,endpoint,status,checker,unlocked,ssh_hint,last_event,reset_cooldown)
		VALUES(101,1,'10.80.1.11:8080','stable','passing',FALSE,'','', 'ready');
		INSERT INTO issued_flags(flag,owner_team_id,owner_team_name,challenge_id,challenge_name,issued_tick,expires_tick,checker_token_snapshot)
		VALUES('reset-flag',101,'Team-101',1,'reset-test',1,1,'before-reset');
		INSERT INTO checker_service_states(tick_id,team_id,challenge_id,service_state) VALUES(1,101,1,'ok');
	`)
	if err != nil {
		t.Fatal(err)
	}
	store := &postgresStore{db: db}
	prepared, err := store.PrepareFactoryResetService(ctx, 101, 1)
	if err != nil || prepared.PenaltyPoints != 20 {
		t.Fatalf("preparation did not charge scaled penalty: %+v %v", prepared, err)
	}
	var count int
	if err = db.QueryRowContext(ctx, `SELECT COUNT(*) FROM flag_retention_failures`).Scan(&count); err != nil || count != 1 {
		t.Fatalf("preparation penalty not durable: count=%d err=%v", count, err)
	}
	if err = store.MarkFactoryResetFailure(ctx, 101, 1); err != nil {
		t.Fatal(err)
	}
	if err = db.QueryRowContext(ctx, `SELECT COUNT(*) FROM flag_retention_failures`).Scan(&count); err != nil || count != 1 {
		t.Fatalf("runtime failure removed penalty: count=%d err=%v", count, err)
	}
	if _, err = db.ExecContext(ctx, `UPDATE service_instances SET runtime_status='ready' WHERE team_id=101 AND challenge_id=1`); err != nil {
		t.Fatal(err)
	}
	if prepared, err = store.PrepareFactoryResetService(ctx, 101, 1); err != nil {
		t.Fatal(err)
	}
	result, err := store.CompleteFactoryResetService(ctx, 101, 1)
	if err != nil || prepared.PenaltyPoints != 0 || result.PenaltyPoints != 0 {
		t.Fatalf("repeated reset charged an already lost flag: prepared=%+v completed=%+v err=%v", prepared, result, err)
	}
	var penalty float64
	if err = db.QueryRowContext(ctx, `SELECT COUNT(*),MAX(penalty_points) FROM flag_retention_failures WHERE team_id=101 AND challenge_id=1`).Scan(&count, &penalty); err != nil || count != 1 || penalty != 20 {
		t.Fatalf("wrong reset loss: count=%d penalty=%v err=%v", count, penalty, err)
	}
	if _, err = store.PrepareFactoryResetService(ctx, 101, 1); err != nil {
		t.Fatal(err)
	}
	if _, err = store.PrepareFactoryResetService(ctx, 101, 1); !errors.Is(err, ErrServiceUnavailable) {
		t.Fatalf("concurrent reset was not rejected: %v", err)
	}
	if _, err = db.ExecContext(ctx, `UPDATE service_instances SET updated_at=NOW()-INTERVAL '6 minutes' WHERE team_id=101 AND challenge_id=1`); err != nil {
		t.Fatal(err)
	}
	if _, err = store.PrepareFactoryResetService(ctx, 101, 1); err != nil {
		t.Fatalf("stale reset could not be retried: %v", err)
	}
	if _, err = store.CompleteFactoryResetService(ctx, 101, 1); err != nil {
		t.Fatal(err)
	}
	if err = db.QueryRowContext(ctx, `SELECT COUNT(*),MAX(penalty_points) FROM flag_retention_failures WHERE team_id=101 AND challenge_id=1`).Scan(&count, &penalty); err != nil || count != 1 || penalty != 20 {
		t.Fatalf("reset charged same flag twice: count=%d penalty=%v err=%v", count, penalty, err)
	}
	var forfeited bool
	if err = db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM flag_retention_failures WHERE tick_id=1 AND team_id=101 AND challenge_id=1)`).Scan(&forfeited); err != nil || !forfeited {
		t.Fatalf("tick SLA not forfeited: %v %v", forfeited, err)
	}
	_, err = db.ExecContext(ctx, `
		INSERT INTO service_instances(team_id,challenge_id,runtime_kind,runtime_status,container_name,baseline_image,checker_image,endpoint,ssh_host,checker_token)
		VALUES(102,1,'docker','ready','reset-test-102','test','checker','10.80.1.12:8080','10.80.1.12','before-finish');
		INSERT INTO team_service_states(team_id,challenge_id,endpoint,status,checker,unlocked,ssh_hint,last_event,reset_cooldown)
		VALUES(102,1,'10.80.1.12:8080','stable','passing',FALSE,'','','ready');
		INSERT INTO issued_flags(flag,owner_team_id,owner_team_name,challenge_id,challenge_name,issued_tick,expires_tick,checker_token_snapshot)
		VALUES('finish-race-flag',102,'Team-102',1,'reset-test',1,1,'before-finish');
	`)
	if err != nil {
		t.Fatal(err)
	}
	prepared, err = store.PrepareFactoryResetService(ctx, 102, 1)
	if err != nil || prepared.PenaltyPoints != 20 {
		t.Fatalf("finish race was not charged at preparation: %+v %v", prepared, err)
	}
	if _, err = db.ExecContext(ctx, `UPDATE game_match_state SET state='finished',ended_at=NOW() WHERE singleton=TRUE`); err != nil {
		t.Fatal(err)
	}
	result, err = store.CompleteFactoryResetService(ctx, 102, 1)
	if err != nil || result.PenaltyPoints != 0 {
		t.Fatalf("completion charged after finish: %+v %v", result, err)
	}
	_, err = db.ExecContext(ctx, `
		INSERT INTO service_instances(team_id,challenge_id,runtime_kind,runtime_status,container_name,baseline_image,checker_image,endpoint,ssh_host,checker_token)
		VALUES(103,1,'docker','ready','reset-test-103','test','checker','10.80.1.13:8080','10.80.1.13','after-finish');
		INSERT INTO team_service_states(team_id,challenge_id,endpoint,status,checker,unlocked,ssh_hint,last_event,reset_cooldown)
		VALUES(103,1,'10.80.1.13:8080','stable','passing',FALSE,'','','ready');
		INSERT INTO issued_flags(flag,owner_team_id,owner_team_name,challenge_id,challenge_name,issued_tick,expires_tick,checker_token_snapshot)
		VALUES('finished-match-flag',103,'Team-103',1,'reset-test',1,1,'after-finish');
	`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = store.PrepareFactoryResetService(ctx, 103, 1); err != nil {
		t.Fatal(err)
	}
	result, err = store.CompleteFactoryResetService(ctx, 103, 1)
	if err != nil || result.PenaltyPoints != 0 {
		t.Fatalf("post-match reset charged a completed match: %+v %v", result, err)
	}
}

func TestPostgresParticipantResetWaitsForInFlightCheckerFlag(t *testing.T) {
	dsn := os.Getenv("TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("TEST_POSTGRES_DSN is not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	base, err := database.OpenPostgres(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	schema := fmt.Sprintf("reset_race_test_%d", time.Now().UnixNano())
	if _, err := base.ExecContext(ctx, "CREATE SCHEMA "+schema); err != nil {
		base.Close()
		t.Fatal(err)
	}
	parsed, err := url.Parse(dsn)
	if err != nil {
		t.Fatal(err)
	}
	query := parsed.Query()
	query.Set("search_path", schema)
	parsed.RawQuery = query.Encode()
	db, err := database.OpenPostgres(ctx, parsed.String())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		db.Close()
		base.ExecContext(context.Background(), "DROP SCHEMA "+schema+" CASCADE")
		base.Close()
	})
	if err := database.RunEmbeddedMigrationsWithOptions(ctx, db, database.MigrationOptions{IncludeSeeds: false}); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `
		INSERT INTO teams(id,name,email,join_key) VALUES(101,'Team-101','101@test.local','key-101'),(102,'Team-102','102@test.local','key-102');
		INSERT INTO challenges(id,name,published) VALUES(1,'reset-race',TRUE);
		INSERT INTO game_match_state(singleton,state) VALUES(TRUE,'running');
		INSERT INTO game_ticks(id,status) VALUES(1,'running');
		INSERT INTO service_instances(team_id,challenge_id,runtime_kind,runtime_status,container_name,baseline_image,checker_image,endpoint,ssh_host,checker_token)
		VALUES(101,1,'docker','ready','reset-race','test','checker','10.80.1.11:8080','10.80.1.11','before-reset');
		INSERT INTO team_service_states(team_id,challenge_id,endpoint,status,checker,unlocked,ssh_hint,last_event,reset_cooldown)
		VALUES(101,1,'10.80.1.11:8080','stable','passing',FALSE,'','','ready');
	`); err != nil {
		t.Fatal(err)
	}
	checkerTx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer checkerTx.Rollback()
	if _, err := checkerTx.ExecContext(ctx, `SELECT pg_advisory_xact_lock($1::integer,$2::integer)`, 101, 1); err != nil {
		t.Fatal(err)
	}
	if _, err := checkerTx.ExecContext(ctx, `
		INSERT INTO issued_flags(flag,owner_team_id,owner_team_name,challenge_id,challenge_name,issued_tick,expires_tick,checker_token_snapshot)
		VALUES('in-flight-flag',101,'Team-101',1,'reset-race',1,1,'before-reset')
	`); err != nil {
		t.Fatal(err)
	}
	resultCh := make(chan struct {
		data resetData
		err  error
	}, 1)
	go func() {
		data, err := (&postgresStore{db: db}).PrepareFactoryResetService(ctx, 101, 1)
		resultCh <- struct {
			data resetData
			err  error
		}{data, err}
	}()
	select {
	case result := <-resultCh:
		t.Fatalf("reset passed in-flight checker lock: %+v %v", result.data, result.err)
	case <-time.After(100 * time.Millisecond):
	}
	if err := checkerTx.Commit(); err != nil {
		t.Fatal(err)
	}
	select {
	case result := <-resultCh:
		if result.err != nil || result.data.PenaltyPoints != 10 {
			t.Fatalf("reset missed in-flight flag: %+v %v", result.data, result.err)
		}
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
}
