#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${ROOT_DIR}"
source "${ROOT_DIR}/scripts/lib/common.sh"

load_default_env_files
PROD_ENV="${PROD_ENV:-${ROOT_DIR}/deploy/compose/prod.env}"
load_env_file_override "${PROD_ENV}"

require_bin curl
require_bin jq

API_URL="${AD_PLATFORM_API_URL:-http://localhost:8080}"
PUBLIC_URL="${AD_PLATFORM_PUBLIC_BASE_URL:-}"
ADMIN_TOKEN="${ADMIN_API_TOKEN:-dev-admin-token}"
CHALLENGE_REPO_DIR="${CHALLENGE_REPO_DIR:-${ROOT_DIR}/../adplatform-challenges}"
CHALLENGE_CATALOG_PATH="${CHALLENGE_CATALOG_PATH:-${CHALLENGE_REPO_DIR}/catalog/challenges.json}"
IMPORT_VALIDATE="${IMPORT_VALIDATE:-false}"
IMPORT_DEPLOY="${IMPORT_DEPLOY:-false}"

if [[ "${API_URL}" == *"api-gateway"* ]] && [[ -n "${PUBLIC_URL}" ]]; then
  API_URL="${PUBLIC_URL}"
fi

usage() {
  cat <<'EOF'
Usage: scripts/import-local-challenges.sh

Imports challenge manifests from the sibling adplatform-challenges repository into
the platform admin API using local image names.

Environment overrides:
  CHALLENGE_REPO_DIR=../adplatform-challenges
  CHALLENGE_CATALOG_PATH=<repo>/catalog/challenges.json
  IMPORT_VALIDATE=false|true
  IMPORT_DEPLOY=false|true
  AD_PLATFORM_API_URL
  AD_PLATFORM_PUBLIC_BASE_URL
  ADMIN_API_TOKEN

Notes:
  - source_bundle_path is imported as "<slug>" relative to AD_CHALLENGE_SOURCE_ROOT.
  - to make participant source downloads work, point CHALLENGE_SOURCE_HOST_PATH at:
      <challenge repo>/challenges
EOF
}

if [[ "${1:-}" == "--help" ]]; then
  usage
  exit 0
fi

if [[ ! -f "${CHALLENGE_CATALOG_PATH}" ]]; then
  echo "challenge catalog not found: ${CHALLENGE_CATALOG_PATH}" >&2
  exit 1
fi

curl_json() {
  local label="$1"
  shift

  local response_file
  response_file="$(mktemp)"

  local status
  status="$(curl -sS -o "${response_file}" -w '%{http_code}' "$@")"
  if [[ "${status}" -lt 200 || "${status}" -ge 300 ]]; then
    echo "${label} failed (status=${status}):" >&2
    cat "${response_file}" >&2
    rm -f "${response_file}"
    exit 1
  fi

  cat "${response_file}"
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

admin_challenges="$(
  curl_json "list admin challenges" \
    "${API_URL}/api/v2/admin/challenges" \
    -H "Authorization: Bearer ${ADMIN_TOKEN}"
)"

mapfile -t manifests < <(jq -r '.[].manifest' "${CHALLENGE_CATALOG_PATH}")

for manifest_ref in "${manifests[@]}"; do
  manifest_path="${CHALLENGE_REPO_DIR}/${manifest_ref}"
  if [[ ! -f "${manifest_path}" ]]; then
    echo "challenge manifest not found: ${manifest_path}" >&2
    exit 1
  fi

  slug="$(jq -r '.slug' "${manifest_path}")"
  display_name="$(jq -r '.display_name' "${manifest_path}")"
  service_image="$(jq -r '.service_image' "${manifest_path}")"
  checker_image="$(jq -r '.checker_image' "${manifest_path}")"
  source_bundle_path="${slug}"
  weight="$(jq -r '(.weight // 1)' "${manifest_path}")"
  service_port="$(jq -r '(.service_port // 0)' "${manifest_path}")"
  service_subnet_octet="$(jq -r '(.service_subnet_octet // 0)' "${manifest_path}")"

  existing_challenge="$(
    printf '%s\n' "${admin_challenges}" | jq -c \
      --arg display_name "${display_name}" \
      --arg source_bundle_path "${source_bundle_path}" \
      'first(.[] | select(.name == $display_name or .source_bundle_path == $source_bundle_path)) // empty'
  )"

  payload="$(
    jq -nc \
      --arg name "${display_name}" \
      --arg baseline_image "${service_image}" \
      --arg checker_image "${checker_image}" \
      --arg source_bundle_path "${source_bundle_path}" \
      --argjson weight "${weight}" \
      --argjson service_port "${service_port}" \
      --argjson service_subnet_octet "${service_subnet_octet}" \
      '{
        name: $name,
        baseline_image: $baseline_image,
        checker_image: $checker_image,
        source_bundle_path: $source_bundle_path,
        weight: $weight
      }
      + (if $service_port > 0 then {service_port: $service_port} else {} end)
      + (if $service_subnet_octet > 0 then {service_subnet_octet: $service_subnet_octet} else {} end)'
  )"

  if [[ -n "${existing_challenge}" ]]; then
    challenge_id="$(printf '%s\n' "${existing_challenge}" | jq -r '.id')"
    echo "updating ${display_name} (${challenge_id})"
    curl_json "update challenge ${display_name}" \
      -X PUT "${API_URL}/api/v2/admin/challenges/${challenge_id}" \
      -H "Authorization: Bearer ${ADMIN_TOKEN}" \
      -H 'Content-Type: application/json' \
      -d "${payload}" >/dev/null
  else
    echo "creating ${display_name}"
    created="$(
      curl_json "create challenge ${display_name}" \
        -X POST "${API_URL}/api/v2/admin/challenges" \
        -H "Authorization: Bearer ${ADMIN_TOKEN}" \
        -H 'Content-Type: application/json' \
        -d "${payload}"
    )"
    challenge_id="$(printf '%s\n' "${created}" | jq -r '.id')"
  fi

  if [[ "${IMPORT_VALIDATE}" == "true" ]]; then
    echo "validating ${display_name}"
    curl_json "validate challenge ${display_name}" \
      -X POST "${API_URL}/api/v2/admin/challenges/${challenge_id}/validate" \
      -H "Authorization: Bearer ${ADMIN_TOKEN}" >/dev/null
  fi

  if [[ "${IMPORT_DEPLOY}" == "true" ]]; then
    echo "deploying ${display_name}"
    curl_json "deploy challenge ${display_name}" \
      -X POST "${API_URL}/api/v2/admin/challenges/${challenge_id}/deploy" \
      -H "Authorization: Bearer ${ADMIN_TOKEN}" >/dev/null
  fi
done

echo "local challenge import complete"
printf '  %s\n' \
  "challenge repo: ${CHALLENGE_REPO_DIR}" \
  "catalog: ${CHALLENGE_CATALOG_PATH}" \
  "validate after import: ${IMPORT_VALIDATE}" \
  "deploy after import: ${IMPORT_DEPLOY}"
