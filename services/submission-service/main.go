package main

import (
	"context"
	"log"

	"adplatform/internal/platform/config"
	"adplatform/internal/platform/httpapi"
)

func main() {
	ctx := context.Background()
	info := httpapi.ServiceInfo{Name: "submission-service", Version: "dev", Addr: config.String("SUBMISSION_SERVICE_ADDR", ":8082")}

	server := newSubmissionServiceServer(
		config.Secret("SUBMISSION_SERVICE_INTERNAL_TOKEN", "ADMIN_API_TOKEN"),
		newGameCoreSubmissionClient(
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
