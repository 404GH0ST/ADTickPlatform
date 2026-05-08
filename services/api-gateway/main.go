package main

import (
	"context"
	"log"
	"strings"
	"time"

	"adplatform/internal/platform/config"
	"adplatform/internal/platform/database"
	"adplatform/internal/platform/httpapi"
	"adplatform/internal/services/apigateway"
)

func main() {
	ctx := context.Background()
	info := httpapi.ServiceInfo{
		Name:    "api-gateway",
		Version: "dev",
		Addr:    config.String("API_GATEWAY_ADDR", ":8080"),
	}

	store, err := buildStore(ctx)
	if err != nil {
		log.Fatal(err)
	}
	defer store.Close()

	teamJWTSecret := requiredSecret("TEAM_JWT_SECRET")
	adminAPIToken := requiredSecret("ADMIN_API_TOKEN")
	unlockProofSecret := requiredSecret("UNLOCK_PROOF_SECRET")
	sshCredentialSecret := requiredSecret("SSH_CREDENTIAL_SECRET")

	server := apigateway.NewWithDeps(
		teamJWTSecret,
		adminAPIToken,
		config.Int("API_GATEWAY_TEAM_ID", 101),
		store,
		apigateway.NewHTTPControllerClient(
			config.String("CONTROLLER_INTERNAL_URL", ""),
			config.String("CONTROLLER_INTERNAL_TOKEN", adminAPIToken),
		),
		apigateway.NewHTTPWireGuardClient(
			config.String("WIREGUARD_GATEWAY_INTERNAL_URL", ""),
			config.String("WIREGUARD_GATEWAY_INTERNAL_TOKEN", adminAPIToken),
		),
		apigateway.NewHTTPGameCoreClientWithTimeout(
			config.String("GAME_CORE_INTERNAL_URL", ""),
			config.String("GAME_CORE_INTERNAL_TOKEN", adminAPIToken),
			config.Duration("GAME_CORE_CLIENT_TIMEOUT", 3*time.Minute),
		),
	)
	server.WithSubmissionClient(
		apigateway.NewHTTPSubmissionClient(
			config.String("SUBMISSION_SERVICE_INTERNAL_URL", ""),
			config.String("SUBMISSION_SERVICE_INTERNAL_TOKEN", adminAPIToken),
		),
	)
	server.WithScoringClient(
		apigateway.NewHTTPScoringClient(
			config.String("SCORING_WORKER_INTERNAL_URL", ""),
			config.String("SCORING_WORKER_INTERNAL_TOKEN", adminAPIToken),
		),
	)
	server.WithUnlockProofSecret(unlockProofSecret)
	server.WithSSHCredentialSecret(sshCredentialSecret)
	server.WithRateLimiter(
		apigateway.NewRateLimiter(
			config.String("API_GATEWAY_RATE_LIMIT_REDIS_ADDR", config.String("REDIS_ADDR", "")),
			config.String("API_GATEWAY_RATE_LIMIT_REDIS_PASSWORD", config.String("REDIS_PASSWORD", "")),
		),
	)

	mux := httpapi.NewBaseMux(info)
	server.RegisterRoutes(mux)

	log.Printf("starting %s on %s", info.Name, info.Addr)
	if err := httpapi.RunServer(ctx, info, mux); err != nil {
		log.Fatal(err)
	}
}

func buildStore(ctx context.Context) (apigateway.Store, error) {
	backend := config.String("API_GATEWAY_STATE_BACKEND", "memory")
	if backend != "postgres" {
		return apigateway.NewMemoryStore(config.Int("API_GATEWAY_TEAM_ID", 101)), nil
	}

	db, err := database.OpenPostgres(ctx, config.String("POSTGRES_DSN", "postgres://adplatform:adplatform@localhost:5432/adplatform?sslmode=disable"))
	if err != nil {
		return nil, err
	}
	if config.Bool("API_GATEWAY_AUTO_MIGRATE", true) {
		if err := database.RunEmbeddedMigrationsWithOptions(ctx, db, database.MigrationOptions{
			IncludeSeeds: config.Bool("DATABASE_INCLUDE_SEEDS", config.Bool("API_GATEWAY_AUTO_SEED", false)),
		}); err != nil {
			db.Close()
			return nil, err
		}
	}
	return apigateway.NewPostgresStore(db), nil
}

func requiredSecret(key string) string {
	value := strings.TrimSpace(config.String(key, ""))
	if value == "" {
		log.Fatalf("%s must be set", key)
	}
	switch value {
	case "dev-team-token", "dev-admin-token", "change-this-admin-token", "change-this-team-jwt-secret", "change-this-unlock-proof-secret", "change-this-stable-ssh-credential-secret":
		log.Fatalf("%s must not use the example or development value", key)
	}
	return value
}
