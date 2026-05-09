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
require_bin awk

EDGE_BASE_URL="${PROD_EDGE_BASE_URL:-$(derive_edge_base_url)}"

wait_for_http "${EDGE_BASE_URL}/healthz" 60 "edge healthz"

http_body() {
  curl -fsS "$1"
}

assert_no_forbidden_keys() {
  local label="$1" body="$2"
  shift 2
  local forbidden
  for forbidden in "$@"; do
    if printf '%s' "${body}" | jq -e --arg key "${forbidden}" '
      paths(scalars) as $p
      | select($p[-1] == $key)
    ' >/dev/null; then
      echo "${label} leaked forbidden key: ${forbidden}" >&2
      exit 1
    fi
  done
}

assert_jq() {
  local label="$1" body="$2" expr="$3"
  if ! printf '%s' "${body}" | jq -e "${expr}" >/dev/null; then
    echo "${label} failed jq assertion: ${expr}" >&2
    printf '%s\n' "${body}" >&2
    exit 1
  fi
}

first_sse_payload() {
  local url="$1"
  curl -NsS --max-time 10 "${url}" | awk '/^data: /{sub(/^data: /,""); print; exit}'
}

http_status_body() {
  local body_file status
  body_file="$(mktemp)"
  status="$(curl -sS -o "${body_file}" -w '%{http_code}' "$@")"
  printf '%s\n' "${status}"
  cat "${body_file}"
  rm -f "${body_file}"
}

echo "public surface audit: base_url=${EDGE_BASE_URL}"

forbidden_fields=(
  baseline_image
  checker_image
  source_bundle_path
  service_port
  service_subnet_octet
  password
  token
  admin_api_token
  internal_token
  endpoint
  ssh_hint
  wireguard_peer
  wireguard_address
  wireguard_status
  role
  email
  contact_email
  checker_image
  output
)

challenges_body="$(http_body "${EDGE_BASE_URL}/api/v2/challenges")"
assert_jq \
  "public challenges" \
  "${challenges_body}" \
  'type == "array" and all(.[]; ((keys_unsorted - ["id","name","has_source_download"]) | length) == 0)'
assert_no_forbidden_keys "public challenges" "${challenges_body}" "${forbidden_fields[@]}"
echo "public challenges surface is minimal"

scoreboard_body="$(http_body "${EDGE_BASE_URL}/api/v2/scoreboard")"
assert_jq \
  "public scoreboard" \
  "${scoreboard_body}" \
  'type == "array" and all(.[]; ((keys_unsorted - ["rank","team","attack","defense","sla","total","delta","services"]) | length) == 0 and (((.services // []) | all(.[]; ((keys_unsorted - ["challenge_id","service","attack","defense","sla","total"]) | length) == 0))))'
assert_no_forbidden_keys "public scoreboard" "${scoreboard_body}" "${forbidden_fields[@]}"
echo "public scoreboard surface is minimal"

game_status_body="$(http_body "${EDGE_BASE_URL}/api/v2/game/status")"
assert_jq \
  "public game status" \
  "${game_status_body}" \
  'type == "object"
   and ((keys_unsorted - ["match","current_tick","scheduler","total_ticks","total_checker_runs","successful_checker_runs","failed_checker_runs","skipped_checker_runs"]) | length) == 0
   and ((.match == null) or (((.match | keys_unsorted) - ["state","started_at","ended_at","scheduled_start_at","scheduled_end_at","schedule_configured","accepting_submissions"]) | length) == 0)
   and ((.current_tick == null) or (((.current_tick | keys_unsorted) - ["id","status","total_checker_runs","successful_checker_runs","failed_checker_runs","skipped_checker_runs","started_at","completed_at","message"]) | length) == 0)
   and ((.scheduler == null) or (((.scheduler | keys_unsorted) - ["state","interval_seconds","last_run_at","next_run_at","last_tick_id","last_error"]) | length) == 0)'
assert_no_forbidden_keys "public game status" "${game_status_body}" "${forbidden_fields[@]}"
echo "public game status surface is minimal"

attacks_body="$(http_body "${EDGE_BASE_URL}/api/v2/attacks")"
assert_jq \
  "public attacks" \
  "${attacks_body}" \
  'type == "object"
   and ((keys_unsorted - ["items","limit","offset","total_count","has_prev","has_next"]) | length) == 0
   and ((.items // []) | all(.[]; ((keys_unsorted - ["id","attacker","victim","service","tick","verdict"]) | length) == 0))'
assert_no_forbidden_keys "public attacks" "${attacks_body}" "${forbidden_fields[@]}"
echo "public attacks surface is minimal"

scoreboard_stream_response="$(http_status_body --max-time 10 "${EDGE_BASE_URL}/api/platform/realtime/scoreboard/stream")"
scoreboard_stream_status="$(printf '%s\n' "${scoreboard_stream_response}" | sed -n '1p')"
scoreboard_stream_body="$(printf '%s\n' "${scoreboard_stream_response}" | tail -n +2)"
if [[ "${scoreboard_stream_status}" == "200" ]]; then
  scoreboard_stream_payload="$(printf '%s\n' "${scoreboard_stream_body}" | awk '/^data: /{sub(/^data: /,""); print; exit}')"
  if [[ -z "${scoreboard_stream_payload}" ]]; then
    echo "public realtime scoreboard stream did not yield an SSE payload" >&2
    exit 1
  fi
  assert_jq \
    "public realtime scoreboard" \
    "${scoreboard_stream_payload}" \
    'type == "array" and all(.[]; ((keys_unsorted - ["rank","team","attack","defense","sla","total","delta","services"]) | length) == 0 and (((.services // []) | all(.[]; ((keys_unsorted - ["challenge_id","service","attack","defense","sla","total"]) | length) == 0))))'
  assert_no_forbidden_keys "public realtime scoreboard" "${scoreboard_stream_payload}" "${forbidden_fields[@]}"
  echo "public realtime scoreboard surface is minimal"
elif [[ "${scoreboard_stream_status}" == "502" ]]; then
  assert_jq \
    "public realtime scoreboard failure" \
    "${scoreboard_stream_body}" \
    'type == "object" and ((keys_unsorted - ["status","message"]) | length) == 0 and .status == "failed"'
  assert_no_forbidden_keys "public realtime scoreboard failure" "${scoreboard_stream_body}" "${forbidden_fields[@]}"
  echo "public realtime scoreboard failure is generic"
else
  echo "unexpected public realtime scoreboard status ${scoreboard_stream_status}" >&2
  printf '%s\n' "${scoreboard_stream_body}" >&2
  exit 1
fi

attacks_stream_response="$(http_status_body --max-time 10 "${EDGE_BASE_URL}/api/platform/realtime/attacks/stream")"
attacks_stream_status="$(printf '%s\n' "${attacks_stream_response}" | sed -n '1p')"
attacks_stream_body="$(printf '%s\n' "${attacks_stream_response}" | tail -n +2)"
if [[ "${attacks_stream_status}" == "200" ]]; then
  attacks_stream_payload="$(printf '%s\n' "${attacks_stream_body}" | awk '/^data: /{sub(/^data: /,""); print; exit}')"
  if [[ -z "${attacks_stream_payload}" ]]; then
    echo "public realtime attacks stream did not yield an SSE payload" >&2
    exit 1
  fi
  assert_jq \
    "public realtime attacks" \
    "${attacks_stream_payload}" \
    'type == "object"
     and ((keys_unsorted - ["items","limit","offset","total_count","has_prev","has_next"]) | length) == 0
     and ((.items // []) | all(.[]; ((keys_unsorted - ["id","attacker","victim","service","tick","verdict"]) | length) == 0))'
  assert_no_forbidden_keys "public realtime attacks" "${attacks_stream_payload}" "${forbidden_fields[@]}"
  echo "public realtime attacks surface is minimal"
elif [[ "${attacks_stream_status}" == "502" ]]; then
  assert_jq \
    "public realtime attacks failure" \
    "${attacks_stream_body}" \
    'type == "object" and ((keys_unsorted - ["status","message"]) | length) == 0 and .status == "failed"'
  assert_no_forbidden_keys "public realtime attacks failure" "${attacks_stream_body}" "${forbidden_fields[@]}"
  echo "public realtime attacks failure is generic"
else
  echo "unexpected public realtime attacks status ${attacks_stream_status}" >&2
  printf '%s\n' "${attacks_stream_body}" >&2
  exit 1
fi

echo "public surface audit passed"
