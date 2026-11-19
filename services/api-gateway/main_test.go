package main

import (
	"context"
	"testing"
	"time"

	"adplatform/internal/platform/database/dbtest"
	"adplatform/internal/platform/servicetest"
)

func setGatewayEnv(t *testing.T) {
	t.Helper()
	t.Setenv("API_GATEWAY_ADDR", "127.0.0.1:0")
	t.Setenv("API_GATEWAY_STATE_BACKEND", "memory")
	t.Setenv("TEAM_JWT_SECRET", "unit-test-team-jwt-secret-91c2")
	t.Setenv("ADMIN_API_TOKEN", "unit-test-admin-token-7f3a")
	t.Setenv("SSH_CREDENTIAL_SECRET", "unit-test-ssh-secret-4d8e")
}

func TestRunServesUntilCancelled(t *testing.T) {
	setGatewayEnv(t)
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	if err := run(ctx); err != nil {
		t.Fatalf("run must shut down cleanly, got %v", err)
	}
}

func TestMainStopsOnSIGTERM(t *testing.T) {
	setGatewayEnv(t)
	got := servicetest.CaptureFatal(t, &fatal)
	servicetest.RunMainUntilSIGTERM(t, main)
	if len(got()) != 0 {
		t.Fatalf("clean shutdown must not be fatal, got %v", got())
	}
}

func TestMainReportsStartupFailure(t *testing.T) {
	setGatewayEnv(t)
	t.Setenv("API_GATEWAY_STATE_BACKEND", "postgres")
	t.Setenv("POSTGRES_DSN", "postgres://nobody@127.0.0.1:1/none?sslmode=disable&connect_timeout=1")
	got := servicetest.CaptureFatal(t, &fatal)
	main()
	if len(got()) == 0 {
		t.Fatal("an unreachable database must be fatal")
	}
}

func TestBuildStorePostgres(t *testing.T) {
	db := dbtest.IsolatedSchema(t, "TEST_POSTGRES_DSN", "gw_buildstore")
	t.Setenv("API_GATEWAY_STATE_BACKEND", "postgres")
	t.Setenv("API_GATEWAY_AUTO_MIGRATE", "true")
	t.Setenv("POSTGRES_DSN", dbtest.SchemaDSN(t, "TEST_POSTGRES_DSN", db))
	store, err := buildStore(context.Background())
	if err != nil {
		t.Fatalf("build postgres store: %v", err)
	}
	store.Close()

	readOnly := dbtest.IsolatedSchema(t, "TEST_POSTGRES_DSN", "gw_buildstore_ro")
	t.Setenv("POSTGRES_DSN", dbtest.SchemaDSN(t, "TEST_POSTGRES_DSN", readOnly)+"&default_transaction_read_only=on")
	if _, err := buildStore(context.Background()); err == nil {
		t.Fatal("a migration failure must be returned")
	}
}
