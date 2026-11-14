// Package dbtest gives Postgres integration tests an isolated schema on a
// shared database, so tests with fixed fixture IDs can rerun safely.
package dbtest

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"testing"
	"time"

	"adplatform/internal/platform/database"
)

// IsolatedSchema opens the URL-form DSN in envVar inside a fresh schema named
// prefix_<nanos>, dropped on cleanup. It skips the test when envVar is unset.
func IsolatedSchema(t testing.TB, envVar, prefix string) *sql.DB {
	t.Helper()
	dsn := os.Getenv(envVar)
	if dsn == "" {
		t.Skip(envVar + " is not set")
	}
	ctx := context.Background()
	base, err := database.OpenPostgres(ctx, dsn)
	if err != nil {
		t.Fatalf("open postgres: %v", err)
	}
	schema := fmt.Sprintf("%s_%d", prefix, time.Now().UnixNano())
	if _, err := base.ExecContext(ctx, "CREATE SCHEMA "+schema); err != nil {
		base.Close()
		t.Fatalf("create schema %s: %v", schema, err)
	}
	parsed, err := url.Parse(dsn)
	if err != nil {
		base.Close()
		t.Fatalf("%s must be a URL-form DSN: %v", envVar, err)
	}
	query := parsed.Query()
	query.Set("search_path", schema)
	parsed.RawQuery = query.Encode()
	db, err := database.OpenPostgres(ctx, parsed.String())
	if err != nil {
		base.Close()
		t.Fatalf("open schema %s: %v", schema, err)
	}
	t.Cleanup(func() {
		db.Close()
		if _, err := base.ExecContext(context.Background(), "DROP SCHEMA "+schema+" CASCADE"); err != nil {
			t.Logf("drop test schema %s: %v", schema, err)
		}
		base.Close()
	})
	return db
}
