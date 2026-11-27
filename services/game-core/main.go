package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strconv"
	"strings"
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

// run wires the service and serves until ctx is cancelled.
func run(ctx context.Context) error {
	info := httpapi.ServiceInfo{Name: "game-core", Version: "dev", Addr: config.String("GAME_CORE_ADDR", ":8081")}

	store, err := buildStore(ctx)
	if err != nil {
		return err
	}
	defer store.Close()
	// Recover from a crash that left a tick stuck in the "running" state.
	// StartNextTick refuses every future tick while the last one is "running",
	// so without this the scheduler would deadlock after any mid-tick blackout.
	if reaped, err := store.ReapRunningTicks(ctx); err != nil {
		return err
	} else if reaped > 0 {
		log.Printf("game-core: reaped %d incomplete tick(s) left running by a previous crash", reaped)
	}
	if err := recomputeScoreboardOnStartup(ctx, store); err != nil {
		return err
	}

	adminToken := config.Secret("GAME_CORE_INTERNAL_TOKEN", "ADMIN_API_TOKEN")
	configuredPrefix := strings.TrimSpace(config.String("GAME_CORE_FLAG_FORMAT_PREFIX", ""))
	if len(configuredPrefix) > apigateway.MaxFlagFormatPrefixBytes {
		return fmt.Errorf("GAME_CORE_FLAG_FORMAT_PREFIX must be at most %d bytes", apigateway.MaxFlagFormatPrefixBytes)
	}
	var maxTeamMembers *int
	if value := strings.TrimSpace(config.String("GAME_CORE_MAX_TEAM_MEMBERS", "")); value != "" {
		parsed, err := strconv.ParseInt(value, 10, 32)
		if err != nil || parsed < 0 {
			return errors.New("GAME_CORE_MAX_TEAM_MEMBERS must be an integer between 0 and 2147483647")
		}
		limit := int(parsed)
		maxTeamMembers = &limit
	}
	matchStartAt, err := optionalRFC3339Env("GAME_CORE_MATCH_START_AT")
	if err != nil {
		return err
	}
	matchEndAt, err := optionalRFC3339Env("GAME_CORE_MATCH_END_AT")
	if err != nil {
		return err
	}
	if matchStartAt != nil && matchEndAt != nil && matchEndAt.Before(*matchStartAt) {
		return errors.New("GAME_CORE_MATCH_END_AT must not be earlier than GAME_CORE_MATCH_START_AT")
	}
	flagSecret := config.RequiredSecret("GAME_CORE_FLAG_SECRET")
	flagPrefix, err := store.ApplyStartupPlatformSettings(ctx, configuredPrefix, maxTeamMembers)
	if err != nil {
		return fmt.Errorf("initialize platform settings: %w", err)
	}
	server := newGameCoreServer(
		adminToken,
		store,
		withPlayerCheckerSource(newCheckerClient(
			config.String("CHECKER_RUNNER_INTERNAL_URL", ""),
			config.Secret("CHECKER_RUNNER_INTERNAL_TOKEN", "ADMIN_API_TOKEN"),
		), store, config.String("GAME_CORE_CHECKER_SOURCE_MODE", "service")),
		newFlagCodec(flagSecret, flagPrefix),
		nil,
		parseCheckerPhases(config.String("GAME_CORE_CHECKER_PHASES", "put,get,check")),
		config.Int("GAME_CORE_CHECKER_TIMEOUT_SECONDS", 15),
	).
		WithCheckerParallelism(config.Int("GAME_CORE_CHECKER_PARALLELISM", 16)).
		WithTickTimeout(config.Duration("GAME_CORE_TICK_TIMEOUT", 180*time.Second)).
		WithScoringDebounce(config.Duration("GAME_CORE_SCORING_DEBOUNCE", time.Second)).
		WithScoringRetryDelay(config.Duration("GAME_CORE_SCORING_RETRY_DELAY", 5*time.Second)).
		WithScoringTimeout(config.Duration("GAME_CORE_SCORING_TIMEOUT", 30*time.Second)).
		WithWarmupRequired(config.Bool("GAME_CORE_WARMUP_REQUIRED", true)).
		WithWarmupTimeout(config.Duration("GAME_CORE_WARMUP_TIMEOUT_SECONDS", 60*time.Second)).
		WithWarmupMinSuccessRate(config.Float("GAME_CORE_WARMUP_MIN_SUCCESS_RATE", 1.0)).
		WithWarmupPutRetries(config.Int("GAME_CORE_WARMUP_PUT_RETRIES", 2)).
		WithWarmupPutRetryDelay(config.Duration("GAME_CORE_WARMUP_PUT_RETRY_DELAY", 2*time.Second)).
		WithAutoTickOnMatchStart(config.Bool("GAME_CORE_AUTO_TICK_ON_MATCH_START", true))
	// After each tick, re-apply access so challenges deferred to this tick open.
	server.accessReconcile = newAccessReconcileClient(
		config.String("CONTROLLER_INTERNAL_URL", ""),
		config.Secret("CONTROLLER_INTERNAL_TOKEN", "ADMIN_API_TOKEN"),
	)
	httpapi.RegisterMetricsSource(info.Name, server)
	server.matchStartAt = matchStartAt
	server.matchEndAt = matchEndAt
	scheduler := newIntervalGameScheduler(
		store,
		config.Duration("GAME_CORE_SCHEDULER_INTERVAL", 300*time.Second),
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
	stopRetentionMonitor := server.startRetentionMonitor()
	defer stopRetentionMonitor()
	stopSubmissionHistoryPruner := server.startSubmissionHistoryPruner(ctx)
	defer stopSubmissionHistoryPruner()
	stopAccessRetry := server.startAccessReconcileRetry(ctx)
	defer stopAccessRetry()

	mux := httpapi.NewBaseMux(info)
	server.RegisterRoutes(mux)
	log.Printf("starting %s on %s", info.Name, info.Addr)
	// Returning runs the deferred scheduler, monitor, and store shutdowns.
	return httpapi.RunServer(ctx, info, mux)
}

func recomputeScoreboardOnStartup(ctx context.Context, store gameStore) error {
	timeout := config.Duration("GAME_CORE_STARTUP_RECOMPUTE_TIMEOUT", 30*time.Second)
	return recomputeScoreboardWithTimeout(ctx, timeout, func(recomputeCtx context.Context) error {
		_, err := store.RecomputeScoreboard(recomputeCtx)
		return err
	})
}

func recomputeScoreboardWithTimeout(ctx context.Context, timeout time.Duration, recompute func(context.Context) error) error {
	recomputeCtx := ctx
	cancel := func() {}
	if timeout > 0 {
		recomputeCtx, cancel = context.WithTimeout(ctx, timeout)
	}
	defer cancel()

	if err := recompute(recomputeCtx); err != nil {
		return fmt.Errorf("initial scoreboard recompute failed: %w", err)
	}
	return nil
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
