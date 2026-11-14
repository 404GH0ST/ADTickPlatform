package main

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"adplatform/internal/platform/database"
	"adplatform/internal/platform/database/dbtest"
	"adplatform/internal/services/apigateway"
)

// This opt-in test runs in its own schema, so it can share a database.
func TestPostgresFirstBloodConcurrentAccepts(t *testing.T) {
	db := dbtest.IsolatedSchema(t, "FIRST_BLOOD_TEST_POSTGRES_DSN", "first_blood_test")
	ctx := context.Background()
	if err := database.RunEmbeddedMigrationsWithOptions(ctx, db, database.MigrationOptions{IncludeSeeds: false}); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `
 INSERT INTO challenges (id, name) VALUES (900, 'first-blood-concurrency');
 INSERT INTO game_match_state (singleton, state) VALUES (TRUE, 'running');
 INSERT INTO game_ticks (id, status) VALUES (1, 'running');
 `); err != nil {
		t.Fatal(err)
	}
	for i := 900; i < 908; i++ {
		if _, err := db.ExecContext(ctx, `INSERT INTO teams (id, name, email, join_key) VALUES ($1, $2, $3, $2)`, i, fmt.Sprintf("team-%d", i), fmt.Sprintf("%d@test.local", i)); err != nil {
			t.Fatal(err)
		}
	}
	store := &postgresGameStore{db: db}
	var wg sync.WaitGroup
	errors := make(chan error, 8)
	for i := 900; i < 908; i++ {
		wg.Add(1)
		go func(team int) {
			defer wg.Done()
			accepted, err := store.AcceptFlagSubmission(ctx, acceptedFlagSubmission{Flag: "shared-flag", SubmittingTeam: team, AttackerName: fmt.Sprintf("team-%d", team), VictimName: "victim", ChallengeID: 900, ChallengeName: "first-blood-concurrency", ExpiresTick: 1, SubmittedAt: time.Now().UTC()})
			if err != nil {
				errors <- err
			} else if !accepted {
				errors <- fmt.Errorf("team %d was not accepted", team)
			}
		}(i)
	}
	wg.Wait()
	close(errors)
	for err := range errors {
		t.Error(err)
	}
	page, err := store.ListAttackFeed(ctx, apigateway.AttackFeedQuery{Limit: 20, Service: "first-blood-concurrency"})
	if err != nil {
		t.Fatal(err)
	}
	winners := 0
	for _, event := range page.Items {
		if event.FirstBlood {
			winners++
		}
	}
	if len(page.Items) != 8 || winners != 1 {
		t.Fatalf("expected eight attacks and one winner: %+v", page)
	}
}
