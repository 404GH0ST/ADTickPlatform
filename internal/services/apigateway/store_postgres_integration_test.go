package apigateway

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"adplatform/internal/platform/database"
	"adplatform/internal/platform/database/dbtest"
)

func TestPostgresSecurityAndAllocationInvariants(t *testing.T) {
	db := dbtest.IsolatedSchema(t, "TEST_POSTGRES_DSN", "integration_test")
	t.Setenv("WIREGUARD_SERVER_PUBLIC_KEY", "integration-test-server-public-key")

	ctx := context.Background()
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

func TestPostgresLoginUpgradesLegacyPassword(t *testing.T) {
	db := dbtest.IsolatedSchema(t, "TEST_POSTGRES_DSN", "password_upgrade_test")
	ctx := context.Background()
	if err := database.RunEmbeddedMigrationsWithOptions(ctx, db, database.MigrationOptions{IncludeSeeds: false}); err != nil {
		t.Fatalf("migrate postgres: %v", err)
	}
	store := NewPostgresStore(db)
	const password = "legacy-test-password"
	player, err := store.RegisterPlayer(ctx, participantRegisterRequest{
		DisplayName: "Legacy Player",
		Email:       "legacy-player@example.com",
		Password:    password,
	}, time.Now())
	if err != nil {
		t.Fatalf("register player: %v", err)
	}
	if _, err := db.ExecContext(ctx, `UPDATE players SET password_hash = $2 WHERE id = $1`, player.PlayerID, legacyMD5Secret(password)); err != nil {
		t.Fatalf("set legacy credential: %v", err)
	}
	loggedIn, err := store.AuthenticatePlayer(ctx, player.Email, password)
	if err != nil || loggedIn.PlayerID != player.PlayerID || loggedIn.SessionVersion != player.SessionVersion {
		t.Fatalf("login returned %+v, %v", loggedIn, err)
	}
	var hash string
	if err := db.QueryRowContext(ctx, `SELECT password_hash FROM players WHERE id = $1`, player.PlayerID).Scan(&hash); err != nil {
		t.Fatalf("read current credential: %v", err)
	}
	if passwordHashNeedsUpgrade(hash) || !passwordMatches(hash, password) {
		t.Fatal("expected a modern hash for the same password")
	}
}

func TestPostgresTeamMembershipLifecycle(t *testing.T) {
	db := dbtest.IsolatedSchema(t, "TEST_POSTGRES_DSN", "membership_test")
	t.Setenv("WIREGUARD_SERVER_PUBLIC_KEY", "integration-test-server-public-key")
	ctx := context.Background()
	if err := database.RunEmbeddedMigrationsWithOptions(ctx, db, database.MigrationOptions{IncludeSeeds: false}); err != nil {
		t.Fatalf("migrate postgres: %v", err)
	}
	store := NewPostgresStore(db)
	now := time.Now()
	team, err := store.CreateAdminTeam(ctx, adminCreateTeamRequest{Name: "Test Team", ContactEmail: "team@example.test"})
	if err != nil {
		t.Fatal(err)
	}
	join := func(name, email, role string) authenticatedPlayer {
		t.Helper()
		player, err := store.RegisterPlayer(ctx, participantRegisterRequest{DisplayName: name, Email: email, Password: "test-participant-password"}, now)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := store.ValidatePlayerSession(ctx, player.PlayerID, 0, "member", player.SessionVersion); err != nil {
			t.Fatalf("validate teamless session: %v", err)
		}
		joined, err := store.JoinExistingPlayerTeam(ctx, player.PlayerID, team.JoinKey, now)
		if err != nil || joined.TeamID != team.ID || joined.Role != role {
			t.Fatalf("joined player = %+v, %v; want team %d, role %q", joined, err, team.ID, role)
		}
		return joined
	}
	captain := join("First Player", "first@example.test", "captain")
	member := join("Second Player", "second@example.test", "member")
	if _, err := store.RegisterPlayer(ctx, participantRegisterRequest{
		DisplayName: "Duplicate Email", Email: " SECOND@EXAMPLE.TEST ", Password: "test-participant-password",
	}, now); !errors.Is(err, ErrDuplicateResource) {
		t.Fatalf("duplicate registration returned %v", err)
	}
	if _, err := store.UpdateParticipantProfile(ctx, captain.PlayerID, participantUpdateProfileRequest{
		DisplayName: "First Player", Email: captain.Email, TeamName: "Renamed Team", TeamContactEmail: "new@example.test",
	}); err != nil {
		t.Fatalf("update captain profile: %v", err)
	}
	if _, err := store.UpdateParticipantProfile(ctx, member.PlayerID, participantUpdateProfileRequest{
		DisplayName: "Renamed Player", Email: member.Email,
	}); err != nil {
		t.Fatalf("update member profile: %v", err)
	}
	previousCaptain, err := store.TransferTeamCaptain(ctx, captain.PlayerID, member.PlayerID)
	if err != nil || previousCaptain.Role != "member" || previousCaptain.TeamName != "Renamed Team" {
		t.Fatalf("transferred captain = %+v, %v", previousCaptain, err)
	}
	newCaptain, err := store.ValidatePlayerSession(ctx, member.PlayerID, team.ID, "captain", member.SessionVersion+1)
	if err != nil || newCaptain.TeamContactEmail != "new@example.test" || newCaptain.DisplayName != "Renamed Player" {
		t.Fatalf("validate captain session = %+v, %v", newCaptain, err)
	}
	members, err := store.ListTeamMembers(ctx, team.ID)
	if err != nil || len(members) != 2 || members[0].Role != "member" || members[1].Role != "captain" {
		t.Fatalf("list team members = %+v, %v", members, err)
	}
}
