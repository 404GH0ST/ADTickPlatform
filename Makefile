SHELL := /bin/bash
GOCACHE := $(CURDIR)/.cache/go-build
GOMODCACHE := $(CURDIR)/.cache/go-mod
.DEFAULT_GOAL := help

PROD_ENV ?= deploy/compose/prod.env
PROD_HOST_OVERRIDE ?= deploy/compose/prod.host-enforcement.yml
COMPOSE_PARALLEL_LIMIT ?= 1
LINUX_ARM64_TARGETOS ?= linux
LINUX_ARM64_TARGETARCH ?= arm64
LINUX_ARM64_PLATFORM ?= linux/arm64

export GOCACHE
export GOMODCACHE

include make/dev.mk
include make/smoke.mk
include make/prod.mk

.PHONY: help

help:
	@printf '%s\n' \
	  'Core:' \
	  '  make fmt | test | build | ci | e2e | premerge' \
	  '  make run-api-gateway | run-game-core | run-submission-service | run-checker-runner | run-controller-service | run-scoring-worker | run-realtime-gateway | run-wireguard-gateway' \
	  '' \
	  'Faust staging:' \
	  '  make bootstrap-faust-target-shape | finalize-faust-target-shape | validate-faust-target-shape | report-faust-balance' \
	  '  make export-runtime-incident-bundle' \
	  '  make import-local-challenges' \
	  '' \
	  'Security and health smokes:' \
	  '  make smoke-participant-authz | smoke-participant-rate-limits | smoke-participant-submission-abuse' \
	  '  make smoke-public-surface-audit | smoke-admin-realtime-authz | smoke-realtime-health | smoke-admin-runtime' \
	  '' \
	  'Prod host:' \
	  '  make generate-prod-env | preflight-prod-host | prod-config | prod-host-config | up-prod-host | down-prod-host | logs-prod-host' \
	  '  make prod-host-config-arm64 | up-prod-host-arm64 | build-linux-arm64' \
	  '  make go-live-check | validate-prod-release-candidate' \
	  '' \
	  'Release artifacts:' \
	  '  make capture-prod-host-baseline | capture-go-live-metrics | summarize-validation-artifacts | render-validation-report' \
	  '  make render-event-ready-note | verify-event-ready-note | verify-release-candidate | tag-release'
