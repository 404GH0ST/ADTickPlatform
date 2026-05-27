package main

import (
	"context"
	"log"
	"time"

	"adplatform/internal/platform/config"
	"adplatform/internal/platform/httpapi"
)

func main() {
	ctx := context.Background()
	info := httpapi.ServiceInfo{Name: "realtime-gateway", Version: "dev", Addr: config.String("REALTIME_GATEWAY_ADDR", ":8086")}

	gateway := newRealtimeGateway(
		newHTTPPublicSnapshotClient(
			config.String("REALTIME_SOURCE_URL", "http://127.0.0.1:8080"),
			config.Secret("REALTIME_SOURCE_ADMIN_TOKEN", "ADMIN_API_TOKEN"),
		),
		config.Duration("REALTIME_POLL_INTERVAL", 2*time.Second),
		config.Secret("REALTIME_ADMIN_TOKEN", "ADMIN_API_TOKEN"),
	)
	httpapi.RegisterMetricsSource(info.Name, gateway)

	mux := httpapi.NewBaseMux(info)
	gateway.RegisterRoutes(mux)

	go gateway.Run(ctx)

	log.Printf("starting %s on %s", info.Name, info.Addr)
	if err := httpapi.RunServer(ctx, info, mux); err != nil {
		log.Fatal(err)
	}
}
