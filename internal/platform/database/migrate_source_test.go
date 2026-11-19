package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"testing"
	"testing/fstest"
	"time"
)

// isolatedDB opens TEST_POSTGRES_DSN inside a throwaway schema. (dbtest cannot
// be imported here: it depends on this package.)
func isolatedDB(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("TEST_POSTGRES_DSN is not set")
	}
	ctx := context.Background()
	base, err := OpenPostgres(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	schema := fmt.Sprintf("migrate_source_%d", time.Now().UnixNano())
	if _, err := base.ExecContext(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatal(err)
	}
	parsed, _ := url.Parse(dsn)
	query := parsed.Query()
	query.Set("search_path", schema)
	parsed.RawQuery = query.Encode()
	db, err := OpenPostgres(ctx, parsed.String())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		db.Close()
		_, _ = base.ExecContext(context.Background(), "DROP SCHEMA "+schema+" CASCADE")
		base.Close()
	})
	return db
}

func withMigrationSource(t *testing.T, source fs.FS) {
	t.Helper()
	original := migrationSource
	migrationSource = source
	t.Cleanup(func() { migrationSource = original })
}

// unreadableFS lists one migration but fails to open it.
type unreadableFS struct{ fstest.MapFS }

func (u unreadableFS) Open(name string) (fs.File, error) {
	if name == "migrations/0001_x.sql" {
		return nil, errors.New("disk read failed")
	}
	return u.MapFS.Open(name)
}

func (u unreadableFS) ReadFile(name string) ([]byte, error) {
	if name == "migrations/0001_x.sql" {
		return nil, errors.New("disk read failed")
	}
	return u.MapFS.ReadFile(name)
}

func TestMigrationSourceFailures(t *testing.T) {
	db := isolatedDB(t)
	ctx := context.Background()

	withMigrationSource(t, fstest.MapFS{})
	if err := RunEmbeddedMigrationsWithOptions(ctx, db, MigrationOptions{}); err == nil {
		t.Fatal("a missing migrations directory must fail")
	}

	withMigrationSource(t, unreadableFS{fstest.MapFS{"migrations/0001_x.sql": {Data: []byte("SELECT 1")}}})
	if err := RunEmbeddedMigrationsWithOptions(ctx, db, MigrationOptions{}); err == nil {
		t.Fatal("an unreadable migration must fail")
	}

	// Subdirectories are skipped; seeds are skipped unless requested.
	withMigrationSource(t, fstest.MapFS{
		"migrations/0001_tables.sql":    {Data: []byte("CREATE TABLE source_probe (id INT)")},
		"migrations/0002_demo_seed.sql": {Data: []byte("INSERT INTO source_probe VALUES (1)")},
		"migrations/nested/ignored.sql": {Data: []byte("not sql")},
	})
	if err := RunEmbeddedMigrationsWithOptions(ctx, db, MigrationOptions{}); err != nil {
		t.Fatalf("migrate without seeds: %v", err)
	}
	if err := RunEmbeddedMigrations(ctx, db); err != nil {
		t.Fatalf("migrate with seeds: %v", err)
	}
	var rows int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM source_probe`).Scan(&rows); err != nil || rows != 1 {
		t.Fatalf("seed must apply exactly once, rows=%d err=%v", rows, err)
	}
}

func TestOpenPostgresRejectsBadInput(t *testing.T) {
	if _, err := OpenPostgres(context.Background(), "postgres://u@127.0.0.1/db?sslmode=bogus"); err == nil {
		t.Fatal("an unparseable DSN must fail")
	}
	if _, err := OpenPostgres(context.Background(), "postgres://nobody@127.0.0.1:1/none?connect_timeout=1&sslmode=disable"); err == nil {
		t.Fatal("an unreachable server must fail")
	}
	closed, err := sql.Open("pgx", "postgres://nobody@127.0.0.1:1/none")
	if err != nil {
		t.Fatal(err)
	}
	closed.Close()
	if err := RunEmbeddedMigrationsWithOptions(context.Background(), closed, MigrationOptions{}); err == nil {
		t.Fatal("a closed pool must fail to provide a migration connection")
	}
}
