#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${ROOT_DIR}"
source "${ROOT_DIR}/scripts/lib/common.sh"

require_bin docker

output_dir="${GO_LIVE_METRICS_OUTPUT_DIR:-}"
prod_env="${PROD_ENV:-deploy/compose/prod.env}"
host_override="${PROD_HOST_OVERRIDE:-deploy/compose/prod.host-enforcement.yml}"

usage() {
  cat <<'EOF'
Usage:
  GO_LIVE_METRICS_OUTPUT_DIR=.runtime/go-live-check-<timestamp> \
  scripts/capture-go-live-metrics.sh
EOF
}

if [[ -z "${output_dir}" ]]; then
  usage >&2
  exit 1
fi

mkdir -p "${output_dir}"

capture_service_metrics() {
  local service_name="$1"
  local metrics_url="$2"
  local output_file="$3"
  local compose_cmd=(
    docker compose
    --env-file "${prod_env}"
    -f deploy/compose/prod.yml
    -f "${host_override}"
  )
  local container_id=""

  container_id="$("${compose_cmd[@]}" ps -q "${service_name}" | tail -n 1)"
  if [[ -z "${container_id}" ]]; then
    echo "could not resolve container id for ${service_name}" >&2
    return 1
  fi

  if ! docker run --rm --network "container:${container_id}" curlimages/curl:8.12.1 -fsS "${metrics_url}" > "${output_file}"; then
    echo "failed to capture metrics from ${service_name} at ${metrics_url}" >&2
    echo "the running service may still be on a pre-metrics build; rebuild the host stack and rerun the capture" >&2
    return 1
  fi
}

capture_service_metrics "game-core" "http://127.0.0.1:8081/metrics" "${output_dir}/game-core-metrics.prom"
capture_service_metrics "submission-service" "http://127.0.0.1:8082/metrics" "${output_dir}/submission-service-metrics.prom"
capture_service_metrics "realtime-gateway" "http://127.0.0.1:8086/metrics" "${output_dir}/realtime-gateway-metrics.prom"

echo "go-live metrics captured:"
printf '  %s\n' \
  "${output_dir}/game-core-metrics.prom" \
  "${output_dir}/submission-service-metrics.prom" \
  "${output_dir}/realtime-gateway-metrics.prom"
