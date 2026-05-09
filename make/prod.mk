.PHONY: firewall-cleanup compose-config prod-config prod-host-config preflight-prod-host \
	wg-host-keygen wg-host-render wg-host-install wg-host-up wg-host-down \
	wg-host-show wg-host-setup up-prod up-prod-host down-prod down-prod-host \
	logs-prod logs-prod-host

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
