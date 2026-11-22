#!/usr/bin/env bash
set -euo pipefail
umask 077

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${ROOT_DIR}"
source "${ROOT_DIR}/scripts/lib/common.sh"

load_default_env_files
PROD_ENV="${PROD_ENV:-${ROOT_DIR}/deploy/compose/prod.env}"
load_env_file_override "${PROD_ENV}"

require_bin curl
require_bin jq
require_bin docker
require_bin tar
require_bin git

API_URL="${AD_PLATFORM_API_URL:-http://localhost:8080}"
PUBLIC_URL="${AD_PLATFORM_PUBLIC_BASE_URL:-}"
ADMIN_TOKEN="$(resolve_admin_api_token "${ROOT_DIR}/.runtime/backend-stack.env")"

if [[ "${API_URL}" == *"api-gateway"* ]] && [[ -n "${PUBLIC_URL}" ]]; then
  API_URL="${PUBLIC_URL}"
fi

INCIDENT_LOG_TAIL_LINES="${INCIDENT_LOG_TAIL_LINES:-120}"
INCIDENT_BUNDLE_OUTPUT_DIR="${INCIDENT_BUNDLE_OUTPUT_DIR:-.runtime/incident-bundle-$(date -u +"%Y%m%dT%H%M%SZ")}"
INCIDENT_BUNDLE_ARCHIVE="${INCIDENT_BUNDLE_ARCHIVE:-${INCIDENT_BUNDLE_OUTPUT_DIR}.tar.gz}"

usage() {
  cat <<'EOF'
Usage: scripts/export-runtime-incident-bundle.sh

Capture an operator-ready incident bundle from the current runtime state.

Environment overrides:
  INCIDENT_BUNDLE_OUTPUT_DIR=.runtime/incident-bundle-<timestamp>
  INCIDENT_BUNDLE_ARCHIVE=.runtime/incident-bundle-<timestamp>.tar.gz
  INCIDENT_LOG_TAIL_LINES=120
  AD_PLATFORM_API_URL
  AD_PLATFORM_PUBLIC_BASE_URL
  ADMIN_API_TOKEN
EOF
}

if [[ "${1:-}" == "--help" ]]; then
  usage
  exit 0
fi

if [[ -e "${INCIDENT_BUNDLE_OUTPUT_DIR}" || -L "${INCIDENT_BUNDLE_OUTPUT_DIR}" ]]; then
  echo "incident bundle output directory already exists: ${INCIDENT_BUNDLE_OUTPUT_DIR}" >&2
  exit 1
fi
if [[ -e "${INCIDENT_BUNDLE_ARCHIVE}" || -L "${INCIDENT_BUNDLE_ARCHIVE}" ]]; then
  echo "incident bundle archive already exists: ${INCIDENT_BUNDLE_ARCHIVE}" >&2
  exit 1
fi
mkdir -p "$(dirname "${INCIDENT_BUNDLE_OUTPUT_DIR}")"
mkdir -m 700 "${INCIDENT_BUNDLE_OUTPUT_DIR}"

curl_json() {
  local output_file="$1"
  shift

  local response_file
  response_file="$(mktemp)"

  local status
  if ! status="$(curl -s -o "${response_file}" -w '%{http_code}' "$@")"; then
    echo "request failed while fetching ${output_file##*/}" >&2
    rm -f "${response_file}"
    exit 1
  fi
  if [[ "${status}" -lt 200 || "${status}" -ge 300 ]]; then
    echo "request for ${output_file##*/} failed (status=${status})" >&2
    rm -f "${response_file}"
    exit 1
  fi

  cat "${response_file}" > "${output_file}"
  rm -f "${response_file}"
}

wait_for_operator_surface() {
  local base_url="$1"

  if wait_for_http "${base_url}/healthz" 60 "edge healthz"; then
    wait_for_http "${base_url}/api/v2/challenges" 60 "public challenges"
    return 0
  fi

  wait_for_http "${base_url}/api/v2/challenges" 60 "public challenges"
}

wait_for_operator_surface "${API_URL}"

curl_json "${INCIDENT_BUNDLE_OUTPUT_DIR}/deployments.json" \
  "${API_URL}/api/v2/admin/deployments" \
  -H @<(bearer_header "${ADMIN_TOKEN}")
curl_json "${INCIDENT_BUNDLE_OUTPUT_DIR}/access-status.json" \
  "${API_URL}/api/v2/admin/access/status" \
  -H @<(bearer_header "${ADMIN_TOKEN}")
curl_json "${INCIDENT_BUNDLE_OUTPUT_DIR}/wireguard-status.json" \
  "${API_URL}/api/v2/admin/wireguard/status" \
  -H @<(bearer_header "${ADMIN_TOKEN}")
curl_json "${INCIDENT_BUNDLE_OUTPUT_DIR}/operations-status.json" \
  "${API_URL}/api/v2/admin/operations/status" \
  -H @<(bearer_header "${ADMIN_TOKEN}")
curl_json "${INCIDENT_BUNDLE_OUTPUT_DIR}/game-status.json" \
  "${API_URL}/api/v2/admin/game/status" \
  -H @<(bearer_header "${ADMIN_TOKEN}")
curl_json "${INCIDENT_BUNDLE_OUTPUT_DIR}/scheduler-status.json" \
  "${API_URL}/api/v2/admin/game/scheduler" \
  -H @<(bearer_header "${ADMIN_TOKEN}")
curl_json "${INCIDENT_BUNDLE_OUTPUT_DIR}/scheduler-events.json" \
  "${API_URL}/api/v2/admin/game/scheduler/events?limit=50" \
  -H @<(bearer_header "${ADMIN_TOKEN}")
curl_json "${INCIDENT_BUNDLE_OUTPUT_DIR}/checker-runs.json" \
  "${API_URL}/api/v2/admin/game/checker-runs?limit=50" \
  -H @<(bearer_header "${ADMIN_TOKEN}")
curl_json "${INCIDENT_BUNDLE_OUTPUT_DIR}/scoreboard.json" \
  "${API_URL}/api/v2/admin/game/scoreboard" \
  -H @<(bearer_header "${ADMIN_TOKEN}")
curl_json "${INCIDENT_BUNDLE_OUTPUT_DIR}/scoring-audit.json" \
  "${API_URL}/api/v2/admin/game/scoring/audit" \
  -H @<(bearer_header "${ADMIN_TOKEN}")
curl_json "${INCIDENT_BUNDLE_OUTPUT_DIR}/attacks.json" \
  "${API_URL}/api/v2/attacks?limit=50"

cat > "${INCIDENT_BUNDLE_OUTPUT_DIR}/bundle-metadata.json" <<EOF
{
  "generated_at": "$(date -u +"%Y-%m-%dT%H:%M:%SZ")",
  "api_url": "${API_URL}",
  "git_commit": "$(git rev-parse HEAD)"
}
EOF

for service in \
  controller-service \
  api-gateway \
  realtime-gateway \
  wireguard-gateway \
  game-core \
  submission-service \
  web
do
  docker logs --tail "${INCIDENT_LOG_TAIL_LINES}" "ad-platform-prod-${service}-1" \
    > "${INCIDENT_BUNDLE_OUTPUT_DIR}/${service}.log" 2>&1 || true
done

jq -n \
  --arg generated_at "$(date -u +"%Y-%m-%dT%H:%M:%SZ")" \
  --argjson deployments "$(cat "${INCIDENT_BUNDLE_OUTPUT_DIR}/deployments.json")" \
  --argjson access "$(cat "${INCIDENT_BUNDLE_OUTPUT_DIR}/access-status.json")" \
  --argjson wireguard "$(cat "${INCIDENT_BUNDLE_OUTPUT_DIR}/wireguard-status.json")" \
  --argjson ops "$(cat "${INCIDENT_BUNDLE_OUTPUT_DIR}/operations-status.json")" \
  --argjson game "$(cat "${INCIDENT_BUNDLE_OUTPUT_DIR}/game-status.json")" \
  --argjson attacks "$(cat "${INCIDENT_BUNDLE_OUTPUT_DIR}/attacks.json")" \
  '{
    generated_at: $generated_at,
    runtime: {
      deployments_pending: ($deployments | map(select(.status != "completed")) | length),
      deployments_failed: ($deployments | map(select(.status == "failed" or .failed_team_count > 0)) | length),
      access_state: ($access.state // "unknown"),
      access_revision: ($access.revision // ""),
      wireguard_state: ($wireguard.state // "unknown"),
      wireguard_revision: ($wireguard.revision // ""),
      operations_healthy: ($ops.healthy // false),
      alert_count: (($ops.alerts // []) | length),
      total_ticks: ($game.total_ticks // 0),
      checker_runs_failed: ($game.failed_checker_runs // 0),
      accepted_attacks_visible: ($attacks.total_count // 0)
    }
  }' > "${INCIDENT_BUNDLE_OUTPUT_DIR}/incident-summary.json"

archive_tmp="$(mktemp "${INCIDENT_BUNDLE_ARCHIVE}.XXXXXX")"
trap 'rm -f -- "${archive_tmp}"' EXIT
tar -czf "${archive_tmp}" -C "$(dirname "${INCIDENT_BUNDLE_OUTPUT_DIR}")" "$(basename "${INCIDENT_BUNDLE_OUTPUT_DIR}")"
ln -- "${archive_tmp}" "${INCIDENT_BUNDLE_ARCHIVE}"
rm -f -- "${archive_tmp}"
trap - EXIT

echo "runtime incident bundle written:"
printf '  %s\n' "${INCIDENT_BUNDLE_OUTPUT_DIR}"
printf '  %s\n' "${INCIDENT_BUNDLE_ARCHIVE}"
