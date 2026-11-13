package main

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"sync"
	"testing"
	"time"

	"adplatform/internal/platform/database"
	"adplatform/internal/services/apigateway"
)

func isolatedSubmissionDB(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("SUBMISSION_TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("SUBMISSION_TEST_POSTGRES_DSN is not set")
	}
	ctx := context.Background()
	base, err := database.OpenPostgres(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	schema := fmt.Sprintf("submission_test_%d", time.Now().UnixNano())
	if _, err = base.ExecContext(ctx, "CREATE SCHEMA "+schema); err != nil {
		base.Close()
		t.Fatal(err)
	}
	parsed, err := url.Parse(dsn)
	if err != nil {
		t.Fatal(err)
	}
	params := parsed.Query()
	params.Set("search_path", schema)
	parsed.RawQuery = params.Encode()
	db, err := database.OpenPostgres(ctx, parsed.String())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close(); base.ExecContext(ctx, "DROP SCHEMA "+schema+" CASCADE"); base.Close() })
	return db
}

func TestPostgresSubmissionDeletionAndConcurrentNextWinner(t *testing.T) {
	db := isolatedSubmissionDB(t)
	ctx := context.Background()
	if err := database.RunEmbeddedMigrationsWithOptions(ctx, db, database.MigrationOptions{IncludeSeeds: false}); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO challenges(id,name) VALUES(1,'capture-test'); INSERT INTO game_match_state(singleton,state) VALUES(TRUE,'running'); INSERT INTO game_ticks(id,status) VALUES(1,'running');`); err != nil {
		t.Fatal(err)
	}
	teams := []int{101, 103, 900, 901, 902, 903, 904, 905, 906, 907, 908}
	for _, id := range teams {
		if _, err := db.ExecContext(ctx, `INSERT INTO teams(id,name,email,join_key) VALUES($1,$2,$3,$2)`, id, fmt.Sprintf("team-%d", id), fmt.Sprintf("%d@test.local", id)); err != nil {
			t.Fatal(err)
		}
	}
	store := &postgresGameStore{db: db}
	now := time.Now().UTC()
	if err := store.IssueFlag(ctx, issuedFlagRecord{Flag: "shared", OwnerTeamID: 900, OwnerTeamName: "owner", ChallengeID: 1, ChallengeName: "capture-test", IssuedTick: 1, ExpiresTick: 1, CreatedAt: now}); err != nil {
		t.Fatal(err)
	}
	accept := func(team int) (bool, error) {
		return store.AcceptFlagSubmission(ctx, acceptedFlagSubmission{Flag: "shared", SubmittingTeam: team, AttackerName: fmt.Sprintf("team-%d", team), VictimName: "owner", ChallengeID: 1, ChallengeName: "capture-test", ExpiresTick: 1, SubmittedAt: time.Now().UTC()})
	}
	for _, team := range []int{101, 103} {
		if ok, err := accept(team); !ok || err != nil {
			t.Fatalf("initial accept %v %v", ok, err)
		}
	}
	for _, status := range []string{"invalid", "duplicate"} {
		if err := store.RecordSubmission(ctx, apigateway.AdminSubmission{TeamID: 101, Attacker: "team-101", Flag: "shared", Status: status, Tick: 1, SubmittedAt: now}); err != nil {
			t.Fatal(err)
		}
	}
	page, err := store.ListSubmissions(ctx, apigateway.SubmissionQuery{AttackFeedQuery: apigateway.AttackFeedQuery{Limit: 100}})
	if err != nil {
		t.Fatal(err)
	}
	if page.TotalCount != 4 {
		t.Fatalf("missing attempts: %+v", page)
	}
	var first, duplicate int64
	for _, row := range page.Items {
		if row.FirstBlood {
			first = row.ID
		}
		if row.Status == "duplicate" {
			duplicate = row.ID
		}
	}
	if result, err := store.DeleteSubmission(ctx, duplicate); err != nil || result.FirstBloodCleared {
		t.Fatalf("duplicate deletion: %+v %v", result, err)
	}
	if accepted, err := accept(101); err != nil || accepted {
		t.Fatalf("duplicate deletion removed accepted capture: %v %v", accepted, err)
	}
	if result, err := store.DeleteSubmission(ctx, first); err != nil || !result.FirstBloodCleared {
		t.Fatalf("first blood deletion: %+v %v", result, err)
	}
	attacks, err := store.ListAttackFeed(ctx, apigateway.AttackFeedQuery{Limit: 100})
	if err != nil {
		t.Fatal(err)
	}
	if attacks.TotalCount != 1 || attacks.Items[0].FirstBlood || attacks.Revision != 1 {
		t.Fatalf("prior capture promoted or stale feed: %+v", attacks)
	}
	scores, err := store.ListScoreboard(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range scores {
		if row.Team == "team-101" && row.Attack != 0 {
			t.Fatalf("deleted attack still scored: %+v", row)
		}
		if row.Team == "team-103" && row.Attack != 2 {
			t.Fatalf("remaining capture not rescored: %+v", row)
		}
	}
	var wg sync.WaitGroup
	errs := make(chan error, 8)
	for team := 901; team <= 908; team++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			if ok, err := accept(id); err != nil {
				errs <- err
			} else if !ok {
				errs <- fmt.Errorf("team %d rejected", id)
			}
		}(team)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Error(err)
	}
	attacks, err = store.ListAttackFeed(ctx, apigateway.AttackFeedQuery{Limit: 100})
	if err != nil {
		t.Fatal(err)
	}
	winners := 0
	for _, row := range attacks.Items {
		if row.FirstBlood {
			winners++
		}
	}
	if winners != 1 || attacks.TotalCount != 9 {
		t.Fatalf("expected one new first blood: %+v", attacks)
	}
	if ok, err := accept(101); !ok || err != nil {
		t.Fatalf("removed flag cannot be reaccepted: %v %v", ok, err)
	}
	page, err = store.ListSubmissions(ctx, apigateway.SubmissionQuery{AttackFeedQuery: apigateway.AttackFeedQuery{Limit: 100}})
	if err != nil {
		t.Fatal(err)
	}
	var reacceptedID int64
	for _, row := range page.Items {
		if row.TeamID == 101 && row.Status == "accepted" {
			reacceptedID = row.ID
			if row.FirstBlood {
				t.Fatal("another first blood assigned")
			}
		}
	}
	if reacceptedID == 0 || reacceptedID == first {
		t.Fatal("capture identity was reused")
	}
	// A history write failure must roll back the capture and its attack event.
	if _, err := db.ExecContext(ctx, `CREATE FUNCTION reject_history() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF NEW.attacker='fail-history' THEN RAISE EXCEPTION 'injected history failure'; END IF; RETURN NEW; END $$; CREATE TRIGGER reject_history BEFORE INSERT ON submission_events FOR EACH ROW EXECUTE FUNCTION reject_history();`); err != nil {
		t.Fatal(err)
	}
	if ok, err := store.AcceptFlagSubmission(ctx, acceptedFlagSubmission{Flag: "rollback-flag", SubmittingTeam: 101, AttackerName: "fail-history", ChallengeID: 1, ChallengeName: "capture-test", ExpiresTick: 1, SubmittedAt: now}); err == nil || ok {
		t.Fatal("expected atomic capture failure")
	}
	var count int
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM submitted_flags WHERE flag='rollback-flag'`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("partial capture persisted: %d %v", count, err)
	}
}

func TestSubmissionMigrationBackfillsLegacyCaptureAndCanDeleteIt(t *testing.T) {
	db := isolatedSubmissionDB(t)
	ctx := context.Background()
	now := time.Now().UTC()
	if _, err := db.ExecContext(ctx, `CREATE TABLE teams(id integer,name text); CREATE TABLE submitted_flags(flag text,team_id integer,submitted_at timestamptz); CREATE TABLE issued_flags(flag text,owner_team_name text,challenge_id integer,challenge_name text,issued_tick integer); CREATE TABLE attack_events(id text,attacker text,victim text,challenge_id integer,service text,tick integer,created_at timestamptz,first_blood boolean); CREATE TABLE game_ticks(id integer);`); err != nil {
		t.Fatal(err)
	}
	sub := acceptedFlagSubmission{Flag: "legacy-flag", SubmittingTeam: 101, SubmissionTick: 7}
	if _, err := db.ExecContext(ctx, `INSERT INTO teams VALUES(101,'renamed'); INSERT INTO issued_flags VALUES('legacy-flag','victim',3,'legacy-service',6);`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO submitted_flags VALUES($1,101,$2)`, sub.Flag, now); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO attack_events VALUES($1,'original team','victim',3,'legacy-service',7,$2,TRUE)`, acceptedFlagAttackEventID(sub), now); err != nil {
		t.Fatal(err)
	}
	migration, err := os.ReadFile("../../internal/platform/database/migrations/0041_submission_history.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, string(migration)); err != nil {
		t.Fatal(err)
	}
	store := &postgresGameStore{db: db}
	page, err := store.ListSubmissions(ctx, apigateway.SubmissionQuery{})
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Items) != 1 || !page.Items[0].FirstBlood || page.Items[0].Flag != sub.Flag || page.Items[0].Tick != 7 || page.Items[0].Attacker != "original team" {
		t.Fatalf("incorrect historical import: %+v", page)
	}
	result, err := store.DeleteSubmission(ctx, page.Items[0].ID)
	if err != nil || !result.FirstBloodCleared {
		t.Fatalf("legacy deletion: %+v %v", result, err)
	}
	var count int
	db.QueryRowContext(ctx, `SELECT count(*) FROM submitted_flags`).Scan(&count)
	if count != 0 {
		t.Fatal("legacy scoring record retained")
	}
}
