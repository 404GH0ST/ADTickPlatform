.PHONY: fmt test build build-linux-arm64 ci e2e premerge release-notes audit-checker-contracts import-local-challenges \
	run-api-gateway run-api-gateway-postgres run-game-core run-submission-service \
	run-checker-runner run-controller-service run-scoring-worker run-realtime-gateway \
	run-wireguard-gateway run-backend-stack run-backend-stack-postgres \
	bootstrap-clean-match bootstrap-faust-target-shape finalize-faust-target-shape \
	validate-faust-target-shape report-faust-balance export-runtime-incident-bundle \
	create-admin create-teams simulate-attack-map-load validate-attack-map-load \
	prod-web-artifacts install-host-deps \
	monitoring-up monitoring-down monitoring-tail monitoring-status monitoring-clean \
	monitoring-up-prod monitoring-down-prod monitoring-tail-prod monitoring-status-prod monitoring-clean-prod \
	rotate-grafana-password-prod test-alerts

fmt:
	@mkdir -p $(GOCACHE)
	gofmt -w $$(find . -name '*.go' -not -path './node_modules/*')

test:
	@mkdir -p $(GOCACHE)
	GOCACHE=$(GOCACHE) go test -race ./...

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
	GOCACHE=$(GOCACHE) go test -race ./...
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

# Reads ADMIN_DISPLAY_NAME / ADMIN_EMAIL / ADMIN_PASSWORD from prod.env (or .env).
# Optional overrides: make create-admin DISPLAY_NAME=... EMAIL=... PASSWORD=...
create-admin:
	@if [ -n "$(PASSWORD)$(EMAIL)$(DISPLAY_NAME)" ]; then \
		./scripts/create-admin.sh "$(DISPLAY_NAME)" "$(EMAIL)" "$(PASSWORD)" "$(ADMIN_WIREGUARD_OUTPUT_DIR)"; \
	else \
		./scripts/create-admin.sh; \
	fi

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

# Production monitoring — same stack, different compose + scrape targets.
# Reads vars from deploy/compose/prod.env; requires the file to exist
# (copy from prod.env.example first). Prometheus targets use Docker service
# DNS names because in prod all services run in the same compose network.

MONITORING_PROD_COMPOSE := docker compose -f deploy/compose/prod.yml --env-file deploy/compose/prod.env

monitoring-up-prod:
	@test -f deploy/compose/prod.env || (echo "deploy/compose/prod.env not found; copy from prod.env.example"; exit 1)
	$(MONITORING_PROD_COMPOSE) up -d prometheus grafana
	@echo
	@echo "Grafana:    http://localhost:$${GRAFANA_HOST_PORT:-13000}"
	@echo "Prometheus: http://localhost:$${PROMETHEUS_HOST_PORT:-19090}"
	@echo "Both bind to the host only. The edge Caddy does NOT route to them."
	@echo "Grafana admin password: grep '^GRAFANA_ADMIN_PASSWORD=' deploy/compose/prod.env"

monitoring-down-prod:
	$(MONITORING_PROD_COMPOSE) stop prometheus grafana

monitoring-tail-prod:
	$(MONITORING_PROD_COMPOSE) logs -f prometheus grafana

monitoring-status-prod:
	$(MONITORING_PROD_COMPOSE) ps prometheus grafana

monitoring-clean-prod:
	$(MONITORING_PROD_COMPOSE) rm -fsv prometheus grafana
	docker volume rm -f ad-platform-prod_prometheus-data ad-platform-prod_grafana-data

rotate-grafana-password-prod:
	@test -f deploy/compose/prod.env || (echo "deploy/compose/prod.env not found; copy from prod.env.example first"; exit 1)
	./scripts/rotate-grafana-password.sh

# Run promtool's rule unit tests against deploy/prometheus/rules/adplatform-alerts.test.yml.
# Verifies each of the 5 alert rules actually fires under its target
# condition (no more silent-broken alerts). Pulls a one-shot promtool
# image so the host doesn't need a separate install.
test-alerts:
	docker run --rm -v "$(PWD)/deploy/prometheus/rules:/rules:ro" prom/prometheus:v2.55.1 \
		promtool test rules /rules/adplatform-alerts.test.yml
