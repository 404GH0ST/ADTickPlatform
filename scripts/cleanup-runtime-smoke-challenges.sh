#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${ROOT_DIR}"
source "${ROOT_DIR}/scripts/lib/common.sh"
load_default_env_files
load_env_file_override "${ROOT_DIR}/.runtime/backend-stack.env"

require_bin curl
require_bin jq

API_URL="${AD_PLATFORM_API_URL:-http://127.0.0.1:8080}"
ADMIN_TOKEN="$(resolve_admin_api_token "${ROOT_DIR}/.runtime/backend-stack.env")"
PREFIX="${RUNTIME_SMOKE_PREFIX:-runtime-smoke-}"
APPLY=0

if [[ "${1:-}" == "--apply" ]]; then
  APPLY=1
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

matches="$(
  curl_json "challenge list" "${API_URL}/api/v2/admin/challenges" \
    -H "Authorization: Bearer ${ADMIN_TOKEN}" |
    jq -c --arg prefix "${PREFIX}" 'map(select(.name | startswith($prefix))) | sort_by(.id)'
)"

count="$(printf '%s\n' "${matches}" | jq 'length')"
echo "runtime smoke cleanup target count=${count} prefix=${PREFIX}"
printf '%s\n' "${matches}" | jq -c '.[] | {id,name}'

if [[ "${APPLY}" -ne 1 ]]; then
  echo "dry run only; re-run with --apply to delete the listed challenges"
  exit 0
fi

if [[ "${count}" -eq 0 ]]; then
  echo "nothing to delete"
  exit 0
fi

while IFS= read -r row; do
  [[ -n "${row}" ]] || continue
  challenge_id="$(printf '%s\n' "${row}" | jq -r '.id')"
  challenge_name="$(printf '%s\n' "${row}" | jq -r '.name')"
  status="$(curl -sS -o /dev/null -w '%{http_code}' -X DELETE \
    "${API_URL}/api/v2/admin/challenges/${challenge_id}" \
    -H "Authorization: Bearer ${ADMIN_TOKEN}")"
  if [[ "${status}" != "204" ]]; then
    echo "delete failed for challenge_id=${challenge_id} name=${challenge_name} status=${status}" >&2
    exit 1
  fi
  echo "deleted challenge_id=${challenge_id} name=${challenge_name}"
done < <(printf '%s\n' "${matches}" | jq -c '.[]')
