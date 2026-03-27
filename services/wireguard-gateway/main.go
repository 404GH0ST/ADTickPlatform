package main

import (
	"context"
	"log"

	"adplatform/internal/platform/config"
	"adplatform/internal/platform/database"
	"adplatform/internal/platform/httpapi"
	"adplatform/internal/services/apigateway"
)

func main() {
	ctx := context.Background()
	info := httpapi.ServiceInfo{Name: "wireguard-gateway", Version: "dev", Addr: config.String("WIREGUARD_GATEWAY_ADDR", ":8087")}

	store, err := buildStore(ctx)
	if err != nil {
		log.Fatal(err)
	}
	defer store.Close()

	server := newWireGuardGatewayServer(
		config.String("WIREGUARD_GATEWAY_INTERNAL_TOKEN", config.String("ADMIN_API_TOKEN", "dev-admin-token")),
		store,
		newWireGuardApplier(),
	)
	if err := restoreWireGuardState(ctx, store, server); err != nil {
		log.Fatal(err)
	}

	mux := httpapi.NewBaseMux(info)
	server.RegisterRoutes(mux)

	log.Printf("starting %s on %s", info.Name, info.Addr)
	if err := httpapi.RunServer(ctx, info, mux); err != nil {
		log.Fatal(err)
	}
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
