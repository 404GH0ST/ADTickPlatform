package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

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
	info := httpapi.ServiceInfo{Name: "realtime-gateway", Version: "dev", Addr: config.String("REALTIME_GATEWAY_ADDR", ":8086")}

	gateway := newRealtimeGateway(
		newHTTPPublicSnapshotClient(
			config.String("REALTIME_SOURCE_URL", "http://127.0.0.1:8080"),
			config.Secret("REALTIME_SOURCE_ADMIN_TOKEN", "ADMIN_API_TOKEN"),
		),
		config.Duration("REALTIME_POLL_INTERVAL", 2*time.Second),
		config.Secret("REALTIME_ADMIN_TOKEN", "ADMIN_API_TOKEN"),
	).withSubscriberLimits(
		config.Int("REALTIME_MAX_SUBSCRIBERS", 2000),
		config.Int("REALTIME_MAX_ADMIN_SUBSCRIBERS", 32),
		config.Int("REALTIME_MAX_SUBSCRIBERS_PER_CLIENT", 12),
		config.Bool("REALTIME_TRUST_PROXY_HEADERS", false),
	).withShutdown(ctx.Done())
	httpapi.RegisterMetricsSource(info.Name, gateway)

	mux := httpapi.NewBaseMux(info)
	gateway.RegisterRoutes(mux)

	go gateway.Run(ctx)

	log.Printf("starting %s on %s", info.Name, info.Addr)
	return httpapi.RunServer(ctx, info, mux)
}
