.PHONY: firewall-cleanup prod-runtime-cleanup generate-prod-env setup-prod-env compose-config prod-config prod-host-config preflight-prod-host \
	wg-host-keygen wg-host-render wg-host-install wg-host-up wg-host-down \
	wg-host-show wg-host-setup up-prod up-prod-host down-prod down-prod-host down-prod-host-clean \
	logs-prod logs-prod-host prod-config-arm64 prod-host-config-arm64 \
	up-prod-arm64 up-prod-host-arm64

firewall-cleanup:
	@echo "cleaning up all platform firewall rules..."
	@source scripts/lib/common.sh; \
	load_default_env_files; \
	test -f $(PROD_ENV) && load_env_file_override $(PROD_ENV) || true; \
	ADMIN_TOKEN="$$(resolve_admin_api_token .runtime/backend-stack.env)"; \
	API_URL="$${AD_PLATFORM_API_URL:-http://localhost:8080}"; \
	PUBLIC_URL="$${AD_PLATFORM_PUBLIC_BASE_URL:-}"; \
	if [[ "$${API_URL}" == *"api-gateway"* && -n "$${PUBLIC_URL}" ]]; then API_URL="$${PUBLIC_URL}"; fi; \
	CURL_TLS_ARGS=(); \
	if [[ "$${API_URL}" == https://* ]]; then \
		default_admin_ca_cert="deploy/caddy/certs/adplatform-selfsigned.crt"; \
		if [[ "$${ADMIN_CURL_INSECURE:-false}" == "true" ]]; then \
			CURL_TLS_ARGS=(-k); \
		elif [[ -n "$${ADMIN_CA_CERT:-}" && -f "$${ADMIN_CA_CERT}" ]]; then \
			CURL_TLS_ARGS=(--cacert "$${ADMIN_CA_CERT}"); \
		elif [[ "$${EDGE_TLS_DIRECTIVE:-}" == *"adplatform-selfsigned.crt"* && -f "$${default_admin_ca_cert}" ]]; then \
			CURL_TLS_ARGS=(--cacert "$${default_admin_ca_cert}"); \
		fi; \
	fi; \
	curl "$${CURL_TLS_ARGS[@]}" -sS -X POST -H "Authorization: Bearer $${ADMIN_TOKEN}" "$${API_URL}/api/v2/admin/access/teardown" || true; \
	curl "$${CURL_TLS_ARGS[@]}" -sS -X POST -H "Authorization: Bearer $${ADMIN_TOKEN}" "$${API_URL}/api/v2/admin/wireguard/teardown" || true
	@echo "cleanup requested."

prod-runtime-cleanup:
	@echo "removing controller-created service containers, volumes, and game networks..."
	@container_ids="$$(docker ps -aq --filter label=adplatform.team_id)"; \
	volume_names=""; \
	if [[ -n "$${container_ids}" ]]; then \
		while IFS= read -r container_id; do \
			[[ -n "$${container_id}" ]] || continue; \
			mounted="$$(docker inspect --format '{{range .Mounts}}{{if eq .Type "volume"}}{{println .Name}}{{end}}{{end}}' "$${container_id}" 2>/dev/null || true)"; \
			if [[ -n "$${mounted}" ]]; then \
				volume_names="$${volume_names}"$$'\n'"$${mounted}"; \
			fi; \
		done <<< "$${container_ids}"; \
		docker rm -f $${container_ids} >/dev/null 2>&1 || true; \
	fi; \
	if [[ -n "$${volume_names}" ]]; then \
		printf '%s\n' "$${volume_names}" | sort -u | while IFS= read -r volume_name; do \
			[[ -n "$${volume_name}" ]] || continue; \
			docker volume rm -f "$${volume_name}" >/dev/null 2>&1 || true; \
		done; \
	fi; \
	network_names="$$(docker network ls --filter label=adplatform.game_network=true --format '{{.Name}}')"; \
	if [[ -n "$${network_names}" ]]; then \
		while IFS= read -r network_name; do \
			[[ -n "$${network_name}" ]] || continue; \
			docker network rm "$${network_name}" >/dev/null 2>&1 || true; \
		done <<< "$${network_names}"; \
	fi
	@echo "runtime cleanup requested."

compose-config:
	docker compose -f deploy/compose/dev.yml config >/dev/null

# Optional overrides (env or make vars):
#   CHALLENGE_SOURCE_HOST_PATH=/path/to/sources
#   ADMIN_EMAIL=organizer@ctf.local
#   ADMIN_DISPLAY_NAME=Organizer
#   ADMIN_PASSWORD=...   # if unset, a random password is generated
#   FORCE=true           # overwrite existing prod.env
generate-prod-env:
	CHALLENGE_SOURCE_HOST_PATH="$(CHALLENGE_SOURCE_HOST_PATH)" \
	ADMIN_EMAIL="$(ADMIN_EMAIL)" \
	ADMIN_DISPLAY_NAME="$(ADMIN_DISPLAY_NAME)" \
	ADMIN_PASSWORD="$(ADMIN_PASSWORD)" \
	FORCE="$(FORCE)" \
	./scripts/generate-prod-env.sh $(PROD_ENV)

setup-prod-env:
	./scripts/setup-prod-env.sh "$(DOMAIN)" "$(or $(SCHEME),$(SCHEMA))" "$(WG_PORT)"


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

down-prod-host-clean: firewall-cleanup prod-runtime-cleanup
	@test -f $(PROD_ENV) || { echo "missing $(PROD_ENV); copy deploy/compose/prod.env.example first"; exit 1; }
	docker compose --env-file $(PROD_ENV) -f deploy/compose/prod.yml -f $(PROD_HOST_OVERRIDE) down -v

logs-prod:
	@test -f $(PROD_ENV) || { echo "missing $(PROD_ENV); copy deploy/compose/prod.env.example first"; exit 1; }
	docker compose --env-file $(PROD_ENV) -f deploy/compose/prod.yml logs -f

logs-prod-host:
	@test -f $(PROD_ENV) || { echo "missing $(PROD_ENV); copy deploy/compose/prod.env.example first"; exit 1; }
	docker compose --env-file $(PROD_ENV) -f deploy/compose/prod.yml -f $(PROD_HOST_OVERRIDE) logs -f
