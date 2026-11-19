package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"adplatform/internal/platform/config"
	"adplatform/internal/platform/httpapi"
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
	info := httpapi.ServiceInfo{Name: "scoring-worker", Version: "dev", Addr: config.String("SCORING_WORKER_ADDR", ":8085")}

	server := newScoringWorkerServer(
		config.Secret("SCORING_WORKER_INTERNAL_TOKEN", "ADMIN_API_TOKEN"),
		newGameCoreScoringClient(
			config.String("GAME_CORE_INTERNAL_URL", ""),
			config.Secret("GAME_CORE_INTERNAL_TOKEN", "ADMIN_API_TOKEN"),
		),
	)
	httpapi.RegisterMetricsSource(info.Name, server)

	mux := httpapi.NewBaseMux(info)
	server.RegisterRoutes(mux)

	log.Printf("starting %s on %s", info.Name, info.Addr)
	return httpapi.RunServer(ctx, info, mux)
}
