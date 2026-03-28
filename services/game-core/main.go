package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"adplatform/internal/platform/config"
	"adplatform/internal/platform/database"
	"adplatform/internal/platform/httpapi"
)

func main() {
	ctx := context.Background()
	info := httpapi.ServiceInfo{Name: "game-core", Version: "dev", Addr: config.String("GAME_CORE_ADDR", ":8081")}

	store, err := buildStore(ctx)
	if err != nil {
		log.Fatal(err)
	}
	defer store.Close()

	server := newGameCoreServer(
		config.String("GAME_CORE_INTERNAL_TOKEN", config.String("ADMIN_API_TOKEN", "dev-admin-token")),
		store,
		newCheckerClient(
			config.String("CHECKER_RUNNER_INTERNAL_URL", ""),
			config.String("CHECKER_RUNNER_INTERNAL_TOKEN", config.String("ADMIN_API_TOKEN", "dev-admin-token")),
		),
		newFlagCodec(config.String("GAME_CORE_FLAG_SECRET", "dev-flag-secret")),
		nil,
		parseCheckerPhases(config.String("GAME_CORE_CHECKER_PHASES", "put,get,check")),
		config.Int("GAME_CORE_CHECKER_TIMEOUT_SECONDS", 15),
	)
	httpapi.RegisterMetricsSource(info.Name, server)
	matchStartAt, err := optionalRFC3339Env("GAME_CORE_MATCH_START_AT")
	if err != nil {
		log.Fatal(err)
	}
	matchEndAt, err := optionalRFC3339Env("GAME_CORE_MATCH_END_AT")
	if err != nil {
		log.Fatal(err)
	}
	if matchStartAt != nil && matchEndAt != nil && matchEndAt.Before(*matchStartAt) {
		log.Fatal("GAME_CORE_MATCH_END_AT must not be earlier than GAME_CORE_MATCH_START_AT")
	}
	server.matchStartAt = matchStartAt
	server.matchEndAt = matchEndAt
	scheduler := newIntervalGameScheduler(
		store,
		config.Duration("GAME_CORE_SCHEDULER_INTERVAL", 60*time.Second),
		config.Bool("GAME_CORE_SCHEDULER_ENABLED", false),
		server.advanceTick,
		server.matchStatus,
	)
	server.scheduler = scheduler
	defer scheduler.Close()
	matchMonitor := newMatchWindowMonitor(
		server,
		scheduler,
		time.Second,
	)
	defer matchMonitor.Close()

	mux := httpapi.NewBaseMux(info)
	server.RegisterRoutes(mux)
	log.Printf("starting %s on %s", info.Name, info.Addr)
	if err := httpapi.RunServer(ctx, info, mux); err != nil {
		_ = scheduler.Close()
		log.Fatal(err)
	}
}

func optionalRFC3339Env(key string) (*time.Time, error) {
	value, ok := os.LookupEnv(key)
	if !ok || strings.TrimSpace(value) == "" {
		return nil, nil
	}
	parsed, err := time.Parse(time.RFC3339, strings.TrimSpace(value))
	if err != nil {
		return nil, fmt.Errorf("%s must be RFC3339: %w", key, err)
	}
	utc := parsed.UTC()
	return &utc, nil
}

func buildStore(ctx context.Context) (gameStore, error) {
	backend := config.String("GAME_CORE_STATE_BACKEND", config.String("API_GATEWAY_STATE_BACKEND", "memory"))
	if backend != "postgres" {
		return newMemoryGameStore(), nil
	}

	db, err := database.OpenPostgres(ctx, config.String("POSTGRES_DSN", "postgres://adplatform:adplatform@localhost:5432/adplatform?sslmode=disable"))
	if err != nil {
		return nil, err
	}
	if config.Bool("GAME_CORE_AUTO_MIGRATE", config.Bool("API_GATEWAY_AUTO_MIGRATE", true)) {
		if err := database.RunEmbeddedMigrationsWithOptions(ctx, db, database.MigrationOptions{
			IncludeSeeds: config.Bool("DATABASE_INCLUDE_SEEDS", config.Bool("API_GATEWAY_AUTO_SEED", false)),
		}); err != nil {
			db.Close()
			return nil, err
		}
	}
	return newPostgresGameStore(db), nil
}

func parseCheckerPhases(value string) []string {
	parts := strings.Split(value, ",")
	phases := make([]string, 0, len(parts))
	seen := make(map[string]struct{}, len(parts))
	for _, part := range parts {
		phase := strings.ToLower(strings.TrimSpace(part))
		if phase == "" {
			continue
		}
		switch phase {
		case "put", "get", "check":
		default:
			continue
		}
		if _, ok := seen[phase]; ok {
			continue
		}
		seen[phase] = struct{}{}
		phases = append(phases, phase)
	}
	if len(phases) == 0 {
		return []string{"put", "get", "check"}
	}
	return phases
}
