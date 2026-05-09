.PHONY: fmt test build ci e2e release-notes \
	run-api-gateway run-api-gateway-postgres run-game-core run-submission-service \
	run-checker-runner run-controller-service run-scoring-worker run-realtime-gateway \
	run-wireguard-gateway run-backend-stack run-backend-stack-postgres \
	bootstrap-clean-match bootstrap-faust-target-shape finalize-faust-target-shape \
	validate-faust-target-shape report-faust-balance export-runtime-incident-bundle \
	create-admin create-teams simulate-attack-map-load validate-attack-map-load \
	prod-web-artifacts

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

release-notes:
	bun run release-notes

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
