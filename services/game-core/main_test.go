package main

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"adplatform/internal/platform/database"
	"adplatform/internal/platform/database/dbtest"
	"adplatform/internal/platform/servicetest"
)

func setGameCoreEnv(t *testing.T) {
	t.Helper()
	t.Setenv("GAME_CORE_ADDR", "127.0.0.1:0")
	t.Setenv("GAME_CORE_STATE_BACKEND", "memory")
	t.Setenv("ADMIN_API_TOKEN", "unit-test-admin-token-7f3a")
	t.Setenv("GAME_CORE_FLAG_SECRET", "unit-test-flag-secret-3e9b")
	t.Setenv("GAME_CORE_FLAG_FORMAT_PREFIX", "")
	t.Setenv("GAME_CORE_MAX_TEAM_MEMBERS", "")
	t.Setenv("GAME_CORE_MATCH_START_AT", "")
	t.Setenv("GAME_CORE_MATCH_END_AT", "")
}

func TestGameCoreRunServesUntilCancelled(t *testing.T) {
	setGameCoreEnv(t)
	t.Setenv("GAME_CORE_MATCH_START_AT", "2026-11-19T10:00:00Z")
	t.Setenv("GAME_CORE_MATCH_END_AT", "2026-11-19T18:00:00Z")
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	if err := run(ctx); err != nil {
		t.Fatalf("run: %v", err)
	}
}

func TestGameCoreRunRejectsBadMatchWindow(t *testing.T) {
	for name, env := range map[string][2]string{
		"bad start":        {"tomorrow", ""},
		"bad end":          {"", "later"},
		"end before start": {"2026-11-19T18:00:00Z", "2026-11-19T10:00:00Z"},
	} {
		setGameCoreEnv(t)
		t.Setenv("GAME_CORE_MATCH_START_AT", env[0])
		t.Setenv("GAME_CORE_MATCH_END_AT", env[1])
		if err := run(context.Background()); err == nil {
			t.Errorf("%s: an invalid match window must abort startup", name)
		}
	}
}

func TestGameCoreMainLifecycle(t *testing.T) {
	setGameCoreEnv(t)
	got := servicetest.CaptureFatal(t, &fatal)
	servicetest.RunMainUntilSIGTERM(t, main)
	if len(got()) != 0 {
		t.Fatalf("clean shutdown must not be fatal: %v", got())
	}
	t.Setenv("GAME_CORE_STATE_BACKEND", "postgres")
	t.Setenv("POSTGRES_DSN", "postgres://nobody@127.0.0.1:1/none?sslmode=disable&connect_timeout=1")
	main()
	if len(got()) == 0 {
		t.Fatal("an unreachable database must be fatal")
	}
}

func TestGameCoreRunRejectsInvalidMaxTeamMembers(t *testing.T) {
	setGameCoreEnv(t)
	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()
	for _, value := range []string{"-1", "many", "2147483648"} {
		t.Setenv("GAME_CORE_MAX_TEAM_MEMBERS", value)
		if err := run(ctx); err == nil || !strings.Contains(err.Error(), "GAME_CORE_MAX_TEAM_MEMBERS") {
			t.Fatalf("startup must reject a team member limit of %q: %v", value, err)
		}
	}
}

func TestGameCoreStartupAgainstPostgres(t *testing.T) {
	db := dbtest.IsolatedSchema(t, "SUBMISSION_TEST_POSTGRES_DSN", "gc_startup")
	if err := database.RunEmbeddedMigrationsWithOptions(context.Background(), db, database.MigrationOptions{}); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO game_ticks(id, status) VALUES (1, 'running')`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`UPDATE platform_settings SET max_team_members = 7 WHERE id = 1`); err != nil {
		t.Fatal(err)
	}
	dsn := dbtest.SchemaDSN(t, "SUBMISSION_TEST_POSTGRES_DSN", db)
	setGameCoreEnv(t)
	t.Setenv("GAME_CORE_STATE_BACKEND", "postgres")
	t.Setenv("POSTGRES_DSN", dsn)
	t.Setenv("GAME_CORE_FLAG_FORMAT_PREFIX", " PLAYITPLUS ")
	t.Setenv("GAME_CORE_MAX_TEAM_MEMBERS", " 3 ")
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	if err := run(ctx); err != nil {
		t.Fatalf("postgres run: %v", err)
	}
	var status string
	if err := db.QueryRow(`SELECT status FROM game_ticks WHERE id = 1`).Scan(&status); err != nil || status != "failed" {
		t.Fatalf("a tick left running by a crash must be reaped at startup: %q %v", status, err)
	}
	var stored, active string
	var teamLimit int
	if err := db.QueryRow(`SELECT flag_format_prefix, flag_format_active, max_team_members FROM platform_settings WHERE id = 1`).
		Scan(&stored, &active, &teamLimit); err != nil || stored != "PLAYITPLUS" || active != stored || teamLimit != 3 {
		t.Fatalf("startup must sync both prefixes and the team limit: %q %q %d %v", stored, active, teamLimit, err)
	}
	if _, err := db.Exec(`UPDATE platform_settings SET flag_format_prefix = 'PENDING' WHERE id = 1`); err != nil {
		t.Fatal(err)
	}
	store := newPostgresGameStore(db)
	if prefix, err := store.ApplyStartupPlatformSettings(t.Context(), "", nil); err != nil || prefix != "PLAYITPLUS" {
		t.Fatalf("startup without an override must use the active prefix: %q %v", prefix, err)
	}
	unlimited := 0
	if prefix, err := store.ApplyStartupPlatformSettings(t.Context(), "", &unlimited); err != nil || prefix != "PLAYITPLUS" {
		t.Fatalf("startup with a team limit override must use the active prefix: %q %v", prefix, err)
	}
	if err := db.QueryRow(`SELECT flag_format_prefix, max_team_members FROM platform_settings WHERE id = 1`).
		Scan(&stored, &teamLimit); err != nil || stored != "PENDING" || teamLimit != 0 {
		t.Fatalf("team limit override must preserve a pending prefix and accept zero: %q %d %v", stored, teamLimit, err)
	}
	if _, err := store.ApplyStartupPlatformSettings(t.Context(), "MYCTF", nil); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT max_team_members FROM platform_settings WHERE id = 1`).Scan(&teamLimit); err != nil || teamLimit != 0 {
		t.Fatalf("prefix override must preserve the team limit: %d %v", teamLimit, err)
	}

	t.Setenv("GAME_CORE_AUTO_MIGRATE", "false")
	t.Setenv("POSTGRES_DSN", dsn+"&default_transaction_read_only=on")
	if err := run(context.Background()); err == nil {
		t.Fatal("a failed startup reap must abort startup")
	}

	t.Setenv("POSTGRES_DSN", dsn)
	t.Setenv("GAME_CORE_STARTUP_RECOMPUTE_TIMEOUT", "1ns")
	if err := run(context.Background()); err == nil || !strings.Contains(err.Error(), "initial scoreboard recompute failed") {
		t.Fatalf("a failed startup recompute must abort startup: %v", err)
	}

	t.Setenv("GAME_CORE_AUTO_MIGRATE", "true")
	readOnly := dbtest.IsolatedSchema(t, "SUBMISSION_TEST_POSTGRES_DSN", "gc_startup_ro")
	t.Setenv("POSTGRES_DSN", dbtest.SchemaDSN(t, "SUBMISSION_TEST_POSTGRES_DSN", readOnly)+"&default_transaction_read_only=on")
	if _, err := buildStore(context.Background()); err == nil {
		t.Fatal("a migration failure must be returned")
	}
}

func TestStartupHelpers(t *testing.T) {
	if got := strings.Join(parseCheckerPhases(" PUT, ,get,bogus,put,check "), ","); got != "put,get,check" {
		t.Fatalf("phases: %s", got)
	}
	if got := strings.Join(parseCheckerPhases("bogus"), ","); got != "put,get,check" {
		t.Fatalf("no valid phases must fall back to all: %s", got)
	}
	if err := recomputeScoreboardWithTimeout(context.Background(), 0, func(context.Context) error { return nil }); err != nil {
		t.Fatal(err)
	}
	if err := recomputeScoreboardWithTimeout(context.Background(), time.Second, func(context.Context) error { return errors.New("x") }); err == nil {
		t.Fatal("recompute errors must surface")
	}
}
