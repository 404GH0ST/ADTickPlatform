package apigateway

import (
	"context"
	"errors"
	"os"
	"sync"
	"testing"
	"time"

	"adplatform/internal/platform/database"
)

func TestPostgresSecurityAndAllocationInvariants(t *testing.T) {
	dsn := os.Getenv("TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("TEST_POSTGRES_DSN is not set")
	}
	t.Setenv("WIREGUARD_SERVER_PUBLIC_KEY", "integration-test-server-public-key")

	ctx := context.Background()
	db, err := database.OpenPostgres(ctx, dsn)
	if err != nil {
		t.Fatalf("open postgres: %v", err)
	}
	defer db.Close()
	if err := database.RunEmbeddedMigrationsWithOptions(ctx, db, database.MigrationOptions{IncludeSeeds: false}); err != nil {
		t.Fatalf("migrate postgres: %v", err)
	}
	store := NewPostgresStore(db)

	firstTeam, err := store.CreateAdminTeam(ctx, adminCreateTeamRequest{Name: "Integration Temporary", ContactEmail: "integration-temporary@example.com"})
	if err != nil {
		t.Fatalf("create first team: %v", err)
	}
	if err := store.DeleteAdminTeam(ctx, firstTeam.ID); err != nil {
		t.Fatalf("delete first team: %v", err)
	}
	team, err := store.CreateAdminTeam(ctx, adminCreateTeamRequest{Name: "Integration Replacement", ContactEmail: "integration-replacement@example.com"})
	if err != nil {
		t.Fatalf("create replacement team: %v", err)
	}
	if team.ID != firstTeam.ID {
		t.Fatalf("expected reusable team network id %d, got %d", firstTeam.ID, team.ID)
	}

	if _, err := db.ExecContext(ctx, `SELECT setval('players_id_seq', $1, TRUE)`, wireGuardPeerAddressPoolSize); err != nil {
		t.Fatalf("advance player sequence: %v", err)
	}
	player, err := store.CreateAdminPlayer(ctx, adminCreatePlayerRequest{
		TeamID:      team.ID,
		DisplayName: "Integration Player",
		Email:       "integration-player@example.com",
		Password:    "integration-original-password",
		Role:        "captain",
	}, time.Now())
	if err != nil {
		t.Fatalf("create player above WireGuard pool-size id: %v", err)
	}
	if player.ID != wireGuardPeerAddressPoolSize+1 || player.WireGuardAddress == "" {
		t.Fatalf("expected independent player id and WireGuard address, got %+v", player)
	}

	passwords := []string{"integration-first-password", "integration-second-password"}
	type changeResult struct {
		password string
		err      error
	}
	results := make(chan changeResult, len(passwords))
	start := make(chan struct{})
	var ready sync.WaitGroup
	ready.Add(len(passwords))
	for _, password := range passwords {
		go func() {
			ready.Done()
			<-start
			_, err := store.ChangeParticipantPassword(ctx, player.ID, "integration-original-password", password)
			results <- changeResult{password: password, err: err}
		}()
	}
	ready.Wait()
	close(start)

	winningPassword := ""
	invalidCount := 0
	for range passwords {
		result := <-results
		switch {
		case result.err == nil:
			winningPassword = result.password
		case errors.Is(result.err, ErrInvalidCredentials):
			invalidCount++
		default:
			t.Fatalf("unexpected concurrent password error: %v", result.err)
		}
	}
	if winningPassword == "" || invalidCount != 1 {
		t.Fatalf("expected one password update and one stale rejection, winner=%q rejected=%d", winningPassword, invalidCount)
	}

	if _, err := store.SetPlayerActive(ctx, player.ID, false, time.Now()); err != nil {
		t.Fatalf("deactivate player: %v", err)
	}
	if _, err := store.AuthenticatePlayer(ctx, player.Email, "wrong-password"); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("expected wrong password to conceal deactivated state, got %v", err)
	}
	if _, err := store.AuthenticatePlayer(ctx, player.Email, winningPassword); !errors.Is(err, ErrAccountDeactivated) {
		t.Fatalf("expected correct password to reveal deactivated state, got %v", err)
	}
}
