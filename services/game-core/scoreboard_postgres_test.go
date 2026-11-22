package main

import (
	"context"
	"errors"
	"testing"
	"time"

	"adplatform/internal/platform/database"
)

// A flag stolen before the owner was disqualified must stop scoring at once,
// and must score again after the owner is reinstated.
func TestPostgresDisqualifiedOwnerFlagsNoLongerScore(t *testing.T) {
	db := isolatedSubmissionDB(t)
	ctx := context.Background()
	if err := database.RunEmbeddedMigrationsWithOptions(ctx, db, database.MigrationOptions{IncludeSeeds: false}); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `
		INSERT INTO teams(id,name,email,join_key) VALUES(101,'attacker','a@test.local','a'),(102,'victim','v@test.local','v');
		INSERT INTO challenges(id,name) VALUES(1,'one');
		INSERT INTO game_match_state(singleton,state) VALUES(TRUE,'running');
		INSERT INTO game_ticks(id,status) VALUES(1,'running');
		INSERT INTO team_service_states(team_id,challenge_id,endpoint,status,checker,ssh_hint,last_event,reset_cooldown)
		SELECT t.id,c.id,'test','healthy','ok','locked','','' FROM teams t CROSS JOIN challenges c;
		UPDATE teams SET active = FALSE WHERE id = 102;
	`); err != nil {
		t.Fatal(err)
	}
	store := &postgresGameStore{db: db}
	now := time.Now().UTC()
	if err := store.IssueFlag(ctx, issuedFlagRecord{Flag: "enemy", OwnerTeamID: 102, ChallengeID: 1, IssuedTick: 1, ExpiresTick: 2, CreatedAt: now}); err != nil {
		t.Fatal(err)
	}
	capture := acceptedFlagSubmission{Flag: "enemy", SubmittingTeam: 101, OwnerTeamID: 102, ChallengeID: 1, ExpiresTick: 2, SubmittedAt: now}
	if ok, err := store.AcceptFlagSubmission(ctx, capture); ok || !errors.Is(err, errFlagNoLongerValid) {
		t.Fatalf("flag of a disqualified owner: %v %v", ok, err)
	}
	var captures int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM submitted_flags`).Scan(&captures); err != nil || captures != 0 {
		t.Fatalf("rejected capture persisted: %d %v", captures, err)
	}
	if _, err := db.ExecContext(ctx, `UPDATE teams SET active = TRUE WHERE id = 102`); err != nil {
		t.Fatal(err)
	}
	if ok, err := store.AcceptFlagSubmission(ctx, capture); err != nil || !ok {
		t.Fatalf("flag after reinstatement: %v %v", ok, err)
	}
}

// Disqualifying a team must not change the other teams' accumulated SLA.
func TestPostgresDisqualificationKeepsOtherTeamsSLA(t *testing.T) {
	db := isolatedSubmissionDB(t)
	ctx := context.Background()
	if err := database.RunEmbeddedMigrationsWithOptions(ctx, db, database.MigrationOptions{IncludeSeeds: false}); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `
		INSERT INTO teams(id,name,email,join_key) VALUES(101,'Alpha','a@t','a'),(102,'Bravo','b@t','b'),(103,'Charlie','c@t','c'),(104,'Delta','d@t','d');
		INSERT INTO challenges(id,name) VALUES(1,'one');
		INSERT INTO game_ticks(id,status) VALUES(1,'completed');
		INSERT INTO checker_service_states(tick_id,team_id,challenge_id,service_state) VALUES(1,101,1,'ok');
	`); err != nil {
		t.Fatal(err)
	}
	store := &postgresGameStore{db: db}
	slaOf := func(team string) float64 {
		t.Helper()
		rows, err := store.ListScoreboard(ctx)
		if err != nil {
			t.Fatal(err)
		}
		for _, row := range rows {
			if row.Team == team {
				return row.SLA
			}
		}
		t.Fatalf("%s missing from board: %+v", team, rows)
		return 0
	}
	before := slaOf("Alpha")
	if before != 2 {
		t.Fatalf("SLA before disqualification = %v, want 2", before)
	}
	if _, err := db.ExecContext(ctx, `UPDATE teams SET active = FALSE WHERE id = 104`); err != nil {
		t.Fatal(err)
	}
	if after := slaOf("Alpha"); after != before {
		t.Fatalf("SLA changed from %v to %v", before, after)
	}
}

// A deactivation that commits while a recompute is running must still leave
// the team on the board, marked disqualified and ranked last.
func TestPostgresRecomputeRanksDisqualifiedTeamLast(t *testing.T) {
	db := isolatedSubmissionDB(t)
	ctx := context.Background()
	if err := database.RunEmbeddedMigrationsWithOptions(ctx, db, database.MigrationOptions{IncludeSeeds: false}); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `
		INSERT INTO teams(id,name,email,join_key,active) VALUES(101,'Alpha','a@t','a',TRUE),(102,'Bravo','b@t','b',TRUE),(103,'Charlie','c@t','c',FALSE);
		INSERT INTO scoreboard_entries(team_id,rank,team_name,attack_points,defense_points,sla_points,total_points,delta)
		VALUES(103,1,'Charlie',5,0,0,5,'same');
	`); err != nil {
		t.Fatal(err)
	}
	store := &postgresGameStore{db: db}
	if _, err := store.RecomputeScoreboard(ctx); err != nil {
		t.Fatal(err)
	}
	stored, err := store.listStoredScoreboard(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(stored) != 3 {
		t.Fatalf("every team must stay on the board: %+v", stored)
	}
	last := stored[len(stored)-1]
	if last.Team != "Charlie" || !last.Disqualified || last.Rank != 3 {
		t.Fatalf("disqualified team must rank last: %+v", stored)
	}
	for _, row := range stored[:2] {
		if row.Disqualified {
			t.Fatalf("active team marked disqualified: %+v", row)
		}
	}
	audit, err := store.AuditScoreboard(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if audit.MismatchCount != 0 {
		t.Fatalf("scoring audit reports mismatches: %+v", audit)
	}
}
