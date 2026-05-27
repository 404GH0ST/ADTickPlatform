.PHONY: firewall-cleanup generate-prod-env compose-config prod-config prod-host-config preflight-prod-host \
	wg-host-keygen wg-host-render wg-host-install wg-host-up wg-host-down \
	wg-host-show wg-host-setup up-prod up-prod-host down-prod down-prod-host \
	logs-prod logs-prod-host prod-config-arm64 prod-host-config-arm64 \
	up-prod-arm64 up-prod-host-arm64

firewall-cleanup:
	@echo "cleaning up all platform firewall rules..."
	@source scripts/lib/common.sh; \
	load_default_env_files; \
	test -f $(PROD_ENV) && load_env_file_override $(PROD_ENV) || true; \
	ADMIN_TOKEN="$$(resolve_admin_api_token .runtime/backend-stack.env)"; \
	API_URL="$${AD_PLATFORM_API_URL:-http://localhost:8080}"; \
	curl -s -X POST -H "Authorization: Bearer $${ADMIN_TOKEN}" "$${API_URL}/api/v2/admin/access/teardown" || true; \
	curl -s -X POST -H "Authorization: Bearer $${ADMIN_TOKEN}" "$${API_URL}/api/v2/admin/wireguard/teardown" || true
	@echo "cleanup requested."

compose-config:
	docker compose -f deploy/compose/dev.yml config >/dev/null

generate-prod-env:
	./scripts/generate-prod-env.sh $(PROD_ENV)

prod-config:
	@test -f $(PROD_ENV) || { echo "missing $(PROD_ENV); copy deploy/compose/prod.env.example first"; exit 1; }
	docker compose --env-file $(PROD_ENV) -f deploy/compose/prod.yml config >/dev/null

prod-host-config:
	@test -f $(PROD_ENV) || { echo "missing $(PROD_ENV); copy deploy/compose/prod.env.example first"; exit 1; }
	docker compose --env-file $(PROD_ENV) -f deploy/compose/prod.yml -f $(PROD_HOST_OVERRIDE) config >/dev/null

prod-config-arm64:
	@test -f $(PROD_ENV) || { echo "missing $(PROD_ENV); copy deploy/compose/prod.env.example first"; exit 1; }
	AD_PLATFORM_TARGETOS=$(LINUX_ARM64_TARGETOS) AD_PLATFORM_TARGETARCH=$(LINUX_ARM64_TARGETARCH) DOCKER_DEFAULT_PLATFORM=$(LINUX_ARM64_PLATFORM) docker compose --env-file $(PROD_ENV) -f deploy/compose/prod.yml config >/dev/null

prod-host-config-arm64:
	@test -f $(PROD_ENV) || { echo "missing $(PROD_ENV); copy deploy/compose/prod.env.example first"; exit 1; }
	AD_PLATFORM_TARGETOS=$(LINUX_ARM64_TARGETOS) AD_PLATFORM_TARGETARCH=$(LINUX_ARM64_TARGETARCH) DOCKER_DEFAULT_PLATFORM=$(LINUX_ARM64_PLATFORM) docker compose --env-file $(PROD_ENV) -f deploy/compose/prod.yml -f $(PROD_HOST_OVERRIDE) config >/dev/null

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

up-prod: prod-web-artifacts
	@test -f $(PROD_ENV) || { echo "missing $(PROD_ENV); copy deploy/compose/prod.env.example first"; exit 1; }
	COMPOSE_PARALLEL_LIMIT=$(COMPOSE_PARALLEL_LIMIT) docker compose --env-file $(PROD_ENV) -f deploy/compose/prod.yml up -d --build

up-prod-host: prod-web-artifacts preflight-prod-host
	@test -f $(PROD_ENV) || { echo "missing $(PROD_ENV); copy deploy/compose/prod.env.example first"; exit 1; }
	COMPOSE_PARALLEL_LIMIT=$(COMPOSE_PARALLEL_LIMIT) docker compose --env-file $(PROD_ENV) -f deploy/compose/prod.yml -f $(PROD_HOST_OVERRIDE) up -d --build

up-prod-arm64: prod-web-artifacts
	@test -f $(PROD_ENV) || { echo "missing $(PROD_ENV); copy deploy/compose/prod.env.example first"; exit 1; }
	AD_PLATFORM_TARGETOS=$(LINUX_ARM64_TARGETOS) AD_PLATFORM_TARGETARCH=$(LINUX_ARM64_TARGETARCH) DOCKER_DEFAULT_PLATFORM=$(LINUX_ARM64_PLATFORM) COMPOSE_PARALLEL_LIMIT=$(COMPOSE_PARALLEL_LIMIT) docker compose --env-file $(PROD_ENV) -f deploy/compose/prod.yml up -d --build

up-prod-host-arm64: prod-web-artifacts preflight-prod-host
	@test -f $(PROD_ENV) || { echo "missing $(PROD_ENV); copy deploy/compose/prod.env.example first"; exit 1; }
	AD_PLATFORM_TARGETOS=$(LINUX_ARM64_TARGETOS) AD_PLATFORM_TARGETARCH=$(LINUX_ARM64_TARGETARCH) DOCKER_DEFAULT_PLATFORM=$(LINUX_ARM64_PLATFORM) COMPOSE_PARALLEL_LIMIT=$(COMPOSE_PARALLEL_LIMIT) docker compose --env-file $(PROD_ENV) -f deploy/compose/prod.yml -f $(PROD_HOST_OVERRIDE) up -d --build

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
