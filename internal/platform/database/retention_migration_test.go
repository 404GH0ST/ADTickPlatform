package database

import (
	"os"
	"testing"
)

func TestRetentionMigrationRepairsOnlyUnconfirmedLossesAfterPassingTicks(t *testing.T) {
	dsn := os.Getenv("TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("TEST_POSTGRES_DSN is not set")
	}
	ctx := t.Context()
	db, err := OpenPostgres(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := RunEmbeddedMigrationsWithOptions(ctx, db, MigrationOptions{}); err != nil {
		t.Fatal(err)
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO teams(id,name,email,join_key)
		VALUES (910050,'retention fixture','retention-fixture@example.test','retention-fixture');
		INSERT INTO challenges(id,name) VALUES (910050,'retention fixture');
		INSERT INTO game_ticks(id,status)
		SELECT id,'completed' FROM generate_series(910050,910052) id;

		INSERT INTO issued_flags(flag,owner_team_id,owner_team_name,challenge_id,challenge_name,issued_tick,expires_tick)
		SELECT 'retention-fixture-'||id,910050,'fixture',910050,'fixture',id,id
		FROM generate_series(910050,910052) id;
		INSERT INTO flag_retention_failures(tick_id,team_id,challenge_id,flag,confirmed_missing,penalty_points,checked_at)
		SELECT id,910050,910050,'retention-fixture-'||id,id=910052,CASE WHEN id=910052 THEN 10 ELSE 0 END,NOW()
		FROM generate_series(910050,910052) id;
		INSERT INTO checker_service_states(tick_id,team_id,challenge_id,service_state,state_phase,state_message)
		SELECT id,910050,910050,CASE WHEN id=910052 THEN 'flag_not_found' ELSE 'down' END,'get','stored flag failed retention verification'
		FROM generate_series(910050,910052) id;
		INSERT INTO checker_runs(tick_id,team_id,team_name,challenge_id,challenge_name,phase,target,checker_image,status,exit_code,message)
		SELECT id,910050,'fixture',910050,'fixture',phase,'fixture','fixture',
		       CASE WHEN id=910051 AND phase='get' THEN 'failed' ELSE 'success' END,0,'ordinary phase'
		FROM generate_series(910050,910052) id CROSS JOIN unnest(ARRAY['put','get','check']) phase;
		INSERT INTO checker_runs(tick_id,team_id,team_name,challenge_id,challenge_name,phase,target,checker_image,status,exit_code,message)
		SELECT id,910050,'fixture',910050,'fixture','get','fixture','fixture','failed',-1,'stored flag failed retention verification'
		FROM generate_series(910050,910052) id;
	`); err != nil {
		t.Fatal(err)
	}
	migration, err := migrationsFS.ReadFile("migrations/0050_separate_retention_verdicts.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tx.ExecContext(ctx, string(migration)); err != nil {
		t.Fatal(err)
	}
	var verdicts string
	if err := tx.QueryRowContext(ctx, `
		SELECT string_agg(service_state, ',' ORDER BY tick_id) FROM checker_service_states WHERE team_id=910050
	`).Scan(&verdicts); err != nil || verdicts != "ok,down,flag_not_found" {
		t.Fatalf("unexpected repaired verdicts: %q %v", verdicts, err)
	}
	var observations, losses int
	if err := tx.QueryRowContext(ctx, `
		SELECT (SELECT COUNT(*) FROM checker_runs WHERE team_id=910050 AND retention_check),
		       (SELECT COUNT(*) FROM flag_retention_failures WHERE team_id=910050)
	`).Scan(&observations, &losses); err != nil || observations != 3 || losses != 3 {
		t.Fatalf("history was not preserved: observations=%d losses=%d error=%v", observations, losses, err)
	}
}
