package main

import (
	"context"
	"log"

	"adplatform/internal/platform/config"
	"adplatform/internal/platform/httpapi"
)

func main() {
	info := httpapi.ServiceInfo{Name: "checker-runner", Version: "dev", Addr: config.String("CHECKER_RUNNER_ADDR", ":8083")}
	server := newCheckerRunnerServer(config.Secret("CHECKER_RUNNER_INTERNAL_TOKEN", "ADMIN_API_TOKEN"), newCheckerExecutor())
	mux := httpapi.NewBaseMux(info)
	server.RegisterRoutes(mux)

	log.Printf("starting %s on %s", info.Name, info.Addr)
	if err := httpapi.RunServer(context.Background(), info, mux); err != nil {
		log.Fatal(err)
	}
}
