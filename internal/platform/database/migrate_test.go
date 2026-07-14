package database

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
)

func TestSecurityMigrationRotatesJoinKeysAndScrubsSettingsActor(t *testing.T) {
	script, err := migrationsFS.ReadFile("migrations/0039_rotate_join_keys_and_scrub_settings_actor.sql")
	if err != nil {
		t.Fatalf("read security migration: %v", err)
	}
	sql := string(script)
	for _, required := range []string{
		"UPDATE teams",
		"gen_random_uuid()",
		"UPDATE platform_settings",
		"updated_by = 'organizer'",
	} {
		if !strings.Contains(sql, required) {
			t.Fatalf("security migration is missing %q", required)
		}
	}
	if strings.Contains(sql, "UPPER(REPLACE(name") {
		t.Fatal("security migration must not derive join keys from team names")
	}
}

func TestSecurityMigrationRotatesJoinKeysAndScrubsSettingsActorAgainstPostgres(t *testing.T) {
	dsn := os.Getenv("TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("TEST_POSTGRES_DSN is not set")
	}

	ctx := context.Background()
	db, err := OpenPostgres(ctx, dsn)
	if err != nil {
		t.Fatalf("open postgres: %v", err)
	}
	defer db.Close()

	if err := RunEmbeddedMigrationsWithOptions(ctx, db, MigrationOptions{IncludeSeeds: false}); err != nil {
		t.Fatalf("migrate postgres: %v", err)
	}

	script, err := migrationsFS.ReadFile("migrations/0039_rotate_join_keys_and_scrub_settings_actor.sql")
	if err != nil {
		t.Fatalf("read security migration: %v", err)
	}
	migrationSQL := string(script)

	const (
		teamID         = 910039
		teamName       = "Public Name"
		predictableKey = "TEAM-PUBLIC-NAME-910039"
		leakedToken    = "super-secret-admin-token"
	)

	t.Run("rotates_predictable_join_keys_and_scrubs_token_actor", func(t *testing.T) {
		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			t.Fatalf("begin tx: %v", err)
		}
		defer tx.Rollback()

		if _, err := tx.ExecContext(ctx, `DELETE FROM teams WHERE id = $1`, teamID); err != nil {
			t.Fatalf("cleanup team: %v", err)
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO teams (id, name, email, join_key)
			VALUES ($1, $2, $3, $4)
		`, teamID, teamName, fmt.Sprintf("migrate-security-%d@example.com", teamID), predictableKey); err != nil {
			t.Fatalf("insert vulnerable team: %v", err)
		}
		if _, err := tx.ExecContext(ctx, `
			UPDATE platform_settings
			SET updated_by = $1
			WHERE id = 1
		`, leakedToken); err != nil {
			t.Fatalf("plant leaked settings actor: %v", err)
		}

		if _, err := tx.ExecContext(ctx, migrationSQL); err != nil {
			t.Fatalf("apply security migration: %v", err)
		}

		var joinKey string
		if err := tx.QueryRowContext(ctx, `SELECT join_key FROM teams WHERE id = $1`, teamID).Scan(&joinKey); err != nil {
			t.Fatalf("load rotated join key: %v", err)
		}
		if joinKey == predictableKey {
			t.Fatalf("join key was not rotated, still %q", joinKey)
		}
		if strings.Contains(strings.ToUpper(joinKey), "PUBLIC") || strings.Contains(joinKey, "910039") {
			t.Fatalf("rotated join key still derived from public team data: %q", joinKey)
		}
		if !strings.HasPrefix(joinKey, "TEAM-") {
			t.Fatalf("rotated join key has unexpected prefix: %q", joinKey)
		}

		var updatedBy string
		if err := tx.QueryRowContext(ctx, `SELECT updated_by FROM platform_settings WHERE id = 1`).Scan(&updatedBy); err != nil {
			t.Fatalf("load scrubbed settings actor: %v", err)
		}
		if updatedBy != "organizer" {
			t.Fatalf("expected leaked admin token to be scrubbed to organizer, got %q", updatedBy)
		}
		if strings.Contains(updatedBy, leakedToken) {
			t.Fatalf("settings actor still contains leaked token: %q", updatedBy)
		}
	})

	t.Run("preserves_system_settings_actor", func(t *testing.T) {
		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			t.Fatalf("begin tx: %v", err)
		}
		defer tx.Rollback()

		if _, err := tx.ExecContext(ctx, `
			UPDATE platform_settings
			SET updated_by = 'system'
			WHERE id = 1
		`); err != nil {
			t.Fatalf("plant system settings actor: %v", err)
		}
		if _, err := tx.ExecContext(ctx, migrationSQL); err != nil {
			t.Fatalf("apply security migration: %v", err)
		}

		var updatedBy string
		if err := tx.QueryRowContext(ctx, `SELECT updated_by FROM platform_settings WHERE id = 1`).Scan(&updatedBy); err != nil {
			t.Fatalf("load settings actor: %v", err)
		}
		if updatedBy != "system" {
			t.Fatalf("expected system settings actor to be preserved, got %q", updatedBy)
		}
	})
}
