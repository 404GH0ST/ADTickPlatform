#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${ROOT_DIR}"
source "${ROOT_DIR}/scripts/lib/common.sh"

require_bin curl
require_bin jq
require_bin mktemp
require_bin sed
require_bin grep

PROD_ENV="${PROD_ENV:-deploy/compose/prod.env}"
load_env_file "${PROD_ENV}"

EDGE_BASE_URL="${PROD_EDGE_BASE_URL:-$(derive_edge_base_url)}"
SSE_TIMEOUT_SECONDS="${SSE_TIMEOUT_SECONDS:-5}"

root_response_file="$(mktemp)"
sse_response_file="$(mktemp)"
trap 'rm -f "${root_response_file}" "${sse_response_file}"' EXIT

echo "production edge smoke: base_url=${EDGE_BASE_URL}"

echo "health:"
curl -fsS "${EDGE_BASE_URL}/healthz" | jq -c '.'

echo "challenges:"
curl -fsS "${EDGE_BASE_URL}/api/v2/challenges" |
  jq -c 'map({id,name})'

echo "participant overview:"
curl -fsS "${EDGE_BASE_URL}/" >"${root_response_file}"
if ! grep -q 'Participant Overview' "${root_response_file}"; then
  echo "participant overview page did not contain expected title" >&2
  exit 1
fi
printf '  title matched: %s\n' 'Participant Overview'
printf '  preview: %s\n' "$(head -c 180 "${root_response_file}")"

echo "scoreboard stream:"
curl_status=0
if ! curl -sS -N --max-time "${SSE_TIMEOUT_SECONDS}" \
  -H 'Accept: text/event-stream' \
  "${EDGE_BASE_URL}/public/v1/scoreboard/stream" >"${sse_response_file}"; then
  curl_status=$?
fi

if [[ "${curl_status}" -ne 0 && "${curl_status}" -ne 28 ]]; then
  echo "scoreboard stream probe failed with curl exit code ${curl_status}" >&2
  exit "${curl_status}"
fi

if ! grep -q '^data:' "${sse_response_file}"; then
  echo "scoreboard stream did not emit an SSE data frame" >&2
  exit 1
fi
sed -n '1,4p' "${sse_response_file}"
