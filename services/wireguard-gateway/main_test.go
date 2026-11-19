package main

import (
	"context"
	"testing"
	"time"

	"adplatform/internal/platform/database/dbtest"
	"adplatform/internal/platform/servicetest"
)

func setMemoryGatewayEnv(t *testing.T) {
	t.Helper()
	t.Setenv("WIREGUARD_GATEWAY_ADDR", "127.0.0.1:0")
	t.Setenv("WIREGUARD_GATEWAY_STATE_BACKEND", "memory")
	t.Setenv("API_GATEWAY_STATE_BACKEND", "memory")
	t.Setenv("WIREGUARD_GATEWAY_MODE", "dry-run")
	t.Setenv("WIREGUARD_SERVER_PRIVATE_KEY", "")
	t.Setenv("WIREGUARD_GATEWAY_INTERNAL_TOKEN", "token")
}

func TestRunServesUntilCancelled(t *testing.T) {
	setMemoryGatewayEnv(t)
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	if err := run(ctx); err != nil {
		t.Fatalf("run must shut down cleanly, got %v", err)
	}
}

func TestRunFailsOnStartupRestoreError(t *testing.T) {
	setMemoryGatewayEnv(t)
	private, _ := defaultWireGuardGatewayKeypair()
	dir := t.TempDir()
	t.Setenv("WIREGUARD_SERVER_PRIVATE_KEY", private)
	t.Setenv("WIREGUARD_GATEWAY_MODE", "host")
	t.Setenv("WIREGUARD_GATEWAY_WG_BIN", "false")
	t.Setenv("WIREGUARD_GATEWAY_CONFIG_PATH", dir+"/wg0.conf")
	t.Setenv("WIREGUARD_GATEWAY_PEERS_PATH", dir+"/peers.json")
	t.Setenv("WIREGUARD_GATEWAY_RULES_PATH", dir+"/nft.conf")
	t.Setenv("WIREGUARD_GATEWAY_STATUS_PATH", dir+"/status.json")
	if err := run(context.Background()); err == nil {
		t.Fatal("a failed startup apply must abort startup")
	}
}

func TestMainStopsOnSIGTERM(t *testing.T) {
	setMemoryGatewayEnv(t)
	got := servicetest.CaptureFatal(t, &fatal)
	servicetest.RunMainUntilSIGTERM(t, main)
	if len(got()) != 0 {
		t.Fatalf("clean shutdown must not be fatal, got %v", got())
	}
}

func TestMainReportsStartupFailure(t *testing.T) {
	setMemoryGatewayEnv(t)
	t.Setenv("WIREGUARD_GATEWAY_STATE_BACKEND", "postgres")
	t.Setenv("POSTGRES_DSN", "postgres://nobody@127.0.0.1:1/none?sslmode=disable&connect_timeout=1")
	got := servicetest.CaptureFatal(t, &fatal)
	main()
	if len(got()) == 0 {
		t.Fatal("an unreachable database must be fatal")
	}
}

func TestBuildStorePostgres(t *testing.T) {
	db := dbtest.IsolatedSchema(t, "TEST_POSTGRES_DSN", "wg_buildstore_test")
	dsn := dbtest.SchemaDSN(t, "TEST_POSTGRES_DSN", db)
	t.Setenv("WIREGUARD_GATEWAY_STATE_BACKEND", "postgres")
	t.Setenv("WIREGUARD_GATEWAY_AUTO_MIGRATE", "true")
	t.Setenv("POSTGRES_DSN", dsn)
	store, err := buildStore(context.Background())
	if err != nil {
		t.Fatalf("build postgres store: %v", err)
	}
	store.Close()

	// A read-only session makes the migration step itself fail.
	t.Setenv("POSTGRES_DSN", dsn+"&default_transaction_read_only=on")
	fresh := dbtest.IsolatedSchema(t, "TEST_POSTGRES_DSN", "wg_buildstore_ro_test")
	t.Setenv("POSTGRES_DSN", dbtest.SchemaDSN(t, "TEST_POSTGRES_DSN", fresh)+"&default_transaction_read_only=on")
	if _, err := buildStore(context.Background()); err == nil {
		t.Fatal("a migration failure must be returned")
	}
}
