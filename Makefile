SHELL := /bin/bash
GOCACHE := $(CURDIR)/.cache/go-build

.PHONY: fmt test build ci e2e run-api-gateway run-api-gateway-postgres run-game-core run-submission-service run-checker-runner run-controller-service run-scoring-worker run-realtime-gateway run-wireguard-gateway run-backend-stack run-backend-stack-postgres bootstrap-clean-match smoke-participant smoke-admin-runtime smoke-sample-challenge-docker smoke-organizer-created smoke-prod-edge smoke-prod-host-enforcement smoke-prod-host-recovery smoke-prod-short-match smoke-prod-db-restore capture-prod-host-baseline go-live-check create-admin create-teams simulate-attack-map-load validate-attack-map-load compose-config prod-config prod-host-config preflight-prod-host prod-web-artifacts up-prod up-prod-host down-prod down-prod-host logs-prod logs-prod-host wg-host-keygen wg-host-render wg-host-install wg-host-up wg-host-down wg-host-show wg-host-setup

PROD_ENV ?= deploy/compose/prod.env
PROD_HOST_OVERRIDE ?= deploy/compose/prod.host-enforcement.yml
COMPOSE_PARALLEL_LIMIT ?= 1

fmt:
	@mkdir -p $(GOCACHE)
	gofmt -w $$(find . -name '*.go' -not -path './node_modules/*')

test:
	@mkdir -p $(GOCACHE)
	GOCACHE=$(GOCACHE) go test ./...

build:
	@mkdir -p $(GOCACHE)
	GOCACHE=$(GOCACHE) go build ./services/...

ci:
	@mkdir -p $(GOCACHE)
	GOCACHE=$(GOCACHE) go test ./...
	bun run web:typecheck
	bun run web:build

e2e:
	bun run web:e2e

run-api-gateway:
	@mkdir -p $(GOCACHE)
	GOCACHE=$(GOCACHE) go run ./services/api-gateway

run-api-gateway-postgres:
	@mkdir -p $(GOCACHE)
	API_GATEWAY_STATE_BACKEND=postgres GOCACHE=$(GOCACHE) go run ./services/api-gateway

run-game-core:
	@mkdir -p $(GOCACHE)
	GOCACHE=$(GOCACHE) go run ./services/game-core

run-submission-service:
	@mkdir -p $(GOCACHE)
	GOCACHE=$(GOCACHE) go run ./services/submission-service

run-checker-runner:
	@mkdir -p $(GOCACHE)
	GOCACHE=$(GOCACHE) go run ./services/checker-runner

run-controller-service:
	@mkdir -p $(GOCACHE)
	GOCACHE=$(GOCACHE) go run ./services/controller-service

run-scoring-worker:
	@mkdir -p $(GOCACHE)
	GOCACHE=$(GOCACHE) go run ./services/scoring-worker

run-realtime-gateway:
	@mkdir -p $(GOCACHE)
	GOCACHE=$(GOCACHE) go run ./services/realtime-gateway

run-wireguard-gateway:
	@mkdir -p $(GOCACHE)
	GOCACHE=$(GOCACHE) go run ./services/wireguard-gateway

run-backend-stack:
	./scripts/run-backend-stack.sh memory

run-backend-stack-postgres:
	./scripts/run-backend-stack.sh postgres

bootstrap-clean-match:
	./scripts/bootstrap-clean-match.sh

smoke-participant:
	./scripts/smoke-participant-flow.sh

smoke-admin-runtime:
	./scripts/smoke-admin-runtime-flow.sh

smoke-sample-challenge-docker:
	./scripts/smoke-sample-challenge-docker.sh

smoke-organizer-created:
	./scripts/smoke-organizer-created-flow.sh

smoke-prod-edge:
	./scripts/smoke-prod-edge.sh

smoke-prod-host-enforcement:
	./scripts/smoke-prod-host-enforcement.sh

smoke-prod-host-recovery:
	./scripts/smoke-prod-host-recovery.sh

smoke-prod-short-match:
	./scripts/smoke-prod-short-match.sh

smoke-prod-db-restore:
	./scripts/smoke-prod-db-restore.sh

capture-prod-host-baseline:
	./scripts/capture-prod-host-baseline.sh

go-live-check:
	./scripts/go-live-check.sh

create-admin:
	./scripts/create-admin.sh "$(DISPLAY_NAME)" "$(EMAIL)" "$(PASSWORD)"

create-teams:
	./scripts/create-teams.sh "$(TEAM_COUNT)" "$(TEAM_PREFIX)" "$(TEAM_EMAIL_DOMAIN)" "$(TEAM_START_INDEX)"

simulate-attack-map-load:
	./scripts/simulate-attack-map-load.sh "$(TEAM_COUNT)" "$(TEAM_PREFIX)" "$(TEAM_EMAIL_DOMAIN)" "$(TEAM_START_INDEX)"

validate-attack-map-load:
	./scripts/validate-attack-map-load.sh

firewall-cleanup:
	@echo "cleaning up all platform firewall rules..."
	@test -f $(PROD_ENV) && source $(PROD_ENV) || true; \
	curl -s -X POST -H "Authorization: Bearer $${ADMIN_API_TOKEN:-dev-admin-token}" $${AD_PLATFORM_API_URL:-http://localhost:8080}/api/v2/admin/access/teardown || true; \
	curl -s -X POST -H "Authorization: Bearer $${ADMIN_API_TOKEN:-dev-admin-token}" $${AD_PLATFORM_API_URL:-http://localhost:8080}/api/v2/admin/wireguard/teardown || true
	@echo "cleanup requested."

compose-config:
	docker compose -f deploy/compose/dev.yml config >/dev/null

prod-config:
	@test -f $(PROD_ENV) || { echo "missing $(PROD_ENV); copy deploy/compose/prod.env.example first"; exit 1; }
	docker compose --env-file $(PROD_ENV) -f deploy/compose/prod.yml config >/dev/null

prod-host-config:
	@test -f $(PROD_ENV) || { echo "missing $(PROD_ENV); copy deploy/compose/prod.env.example first"; exit 1; }
	docker compose --env-file $(PROD_ENV) -f deploy/compose/prod.yml -f $(PROD_HOST_OVERRIDE) config >/dev/null

preflight-prod-host:
	./scripts/preflight-prod-host.sh

wg-host-keygen:
	./scripts/host-wireguard.sh keygen

wg-host-render:
	./scripts/host-wireguard.sh render

wg-host-install:
	./scripts/host-wireguard.sh install

wg-host-up:
	./scripts/host-wireguard.sh up

wg-host-down:
	./scripts/host-wireguard.sh down

wg-host-show:
	./scripts/host-wireguard.sh show

wg-host-setup:
	./scripts/host-wireguard.sh setup

prod-web-artifacts:
	bun run web:build

up-prod: prod-web-artifacts
	@test -f $(PROD_ENV) || { echo "missing $(PROD_ENV); copy deploy/compose/prod.env.example first"; exit 1; }
	COMPOSE_PARALLEL_LIMIT=$(COMPOSE_PARALLEL_LIMIT) docker compose --env-file $(PROD_ENV) -f deploy/compose/prod.yml up -d --build

up-prod-host: prod-web-artifacts preflight-prod-host
	@test -f $(PROD_ENV) || { echo "missing $(PROD_ENV); copy deploy/compose/prod.env.example first"; exit 1; }
	COMPOSE_PARALLEL_LIMIT=$(COMPOSE_PARALLEL_LIMIT) docker compose --env-file $(PROD_ENV) -f deploy/compose/prod.yml -f $(PROD_HOST_OVERRIDE) up -d --build

down-prod:
	@test -f $(PROD_ENV) || { echo "missing $(PROD_ENV); copy deploy/compose/prod.env.example first"; exit 1; }
	docker compose --env-file $(PROD_ENV) -f deploy/compose/prod.yml down

down-prod-host:
	@test -f $(PROD_ENV) || { echo "missing $(PROD_ENV); copy deploy/compose/prod.env.example first"; exit 1; }
	docker compose --env-file $(PROD_ENV) -f deploy/compose/prod.yml -f $(PROD_HOST_OVERRIDE) down

logs-prod:
	@test -f $(PROD_ENV) || { echo "missing $(PROD_ENV); copy deploy/compose/prod.env.example first"; exit 1; }
	docker compose --env-file $(PROD_ENV) -f deploy/compose/prod.yml logs -f

logs-prod-host:
	@test -f $(PROD_ENV) || { echo "missing $(PROD_ENV); copy deploy/compose/prod.env.example first"; exit 1; }
	docker compose --env-file $(PROD_ENV) -f deploy/compose/prod.yml -f $(PROD_HOST_OVERRIDE) logs -f
