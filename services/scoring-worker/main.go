package main

import (
	"context"
	"log"

	"adplatform/internal/platform/config"
	"adplatform/internal/platform/httpapi"
)

func main() {
	ctx := context.Background()
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
	if err := httpapi.RunServer(ctx, info, mux); err != nil {
		log.Fatal(err)
	}
}
