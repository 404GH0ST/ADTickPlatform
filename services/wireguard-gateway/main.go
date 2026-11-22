package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"adplatform/internal/platform/config"
	"adplatform/internal/platform/database"
	"adplatform/internal/platform/httpapi"
	"adplatform/internal/services/apigateway"
)

// fatal reports a startup failure and exits; tests replace it.
var fatal = log.Fatal

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := run(ctx); err != nil {
		fatal(err)
	}
}

// run wires the gateway and serves until ctx is cancelled.
func run(ctx context.Context) error {
	info := httpapi.ServiceInfo{Name: "wireguard-gateway", Version: "dev", Addr: config.String("WIREGUARD_GATEWAY_ADDR", ":8087")}

	store, err := buildStore(ctx)
	if err != nil {
		return err
	}
	defer store.Close()

	server := newWireGuardGatewayServer(
		config.Secret("WIREGUARD_GATEWAY_INTERNAL_TOKEN", "ADMIN_API_TOKEN"),
		store,
		newWireGuardApplier(),
	)
	httpapi.RegisterMetricsSource(info.Name, server)
	if err := restoreWireGuardState(ctx, store, server); err != nil {
		return err
	}
	if interval := config.Duration("WIREGUARD_GATEWAY_RECONCILE_INTERVAL", 15*time.Second); interval > 0 {
		pollCtx, stopPoll := context.WithCancel(ctx)
		pollDone := make(chan struct{})
		go func() {
			defer close(pollDone)
			server.poll(pollCtx, interval)
		}()
		defer func() {
			stopPoll()
			<-pollDone
		}()
	}

	mux := httpapi.NewBaseMux(info)
	server.RegisterRoutes(mux)

	log.Printf("starting %s on %s", info.Name, info.Addr)
	return httpapi.RunServer(ctx, info, mux)
}

func buildStore(ctx context.Context) (apigateway.Store, error) {
	backend := config.String("WIREGUARD_GATEWAY_STATE_BACKEND", config.String("API_GATEWAY_STATE_BACKEND", "memory"))
	if backend != "postgres" {
		return apigateway.NewMemoryStore(config.Int("API_GATEWAY_TEAM_ID", 101)), nil
	}

	db, err := database.OpenPostgres(ctx, config.String("POSTGRES_DSN", "postgres://adplatform:adplatform@localhost:5432/adplatform?sslmode=disable"))
	if err != nil {
		return nil, err
	}
	if config.Bool("WIREGUARD_GATEWAY_AUTO_MIGRATE", config.Bool("API_GATEWAY_AUTO_MIGRATE", true)) {
		if err := database.RunEmbeddedMigrationsWithOptions(ctx, db, database.MigrationOptions{
			IncludeSeeds: config.Bool("DATABASE_INCLUDE_SEEDS", config.Bool("API_GATEWAY_AUTO_SEED", false)),
		}); err != nil {
			db.Close()
			return nil, err
		}
	}
	return apigateway.NewPostgresStore(db), nil
}
