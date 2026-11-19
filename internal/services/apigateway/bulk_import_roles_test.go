package apigateway

import (
	"context"
	"strings"
	"testing"
	"time"

	"adplatform/internal/platform/database"
	"adplatform/internal/platform/database/dbtest"
)

// Team rosters fed to the bulk importer are routinely assembled from
// participant-supplied registration data. A roster row must never be able to
// mint an organizer account (which the web tier maps to the full admin token)
// or stack a second captain onto the team it creates.
type bulkImportStore interface {
	ImportAdminTeams(ctx context.Context, teams []adminBulkImportTeam, now time.Time) (adminBulkImportResult, error)
	AuthenticatePlayer(ctx context.Context, email, password string) (authenticatedPlayer, error)
}

func assertBulkImportRejectsPrivilegedRoles(t *testing.T, store bulkImportStore) {
	t.Helper()
	ctx := context.Background()
	now := time.Now()

	for _, role := range []string{"organizer", "admin", " Admin ", "ORGANIZER"} {
		email := "roster-" + strings.ToLower(strings.TrimSpace(role)) + "-" + time.Now().Format("150405.000000000") + "@example.test"
		result, err := store.ImportAdminTeams(ctx, []adminBulkImportTeam{{
			Name:         "Roster Team " + email,
			ContactEmail: "contact-" + email,
			Players: []adminBulkImportPlayer{{
				DisplayName: "Roster Player",
				Email:       email,
				Password:    "roster-password",
				Role:        role,
			}},
		}}, now)
		if err != nil {
			t.Fatalf("import with role %q: %v", role, err)
		}
		if result.TeamsCreated != 0 || result.PlayersCreated != 0 || len(result.Errors) != 1 {
			t.Fatalf("import with role %q must reject the team row, got %+v", role, result)
		}
		if !strings.Contains(result.Errors[0], "role") {
			t.Fatalf("import error for role %q should name the role problem, got %q", role, result.Errors[0])
		}
		if player, err := store.AuthenticatePlayer(ctx, email, "roster-password"); err == nil {
			t.Fatalf("role %q roster row created a usable %q account", role, player.Role)
		}
	}

	duplicateCaptains, err := store.ImportAdminTeams(ctx, []adminBulkImportTeam{{
		Name:         "Two Captains Team",
		ContactEmail: "two-captains@example.test",
		Players: []adminBulkImportPlayer{
			{DisplayName: "Captain One", Email: "captain-one@example.test", Password: "roster-password", Role: "captain"},
			{DisplayName: "Captain Two", Email: "captain-two@example.test", Password: "roster-password", Role: "captain"},
		},
	}}, now)
	if err != nil {
		t.Fatalf("import two captains: %v", err)
	}
	if duplicateCaptains.TeamsCreated != 0 || len(duplicateCaptains.Errors) != 1 || !strings.Contains(duplicateCaptains.Errors[0], "captain") {
		t.Fatalf("a roster with two captains must be rejected, got %+v", duplicateCaptains)
	}

	unknown, err := store.ImportAdminTeams(ctx, []adminBulkImportTeam{{
		Name:         "Unknown Role Team",
		ContactEmail: "unknown-role@example.test",
		Players: []adminBulkImportPlayer{
			{DisplayName: "Unknown Role", Email: "unknown-role-player@example.test", Password: "roster-password", Role: "superuser"},
		},
	}}, now)
	if err != nil {
		t.Fatalf("import unknown role: %v", err)
	}
	if unknown.TeamsCreated != 0 || len(unknown.Errors) != 1 {
		t.Fatalf("an unknown roster role must be rejected rather than silently mapped, got %+v", unknown)
	}

	allowed, err := store.ImportAdminTeams(ctx, []adminBulkImportTeam{{
		Name:         "Valid Roster Team",
		ContactEmail: "valid-roster@example.test",
		Players: []adminBulkImportPlayer{
			{DisplayName: "Valid Captain", Email: "valid-captain@example.test", Password: "roster-password", Role: " Captain "},
			{DisplayName: "Valid Member", Email: "valid-member@example.test", Password: "roster-password", Role: "member"},
			{DisplayName: "Default Member", Email: "default-member@example.test", Password: "roster-password"},
		},
	}}, now)
	if err != nil {
		t.Fatalf("import valid roster: %v", err)
	}
	if allowed.TeamsCreated != 1 || allowed.PlayersCreated != 3 || len(allowed.Errors) != 0 {
		t.Fatalf("valid roster should import, got %+v", allowed)
	}
	roles := map[string]string{}
	for _, player := range allowed.Players {
		roles[player.Email] = player.Role
	}
	if roles["valid-captain@example.test"] != "captain" || roles["valid-member@example.test"] != "member" || roles["default-member@example.test"] != "member" {
		t.Fatalf("unexpected imported roles: %#v", roles)
	}
}

func TestMemoryBulkImportRejectsPrivilegedRoles(t *testing.T) {
	assertBulkImportRejectsPrivilegedRoles(t, NewMemoryStore(101).(*memoryStore))
}

func TestPostgresBulkImportRejectsPrivilegedRoles(t *testing.T) {
	db := dbtest.IsolatedSchema(t, "TEST_POSTGRES_DSN", "bulk_import_roles_test")
	t.Setenv("WIREGUARD_SERVER_PUBLIC_KEY", "integration-test-server-public-key")
	if err := database.RunEmbeddedMigrationsWithOptions(context.Background(), db, database.MigrationOptions{IncludeSeeds: false}); err != nil {
		t.Fatalf("migrate postgres: %v", err)
	}
	assertBulkImportRejectsPrivilegedRoles(t, NewPostgresStore(db).(*postgresStore))
}
