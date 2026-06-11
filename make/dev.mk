.PHONY: fmt test build build-linux-arm64 ci e2e premerge release-notes audit-checker-contracts import-local-challenges \
	run-api-gateway run-api-gateway-postgres run-game-core run-submission-service \
	run-checker-runner run-controller-service run-scoring-worker run-realtime-gateway \
	run-wireguard-gateway run-backend-stack run-backend-stack-postgres \
	bootstrap-clean-match bootstrap-faust-target-shape finalize-faust-target-shape \
	validate-faust-target-shape report-faust-balance export-runtime-incident-bundle \
	create-admin create-teams simulate-attack-map-load validate-attack-map-load \
	prod-web-artifacts install-host-deps \
	monitoring-up monitoring-down monitoring-tail monitoring-status monitoring-clean

fmt:
	@mkdir -p $(GOCACHE)
	gofmt -w $$(find . -name '*.go' -not -path './node_modules/*')

test:
	@mkdir -p $(GOCACHE)
	GOCACHE=$(GOCACHE) go test ./...

build:
	@mkdir -p $(GOCACHE)
	GOCACHE=$(GOCACHE) go build ./services/...

build-linux-arm64:
	@mkdir -p $(GOCACHE) bin/linux-arm64
	GOCACHE=$(GOCACHE) CGO_ENABLED=0 GOOS=$(LINUX_ARM64_TARGETOS) GOARCH=$(LINUX_ARM64_TARGETARCH) go build -o bin/linux-arm64/api-gateway ./services/api-gateway
	GOCACHE=$(GOCACHE) CGO_ENABLED=0 GOOS=$(LINUX_ARM64_TARGETOS) GOARCH=$(LINUX_ARM64_TARGETARCH) go build -o bin/linux-arm64/game-core ./services/game-core
	GOCACHE=$(GOCACHE) CGO_ENABLED=0 GOOS=$(LINUX_ARM64_TARGETOS) GOARCH=$(LINUX_ARM64_TARGETARCH) go build -o bin/linux-arm64/submission-service ./services/submission-service
	GOCACHE=$(GOCACHE) CGO_ENABLED=0 GOOS=$(LINUX_ARM64_TARGETOS) GOARCH=$(LINUX_ARM64_TARGETARCH) go build -o bin/linux-arm64/checker-runner ./services/checker-runner
	GOCACHE=$(GOCACHE) CGO_ENABLED=0 GOOS=$(LINUX_ARM64_TARGETOS) GOARCH=$(LINUX_ARM64_TARGETARCH) go build -o bin/linux-arm64/controller-service ./services/controller-service
	GOCACHE=$(GOCACHE) CGO_ENABLED=0 GOOS=$(LINUX_ARM64_TARGETOS) GOARCH=$(LINUX_ARM64_TARGETARCH) go build -o bin/linux-arm64/scoring-worker ./services/scoring-worker
	GOCACHE=$(GOCACHE) CGO_ENABLED=0 GOOS=$(LINUX_ARM64_TARGETOS) GOARCH=$(LINUX_ARM64_TARGETARCH) go build -o bin/linux-arm64/realtime-gateway ./services/realtime-gateway
	GOCACHE=$(GOCACHE) CGO_ENABLED=0 GOOS=$(LINUX_ARM64_TARGETOS) GOARCH=$(LINUX_ARM64_TARGETARCH) go build -o bin/linux-arm64/wireguard-gateway ./services/wireguard-gateway

ci:
	@mkdir -p $(GOCACHE)
	GOCACHE=$(GOCACHE) go test ./...
	./scripts/audit-checker-contracts.sh
	bun run web:typecheck
	bun run web:build

audit-checker-contracts:
	./scripts/audit-checker-contracts.sh

import-local-challenges:
	./scripts/import-local-challenges.sh

e2e:
	bun run web:e2e

premerge:
	$(MAKE) ci
	cd apps/web && PLAYWRIGHT_BROWSERS_PATH=$$(pwd)/.cache/ms-playwright bun run e2e

release-notes:
	bun run release-notes

run-api-gateway:
	@mkdir -p $(GOCACHE)
	@source scripts/lib/common.sh; load_default_env_files; load_runtime_env_if_present; ensure_local_runtime_env_file; GOCACHE=$(GOCACHE) go run ./services/api-gateway

run-api-gateway-postgres:
	@mkdir -p $(GOCACHE)
	@source scripts/lib/common.sh; load_default_env_files; load_runtime_env_if_present; ensure_local_runtime_env_file; API_GATEWAY_STATE_BACKEND=postgres GOCACHE=$(GOCACHE) go run ./services/api-gateway

run-game-core:
	@mkdir -p $(GOCACHE)
	@source scripts/lib/common.sh; load_default_env_files; load_runtime_env_if_present; ensure_local_runtime_env_file; GOCACHE=$(GOCACHE) go run ./services/game-core

run-submission-service:
	@mkdir -p $(GOCACHE)
	@source scripts/lib/common.sh; load_default_env_files; load_runtime_env_if_present; ensure_local_runtime_env_file; GOCACHE=$(GOCACHE) go run ./services/submission-service

run-checker-runner:
	@mkdir -p $(GOCACHE)
	@source scripts/lib/common.sh; load_default_env_files; load_runtime_env_if_present; ensure_local_runtime_env_file; GOCACHE=$(GOCACHE) go run ./services/checker-runner

run-controller-service:
	@mkdir -p $(GOCACHE)
	@source scripts/lib/common.sh; load_default_env_files; load_runtime_env_if_present; ensure_local_runtime_env_file; GOCACHE=$(GOCACHE) go run ./services/controller-service

run-scoring-worker:
	@mkdir -p $(GOCACHE)
	@source scripts/lib/common.sh; load_default_env_files; load_runtime_env_if_present; ensure_local_runtime_env_file; GOCACHE=$(GOCACHE) go run ./services/scoring-worker

run-realtime-gateway:
	@mkdir -p $(GOCACHE)
	@source scripts/lib/common.sh; load_default_env_files; load_runtime_env_if_present; ensure_local_runtime_env_file; GOCACHE=$(GOCACHE) go run ./services/realtime-gateway

run-wireguard-gateway:
	@mkdir -p $(GOCACHE)
	@source scripts/lib/common.sh; load_default_env_files; load_runtime_env_if_present; ensure_local_runtime_env_file; GOCACHE=$(GOCACHE) go run ./services/wireguard-gateway

run-backend-stack:
	./scripts/run-backend-stack.sh memory

run-backend-stack-postgres:
	./scripts/run-backend-stack.sh postgres

bootstrap-clean-match:
	./scripts/bootstrap-clean-match.sh

bootstrap-faust-target-shape:
	./scripts/bootstrap-faust-target-shape.sh

finalize-faust-target-shape:
	./scripts/finalize-faust-target-shape.sh

validate-faust-target-shape:
	./scripts/validate-faust-target-shape.sh

report-faust-balance:
	./scripts/report-faust-balance.sh

export-runtime-incident-bundle:
	./scripts/export-runtime-incident-bundle.sh

create-admin:
	./scripts/create-admin.sh "$(DISPLAY_NAME)" "$(EMAIL)" "$(PASSWORD)" "$(ADMIN_WIREGUARD_OUTPUT_DIR)"

create-teams:
	./scripts/create-teams.sh "$(TEAM_COUNT)" "$(TEAM_PREFIX)" "$(TEAM_EMAIL_DOMAIN)" "$(TEAM_START_INDEX)"

simulate-attack-map-load:
	./scripts/simulate-attack-map-load.sh "$(TEAM_COUNT)" "$(TEAM_PREFIX)" "$(TEAM_EMAIL_DOMAIN)" "$(TEAM_START_INDEX)"

validate-attack-map-load:
	./scripts/validate-attack-map-load.sh

prod-web-artifacts:
	bun run web:build

install-host-deps:
	./scripts/install-host-deps.sh

# --- Monitoring (Prometheus + Grafana) -------------------------------------
# Backend Go services run on the host via `make run-backend-stack-postgres`,
# so Prometheus in Docker scrapes them via host.docker.internal. If you run
# the backend inside docker compose too, change the targets in
# deploy/prometheus/prometheus.yml to the service names (e.g. api-gateway:8080).

MONITORING_COMPOSE := docker compose -f deploy/compose/dev.yml

monitoring-up:
	$(MONITORING_COMPOSE) up -d prometheus grafana
	@echo
	@echo "Grafana:    http://localhost:$${GRAFANA_HOST_PORT:-13000}  (default admin / adplatform)"
	@echo "Prometheus: http://localhost:$${PROMETHEUS_HOST_PORT:-19090}"
	@echo "Wait ~15s for first scrape, then open the 'ADTickPlatform — Match Overview' dashboard."

monitoring-down:
	$(MONITORING_COMPOSE) stop prometheus grafana

monitoring-tail:
	$(MONITORING_COMPOSE) logs -f prometheus grafana

monitoring-status:
	$(MONITORING_COMPOSE) ps prometheus grafana

monitoring-clean:
	$(MONITORING_COMPOSE) rm -fsv prometheus grafana
	docker volume rm -f ad-platform-local_prometheus-data ad-platform-local_grafana-data
