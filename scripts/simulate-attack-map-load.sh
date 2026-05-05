#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${ROOT_DIR}"
source "${ROOT_DIR}/scripts/lib/common.sh"

load_default_env_files
PROD_ENV="${PROD_ENV:-${ROOT_DIR}/deploy/compose/prod.env}"
load_env_file_override "${PROD_ENV}"

require_bin curl
require_bin docker
require_bin jq

API_URL="${AD_PLATFORM_API_URL:-http://localhost:8080}"
PUBLIC_URL="${AD_PLATFORM_PUBLIC_BASE_URL:-}"
ADMIN_TOKEN="${ADMIN_API_TOKEN:-dev-admin-token}"

if [[ "${API_URL}" == *"api-gateway"* ]] && [[ -n "${PUBLIC_URL}" ]]; then
  API_URL="${PUBLIC_URL}"
fi

TEAM_COUNT="${TEAM_COUNT:-${1:-12}}"
TEAM_PREFIX="${TEAM_PREFIX:-${2:-Map Team}}"
TEAM_EMAIL_DOMAIN="${TEAM_EMAIL_DOMAIN:-${3:-teams.local}}"
TEAM_START_INDEX="${TEAM_START_INDEX:-${4:-1}}"
ATTACK_MAP_LOAD_ROUNDS="${ATTACK_MAP_LOAD_ROUNDS:-1}"
ATTACK_MAP_LOAD_RESET="${ATTACK_MAP_LOAD_RESET:-true}"
ATTACK_MAP_LOAD_BUILD_IMAGES="${ATTACK_MAP_LOAD_BUILD_IMAGES:-true}"
ATTACK_MAP_LOAD_ARTIFACT_FILE="${ATTACK_MAP_LOAD_ARTIFACT_FILE:-}"
ATTACK_MAP_LOAD_REPORT_FILE="${ATTACK_MAP_LOAD_REPORT_FILE:-}"
ATTACK_MAP_LOAD_MIN_SUBMISSIONS_PER_SECOND="${ATTACK_MAP_LOAD_MIN_SUBMISSIONS_PER_SECOND:-}"
ATTACK_MAP_LOAD_MAX_FLAG_FETCH_P95_MS="${ATTACK_MAP_LOAD_MAX_FLAG_FETCH_P95_MS:-}"
ATTACK_MAP_LOAD_MAX_SUBMISSION_P95_MS="${ATTACK_MAP_LOAD_MAX_SUBMISSION_P95_MS:-}"
ATTACK_MAP_LOAD_MAX_RECOMPUTE_MS="${ATTACK_MAP_LOAD_MAX_RECOMPUTE_MS:-}"
ATTACK_MAP_LOAD_MAX_ATTACK_FEED_LAG_MS="${ATTACK_MAP_LOAD_MAX_ATTACK_FEED_LAG_MS:-}"
ATTACK_MAP_LOAD_MAX_TICK_ADVANCE_P95_MS="${ATTACK_MAP_LOAD_MAX_TICK_ADVANCE_P95_MS:-}"
ATTACK_MAP_LOAD_ATTACK_FEED_TIMEOUT_MS="${ATTACK_MAP_LOAD_ATTACK_FEED_TIMEOUT_MS:-15000}"
ATTACK_MAP_LOAD_ATTACK_FEED_POLL_INTERVAL_MS="${ATTACK_MAP_LOAD_ATTACK_FEED_POLL_INTERVAL_MS:-250}"
ATTACK_MAP_LOAD_VALIDATE_CHECKER_RUNS="${ATTACK_MAP_LOAD_VALIDATE_CHECKER_RUNS:-false}"
BASELINE_IMAGE="${SAMPLE_CHALLENGE_BASELINE_IMAGE:-adplatform/sample-http:baseline}"
CHECKER_IMAGE="${SAMPLE_CHALLENGE_CHECKER_IMAGE:-adplatform/sample-http-checker:latest}"

usage() {
  cat <<'EOF'
Usage: scripts/simulate-attack-map-load.sh [team_count] [team_prefix] [email_domain] [start_index]

Environment overrides:
  TEAM_COUNT
  TEAM_PREFIX
  TEAM_EMAIL_DOMAIN
  TEAM_START_INDEX
  ATTACK_MAP_LOAD_ROUNDS
  ATTACK_MAP_LOAD_RESET=true|false
  ATTACK_MAP_LOAD_BUILD_IMAGES=true|false
  ATTACK_MAP_LOAD_ARTIFACT_FILE
  ATTACK_MAP_LOAD_REPORT_FILE
  ATTACK_MAP_LOAD_MIN_SUBMISSIONS_PER_SECOND
  ATTACK_MAP_LOAD_MAX_FLAG_FETCH_P95_MS
  ATTACK_MAP_LOAD_MAX_SUBMISSION_P95_MS
  ATTACK_MAP_LOAD_MAX_RECOMPUTE_MS
  ATTACK_MAP_LOAD_MAX_ATTACK_FEED_LAG_MS
  ATTACK_MAP_LOAD_MAX_TICK_ADVANCE_P95_MS
  ATTACK_MAP_LOAD_ATTACK_FEED_TIMEOUT_MS
  ATTACK_MAP_LOAD_ATTACK_FEED_POLL_INTERVAL_MS
  ATTACK_MAP_LOAD_VALIDATE_CHECKER_RUNS=true|false
  AD_PLATFORM_API_URL
  AD_PLATFORM_PUBLIC_BASE_URL
  ADMIN_API_TOKEN
EOF
}

validate_optional_integer() {
  local name="$1"
  local value="$2"

  if [[ -n "${value}" && ! "${value}" =~ ^[0-9]+$ ]]; then
    echo "Error: ${name} must be an integer when set." >&2
    exit 1
  fi
}

validate_optional_decimal() {
  local name="$1"
  local value="$2"

  if [[ -n "${value}" && ! "${value}" =~ ^[0-9]+([.][0-9]+)?$ ]]; then
    echo "Error: ${name} must be a positive number when set." >&2
    exit 1
  fi
}

if ! [[ "${TEAM_COUNT}" =~ ^[0-9]+$ ]] || (( TEAM_COUNT < 2 )); then
  echo "Error: TEAM_COUNT must be an integer >= 2." >&2
  usage >&2
  exit 1
fi

if ! [[ "${TEAM_START_INDEX}" =~ ^[0-9]+$ ]] || (( TEAM_START_INDEX < 1 )); then
  echo "Error: TEAM_START_INDEX must be a positive integer." >&2
  exit 1
fi

if ! [[ "${ATTACK_MAP_LOAD_ROUNDS}" =~ ^[0-9]+$ ]] || (( ATTACK_MAP_LOAD_ROUNDS < 1 )); then
  echo "Error: ATTACK_MAP_LOAD_ROUNDS must be a positive integer." >&2
  exit 1
fi

validate_optional_decimal "ATTACK_MAP_LOAD_MIN_SUBMISSIONS_PER_SECOND" "${ATTACK_MAP_LOAD_MIN_SUBMISSIONS_PER_SECOND}"
validate_optional_integer "ATTACK_MAP_LOAD_MAX_FLAG_FETCH_P95_MS" "${ATTACK_MAP_LOAD_MAX_FLAG_FETCH_P95_MS}"
validate_optional_integer "ATTACK_MAP_LOAD_MAX_SUBMISSION_P95_MS" "${ATTACK_MAP_LOAD_MAX_SUBMISSION_P95_MS}"
validate_optional_integer "ATTACK_MAP_LOAD_MAX_RECOMPUTE_MS" "${ATTACK_MAP_LOAD_MAX_RECOMPUTE_MS}"
validate_optional_integer "ATTACK_MAP_LOAD_MAX_ATTACK_FEED_LAG_MS" "${ATTACK_MAP_LOAD_MAX_ATTACK_FEED_LAG_MS}"
validate_optional_integer "ATTACK_MAP_LOAD_MAX_TICK_ADVANCE_P95_MS" "${ATTACK_MAP_LOAD_MAX_TICK_ADVANCE_P95_MS}"
validate_optional_integer "ATTACK_MAP_LOAD_ATTACK_FEED_TIMEOUT_MS" "${ATTACK_MAP_LOAD_ATTACK_FEED_TIMEOUT_MS}"
validate_optional_integer "ATTACK_MAP_LOAD_ATTACK_FEED_POLL_INTERVAL_MS" "${ATTACK_MAP_LOAD_ATTACK_FEED_POLL_INTERVAL_MS}"

if (( ATTACK_MAP_LOAD_ATTACK_FEED_TIMEOUT_MS < 1 )); then
  echo "Error: ATTACK_MAP_LOAD_ATTACK_FEED_TIMEOUT_MS must be >= 1." >&2
  exit 1
fi

if (( ATTACK_MAP_LOAD_ATTACK_FEED_POLL_INTERVAL_MS < 1 )); then
  echo "Error: ATTACK_MAP_LOAD_ATTACK_FEED_POLL_INTERVAL_MS must be >= 1." >&2
  exit 1
fi

now_ms() {
  date +%s%3N
}

milliseconds_from_seconds() {
  awk -v seconds="$1" 'BEGIN { printf "%.0f", seconds * 1000 }'
}

rate_per_second() {
  local count="$1"
  local duration_ms="$2"

  if (( duration_ms <= 0 )); then
    printf '0.00\n'
    return 0
  fi

  awk -v count="${count}" -v duration_ms="${duration_ms}" 'BEGIN { printf "%.2f", (count * 1000) / duration_ms }'
}

percentile_value() {
  local array_name="$1"
  local percentile="$2"
  local -n values_ref="${array_name}"
  local count rank
  local -a sorted_values=()

  count="${#values_ref[@]}"
  if (( count == 0 )); then
    printf '0\n'
    return 0
  fi

  mapfile -t sorted_values < <(printf '%s\n' "${values_ref[@]}" | sort -n)
  rank=$(( (count * percentile + 99) / 100 ))
  (( rank < 1 )) && rank=1
  (( rank > count )) && rank=count
  printf '%s\n' "${sorted_values[$((rank - 1))]}"
}

min_value() {
  local array_name="$1"
  local -n values_ref="${array_name}"
  local -a sorted_values=()

  if (( ${#values_ref[@]} == 0 )); then
    printf '0\n'
    return 0
  fi

  mapfile -t sorted_values < <(printf '%s\n' "${values_ref[@]}" | sort -n)
  printf '%s\n' "${sorted_values[0]}"
}

max_value() {
  local array_name="$1"
  local -n values_ref="${array_name}"
  local -a sorted_values=()
  local count

  count="${#values_ref[@]}"
  if (( count == 0 )); then
    printf '0\n'
    return 0
  fi

  mapfile -t sorted_values < <(printf '%s\n' "${values_ref[@]}" | sort -n)
  printf '%s\n' "${sorted_values[$((count - 1))]}"
}

assert_optional_max() {
  local label="$1"
  local actual="$2"
  local threshold="$3"
  local array_name="$4"
  local -n failures_ref="${array_name}"

  if [[ -z "${threshold}" ]]; then
    return 0
  fi

  if (( actual > threshold )); then
    failures_ref+=("${label} ${actual} exceeded ${threshold}")
  fi
}

assert_optional_min_decimal() {
  local label="$1"
  local actual="$2"
  local threshold="$3"
  local array_name="$4"
  local -n failures_ref="${array_name}"

  if [[ -z "${threshold}" ]]; then
    return 0
  fi

  if ! awk -v actual="${actual}" -v threshold="${threshold}" 'BEGIN { exit !(actual + 0 >= threshold + 0) }'; then
    failures_ref+=("${label} ${actual} was below ${threshold}")
  fi
}

curl_json() {
  local label="$1"
  shift

  local response_file
  response_file="$(mktemp)"

  local meta status time_total
  meta="$(curl -sS -o "${response_file}" -w '%{http_code} %{time_total}' "$@")"
  read -r status time_total <<< "${meta}"
  CURL_LAST_TIME_MS="$(milliseconds_from_seconds "${time_total}")"
  if [[ "${status}" -lt 200 || "${status}" -ge 300 ]]; then
    echo "${label} failed (status=${status}):" >&2
    cat "${response_file}" >&2
    rm -f "${response_file}"
    exit 1
  fi

  cat "${response_file}"
  rm -f "${response_file}"
}

participant_auth() {
  local email="$1"
  local password="$2"

  curl_json "authenticate ${email}" -X POST "${API_URL}/api/v2/authenticate" \
    -H 'Content-Type: application/json' \
    -d "$(jq -nc --arg email "${email}" --arg password "${password}" '{email:$email,password:$password}')" |
    jq -er '.token'
}

prefix_slug="$(slug_name "${TEAM_PREFIX}")"
unique_suffix="$(date +%s)"
scenario_started_ms="$(now_ms)"
end_index=$((TEAM_START_INDEX + TEAM_COUNT - 1))
pad_width="${#end_index}"
(( pad_width < 2 )) && pad_width=2

declare -a team_ids=()
declare -a team_names=()
declare -a player_emails=()
declare -a player_passwords=()
declare -a participant_tokens=()
declare -a tick_advance_latencies_ms=()
declare -a tick_checker_totals=()
declare -a tick_checker_successes=()
declare -a tick_checker_failures=()
declare -a tick_checker_skips=()
declare -a flag_fetch_latencies_ms=()
declare -a submission_latencies_ms=()
declare -a threshold_failures=()

echo "attack-map load setup: teams=${TEAM_COUNT} rounds=${ATTACK_MAP_LOAD_ROUNDS} api=${API_URL}"

if [[ "${ATTACK_MAP_LOAD_BUILD_IMAGES}" == "true" ]]; then
  echo "building sample challenge images"
  docker build -t "${BASELINE_IMAGE}" examples/sample-http-challenge/service >/dev/null
  docker build -t "${CHECKER_IMAGE}" examples/sample-http-challenge/checker >/dev/null
fi

if [[ "${ATTACK_MAP_LOAD_RESET}" == "true" ]]; then
  echo "resetting platform to an empty organizer-managed state"
  BOOTSTRAP_CLEAR_TEAMS=true ./scripts/bootstrap-clean-match.sh >/dev/null
fi

existing_challenges="$(curl_json "list challenges" "${API_URL}/api/v2/admin/challenges" -H "Authorization: Bearer ${ADMIN_TOKEN}")"
SERVICE_SUBNET_OCTET=""
used_subnets="$(printf '%s' "${existing_challenges}" | jq -r '.[].service_subnet_octet')"
for candidate in $(seq 50 254); do
  if ! printf '%s\n' "${used_subnets}" | grep -qx "${candidate}"; then
    SERVICE_SUBNET_OCTET="${candidate}"
    break
  fi
done

if [[ -z "${SERVICE_SUBNET_OCTET}" ]]; then
  echo "could not allocate a free service_subnet_octet" >&2
  exit 1
fi

SERVICE_PORT="$((30000 + SERVICE_SUBNET_OCTET))"
CHALLENGE_NAME="map-http-${unique_suffix}"

echo "creating ${TEAM_COUNT} teams and players"
for ((i = 0; i < TEAM_COUNT; i++)); do
  team_index=$((TEAM_START_INDEX + i))
  team_suffix="$(printf "%0${pad_width}d" "${team_index}")"
  team_name="${TEAM_PREFIX} ${team_suffix}"
  contact_email="${prefix_slug}-${team_suffix}@${TEAM_EMAIL_DOMAIN}"
  player_email="captain.${prefix_slug}.${team_suffix}.${unique_suffix}@${TEAM_EMAIL_DOMAIN}"
  player_password="${prefix_slug}-${team_suffix}-${unique_suffix}-secret"

  team_response="$(
    curl_json "create team ${team_name}" -X POST "${API_URL}/api/v2/admin/teams" \
      -H "Authorization: Bearer ${ADMIN_TOKEN}" \
      -H 'Content-Type: application/json' \
      -d "$(jq -nc --arg name "${team_name}" --arg email "${contact_email}" '{name:$name,contact_email:$email}')"
  )"
  team_id="$(printf '%s' "${team_response}" | jq -er '.id')"

  curl_json "create player ${player_email}" -X POST "${API_URL}/api/v2/admin/players" \
    -H "Authorization: Bearer ${ADMIN_TOKEN}" \
    -H 'Content-Type: application/json' \
    -d "$(jq -nc \
      --argjson team_id "${team_id}" \
      --arg display_name "${team_name} Captain" \
      --arg email "${player_email}" \
      --arg password "${player_password}" \
      --arg role "captain" \
      '{team_id:$team_id,display_name:$display_name,email:$email,password:$password,role:$role}')" >/dev/null

  team_ids+=("${team_id}")
  team_names+=("${team_name}")
  player_emails+=("${player_email}")
  player_passwords+=("${player_password}")
done

echo "creating and deploying challenge ${CHALLENGE_NAME}"
challenge_response="$(
  curl_json "create challenge" -X POST "${API_URL}/api/v2/admin/challenges" \
    -H "Authorization: Bearer ${ADMIN_TOKEN}" \
    -H 'Content-Type: application/json' \
    -d "$(jq -nc \
      --arg name "${CHALLENGE_NAME}" \
      --arg baseline_image "${BASELINE_IMAGE}" \
      --arg checker_image "${CHECKER_IMAGE}" \
      --argjson weight 1 \
      --argjson service_port "${SERVICE_PORT}" \
      --argjson service_subnet_octet "${SERVICE_SUBNET_OCTET}" \
      '{name:$name,baseline_image:$baseline_image,checker_image:$checker_image,weight:$weight,service_port:$service_port,service_subnet_octet:$service_subnet_octet}')"
)"
challenge_id="$(printf '%s' "${challenge_response}" | jq -er '.id')"

curl_json "validate challenge" -X POST "${API_URL}/api/v2/admin/challenges/${challenge_id}/validate" \
  -H "Authorization: Bearer ${ADMIN_TOKEN}" >/dev/null

curl_json "deploy challenge" -X POST "${API_URL}/api/v2/admin/challenges/${challenge_id}/deploy" \
  -H "Authorization: Bearer ${ADMIN_TOKEN}" >/dev/null

reconcile_response="$(
  curl_json "reconcile deployments" -X POST "${API_URL}/api/v2/admin/deployments/reconcile" \
    -H "Authorization: Bearer ${ADMIN_TOKEN}"
)"
printf '%s\n' "${reconcile_response}" | jq -c '{processed_jobs,processed_instances,completed_jobs}'

echo "authenticating participants and reading public service targets"
for ((i = 0; i < TEAM_COUNT; i++)); do
  participant_tokens+=("$(participant_auth "${player_emails[$i]}" "${player_passwords[$i]}")")
done

public_services="$(
  curl_json "list public services" "${API_URL}/api/v2/services" \
    -H "Authorization: Bearer ${participant_tokens[0]}"
)"

echo "starting match"
curl_json "start match" -X POST "${API_URL}/api/v2/admin/game/match/start" \
  -H "Authorization: Bearer ${ADMIN_TOKEN}" >/dev/null

total_submissions=0
last_submission_completed_ms="$(now_ms)"
attack_execution_duration_ms=0

for ((round = 1; round <= ATTACK_MAP_LOAD_ROUNDS; round++)); do
  shift_offset=$(( ((round - 1) % (TEAM_COUNT - 1)) + 1 ))

  tick_response="$(
    curl_json "advance tick round ${round}" -X POST "${API_URL}/api/v2/admin/game/ticks/advance" \
      -H "Authorization: Bearer ${ADMIN_TOKEN}"
  )"
  tick_advance_latencies_ms+=("${CURL_LAST_TIME_MS}")
  tick_id="$(printf '%s\n' "${tick_response}" | jq -er '.id')"
  tick_status="$(printf '%s\n' "${tick_response}" | jq -er '.status')"
  tick_total_checker_runs="$(printf '%s\n' "${tick_response}" | jq -er '.total_checker_runs // 0')"
  tick_successful_checker_runs="$(printf '%s\n' "${tick_response}" | jq -er '.successful_checker_runs // 0')"
  tick_failed_checker_runs="$(printf '%s\n' "${tick_response}" | jq -er '.failed_checker_runs // 0')"
  tick_skipped_checker_runs="$(printf '%s\n' "${tick_response}" | jq -er '.skipped_checker_runs // 0')"
  tick_checker_totals+=("${tick_total_checker_runs}")
  tick_checker_successes+=("${tick_successful_checker_runs}")
  tick_checker_failures+=("${tick_failed_checker_runs}")
  tick_checker_skips+=("${tick_skipped_checker_runs}")
  printf '%s\n' "${tick_response}" | jq -c '{id,status,total_checker_runs,successful_checker_runs,failed_checker_runs,skipped_checker_runs}'

  if [[ "${ATTACK_MAP_LOAD_VALIDATE_CHECKER_RUNS}" == "true" ]]; then
    checker_runs_response="$(
      curl_json "checker runs for tick ${tick_id}" \
        "${API_URL}/api/v2/admin/game/checker-runs?tick_id=${tick_id}&challenge_id=${challenge_id}&limit=${tick_total_checker_runs}" \
        -H "Authorization: Bearer ${ADMIN_TOKEN}"
    )"
    if ! printf '%s\n' "${checker_runs_response}" | jq -e \
      --argjson expected_total "${tick_total_checker_runs}" '
        .total_count == $expected_total and
        .has_next == false and
        (.items | length == $expected_total) and
        all(.items[]?; .status == "success")
      ' >/dev/null; then
      echo "checker runs validation failed for tick ${tick_id}" >&2
      printf '%s\n' "${checker_runs_response}" | jq -c '.items[]? | {tick_id,team_id,phase,status,message}' >&2
      exit 1
    fi
  fi

  echo "submitting ring attacks for tick=${tick_id} shift=${shift_offset}"
  round_submission_started_ms="$(now_ms)"
  for ((i = 0; i < TEAM_COUNT; i++)); do
    attacker_id="${team_ids[$i]}"
    attacker_name="${team_names[$i]}"
    victim_index=$(( (i + shift_offset) % TEAM_COUNT ))
    victim_id="${team_ids[$victim_index]}"
    victim_name="${team_names[$victim_index]}"
    attacker_token="${participant_tokens[$i]}"

    victim_endpoint="$(
      printf '%s\n' "${public_services}" |
        jq -er --arg challenge_id "${challenge_id}" --arg team_id "${victim_id}" '.[$challenge_id][$team_id][0]'
    )"

    stolen_flag="$(
      curl_json "fetch flag ${victim_name}" "http://${victim_endpoint}/leak" | jq -er '.flag'
    )"
    flag_fetch_latencies_ms+=("${CURL_LAST_TIME_MS}")

    submit_response="$(
      curl_json "submit stolen flag ${attacker_name} -> ${victim_name}" -X POST "${API_URL}/api/v2/submit" \
        -H "Authorization: Bearer ${attacker_token}" \
        -H 'Content-Type: application/json' \
        -d "$(jq -nc --arg flag "${stolen_flag}" '{flags:[$flag]}')"
    )"
    submission_latencies_ms+=("${CURL_LAST_TIME_MS}")

    if ! printf '%s\n' "${submit_response}" | jq -e '.results | length == 1 and .[0].verdict == "flag is correct."' >/dev/null; then
      echo "attack submission failed for ${attacker_name} -> ${victim_name}" >&2
      printf '%s\n' "${submit_response}" >&2
      exit 1
    fi

    total_submissions=$((total_submissions + 1))
    last_submission_completed_ms="$(now_ms)"
  done
  round_submission_completed_ms="$(now_ms)"
  attack_execution_duration_ms=$((attack_execution_duration_ms + round_submission_completed_ms - round_submission_started_ms))
done

submission_window_completed_ms="$(now_ms)"

scoreboard_response="$(
  curl_json "recompute scoring" -X POST "${API_URL}/api/v2/admin/game/scoring/recompute" \
    -H "Authorization: Bearer ${ADMIN_TOKEN}"
)"
scoreboard_recompute_ms="${CURL_LAST_TIME_MS}"
scoreboard_rows="$(printf '%s\n' "${scoreboard_response}" | jq 'length')"

attack_feed_poll_started_ms="$(now_ms)"
attack_feed_deadline_ms=$((attack_feed_poll_started_ms + ATTACK_MAP_LOAD_ATTACK_FEED_TIMEOUT_MS))
attack_feed_count="0"
attack_feed_response=""
while true; do
  attack_feed_response="$(
    curl_json "attack feed summary" "${API_URL}/api/v2/attacks?service=${CHALLENGE_NAME}&limit=200" \
      -H "Authorization: Bearer ${participant_tokens[0]}"
  )"
  attack_feed_count="$(printf '%s\n' "${attack_feed_response}" | jq -r '.total_count')"
  if [[ "${attack_feed_count}" == "${total_submissions}" ]]; then
    break
  fi

  current_poll_ms="$(now_ms)"
  if (( current_poll_ms >= attack_feed_deadline_ms )); then
    echo "attack feed did not converge to ${total_submissions} accepted submissions within ${ATTACK_MAP_LOAD_ATTACK_FEED_TIMEOUT_MS}ms" >&2
    printf '%s\n' "${attack_feed_response}" >&2
    exit 1
  fi

  sleep "$(awk -v interval_ms="${ATTACK_MAP_LOAD_ATTACK_FEED_POLL_INTERVAL_MS}" 'BEGIN { printf "%.3f", interval_ms / 1000 }')"
done

attack_feed_visible_ms="$(now_ms)"
attack_feed_lag_ms=$((attack_feed_visible_ms - last_submission_completed_ms))
scenario_completed_ms="$(now_ms)"

flag_fetch_p95_ms="$(percentile_value flag_fetch_latencies_ms 95)"
flag_fetch_min_ms="$(min_value flag_fetch_latencies_ms)"
flag_fetch_max_ms="$(max_value flag_fetch_latencies_ms)"
submission_p50_ms="$(percentile_value submission_latencies_ms 50)"
submission_p95_ms="$(percentile_value submission_latencies_ms 95)"
submission_max_ms="$(max_value submission_latencies_ms)"
submission_min_ms="$(min_value submission_latencies_ms)"
tick_advance_p95_ms="$(percentile_value tick_advance_latencies_ms 95)"
tick_advance_max_ms="$(max_value tick_advance_latencies_ms)"
checker_runs_total=0
checker_runs_successful=0
checker_runs_failed=0
checker_runs_skipped=0
for value in "${tick_checker_totals[@]}"; do
  checker_runs_total=$((checker_runs_total + value))
done
for value in "${tick_checker_successes[@]}"; do
  checker_runs_successful=$((checker_runs_successful + value))
done
for value in "${tick_checker_failures[@]}"; do
  checker_runs_failed=$((checker_runs_failed + value))
done
for value in "${tick_checker_skips[@]}"; do
  checker_runs_skipped=$((checker_runs_skipped + value))
done
submission_window_duration_ms="${attack_execution_duration_ms}"
scenario_duration_ms=$((scenario_completed_ms - scenario_started_ms))
submissions_per_second="$(rate_per_second "${total_submissions}" "${submission_window_duration_ms}")"

assert_optional_min_decimal \
  "submission throughput" \
  "${submissions_per_second}" \
  "${ATTACK_MAP_LOAD_MIN_SUBMISSIONS_PER_SECOND}" \
  threshold_failures
assert_optional_max \
  "flag fetch p95" \
  "${flag_fetch_p95_ms}" \
  "${ATTACK_MAP_LOAD_MAX_FLAG_FETCH_P95_MS}" \
  threshold_failures
assert_optional_max \
  "submission p95" \
  "${submission_p95_ms}" \
  "${ATTACK_MAP_LOAD_MAX_SUBMISSION_P95_MS}" \
  threshold_failures
assert_optional_max \
  "scoreboard recompute" \
  "${scoreboard_recompute_ms}" \
  "${ATTACK_MAP_LOAD_MAX_RECOMPUTE_MS}" \
  threshold_failures
assert_optional_max \
  "attack feed lag" \
  "${attack_feed_lag_ms}" \
  "${ATTACK_MAP_LOAD_MAX_ATTACK_FEED_LAG_MS}" \
  threshold_failures
assert_optional_max \
  "tick advance p95" \
  "${tick_advance_p95_ms}" \
  "${ATTACK_MAP_LOAD_MAX_TICK_ADVANCE_P95_MS}" \
  threshold_failures

if [[ "${ATTACK_MAP_LOAD_VALIDATE_CHECKER_RUNS}" == "true" ]]; then
  if (( checker_runs_failed > 0 )); then
    threshold_failures+=("checker failures ${checker_runs_failed} exceeded 0")
  fi
  if (( checker_runs_skipped > 0 )); then
    threshold_failures+=("checker skips ${checker_runs_skipped} exceeded 0")
  fi
  if (( checker_runs_successful != checker_runs_total )); then
    threshold_failures+=("checker success count ${checker_runs_successful} did not match total ${checker_runs_total}")
  fi
fi

validation_status="passed"
if (( ${#threshold_failures[@]} > 0 )); then
  validation_status="failed"
fi

threshold_failures_json='[]'
if (( ${#threshold_failures[@]} > 0 )); then
  threshold_failures_json="$(printf '%s\n' "${threshold_failures[@]}" | jq -R . | jq -s .)"
fi

summary_json="$(
  jq -nc \
    --arg challenge_id "${challenge_id}" \
    --arg validation_status "${validation_status}" \
    --argjson teams "${TEAM_COUNT}" \
    --argjson rounds "${ATTACK_MAP_LOAD_ROUNDS}" \
    --argjson total_submissions "${total_submissions}" \
    --argjson attack_feed_total "${attack_feed_count}" \
    --argjson scoreboard_rows "${scoreboard_rows}" \
    --argjson scenario_duration_ms "${scenario_duration_ms}" \
    --argjson submission_window_duration_ms "${submission_window_duration_ms}" \
    --arg submissions_per_second "${submissions_per_second}" \
    --argjson flag_fetch_min_ms "${flag_fetch_min_ms}" \
    --argjson flag_fetch_p95_ms "${flag_fetch_p95_ms}" \
    --argjson flag_fetch_max_ms "${flag_fetch_max_ms}" \
    --argjson submission_min_ms "${submission_min_ms}" \
    --argjson submission_p50_ms "${submission_p50_ms}" \
    --argjson submission_p95_ms "${submission_p95_ms}" \
    --argjson submission_max_ms "${submission_max_ms}" \
    --argjson tick_advance_p95_ms "${tick_advance_p95_ms}" \
    --argjson tick_advance_max_ms "${tick_advance_max_ms}" \
    --argjson checker_runs_total "${checker_runs_total}" \
    --argjson checker_runs_successful "${checker_runs_successful}" \
    --argjson checker_runs_failed "${checker_runs_failed}" \
    --argjson checker_runs_skipped "${checker_runs_skipped}" \
    --argjson scoreboard_recompute_ms "${scoreboard_recompute_ms}" \
    --argjson attack_feed_lag_ms "${attack_feed_lag_ms}" \
    --argjson attack_feed_timeout_ms "${ATTACK_MAP_LOAD_ATTACK_FEED_TIMEOUT_MS}" \
    --argjson attack_feed_poll_interval_ms "${ATTACK_MAP_LOAD_ATTACK_FEED_POLL_INTERVAL_MS}" \
    --arg min_submissions_per_second "${ATTACK_MAP_LOAD_MIN_SUBMISSIONS_PER_SECOND}" \
    --arg max_flag_fetch_p95_ms "${ATTACK_MAP_LOAD_MAX_FLAG_FETCH_P95_MS}" \
    --arg max_submission_p95_ms "${ATTACK_MAP_LOAD_MAX_SUBMISSION_P95_MS}" \
    --arg max_recompute_ms "${ATTACK_MAP_LOAD_MAX_RECOMPUTE_MS}" \
    --arg max_attack_feed_lag_ms "${ATTACK_MAP_LOAD_MAX_ATTACK_FEED_LAG_MS}" \
    --arg max_tick_advance_p95_ms "${ATTACK_MAP_LOAD_MAX_TICK_ADVANCE_P95_MS}" \
    --arg validate_checker_runs "${ATTACK_MAP_LOAD_VALIDATE_CHECKER_RUNS}" \
    --argjson threshold_failures "${threshold_failures_json}" \
    '{
      challenge_id: $challenge_id,
      validation_status: $validation_status,
      teams: $teams,
      rounds: $rounds,
      total_submissions: $total_submissions,
      attack_feed_total: $attack_feed_total,
      scoreboard_rows: $scoreboard_rows,
      scenario_duration_ms: $scenario_duration_ms,
      submission_window_duration_ms: $submission_window_duration_ms,
      submissions_per_second: ($submissions_per_second | tonumber),
      checker_runs: {
        total: $checker_runs_total,
        successful: $checker_runs_successful,
        failed: $checker_runs_failed,
        skipped: $checker_runs_skipped
      },
      latencies_ms: {
        flag_fetch: {
          min: $flag_fetch_min_ms,
          p95: $flag_fetch_p95_ms,
          max: $flag_fetch_max_ms
        },
        submission: {
          min: $submission_min_ms,
          p50: $submission_p50_ms,
          p95: $submission_p95_ms,
          max: $submission_max_ms
        },
        tick_advance: {
          p95: $tick_advance_p95_ms,
          max: $tick_advance_max_ms
        },
        scoreboard_recompute: $scoreboard_recompute_ms,
        attack_feed_visibility: $attack_feed_lag_ms
      },
      thresholds: {
        min_submissions_per_second: (if $min_submissions_per_second == "" then null else ($min_submissions_per_second | tonumber) end),
        max_flag_fetch_p95_ms: (if $max_flag_fetch_p95_ms == "" then null else ($max_flag_fetch_p95_ms | tonumber) end),
        max_submission_p95_ms: (if $max_submission_p95_ms == "" then null else ($max_submission_p95_ms | tonumber) end),
        max_recompute_ms: (if $max_recompute_ms == "" then null else ($max_recompute_ms | tonumber) end),
        max_attack_feed_lag_ms: (if $max_attack_feed_lag_ms == "" then null else ($max_attack_feed_lag_ms | tonumber) end),
        max_tick_advance_p95_ms: (if $max_tick_advance_p95_ms == "" then null else ($max_tick_advance_p95_ms | tonumber) end),
        attack_feed_timeout_ms: $attack_feed_timeout_ms,
        attack_feed_poll_interval_ms: $attack_feed_poll_interval_ms,
        validate_checker_runs: ($validate_checker_runs == "true")
      },
      threshold_failures: $threshold_failures
    }'
)"

printf 'attack-map load ready: challenge_id=%s teams=%s rounds=%s accepted_attacks=%s scoreboard_rows=%s\n' \
  "${challenge_id}" "${TEAM_COUNT}" "${ATTACK_MAP_LOAD_ROUNDS}" "${attack_feed_count}" "${scoreboard_rows}"
printf 'attack-map metrics: submissions=%s duration_ms=%s throughput=%s/s submission_p95_ms=%s recompute_ms=%s attack_feed_lag_ms=%s validation=%s\n' \
  "${total_submissions}" \
  "${submission_window_duration_ms}" \
  "${submissions_per_second}" \
  "${submission_p95_ms}" \
  "${scoreboard_recompute_ms}" \
  "${attack_feed_lag_ms}" \
  "${validation_status}"
printf 'attack-map checker summary: total=%s successful=%s failed=%s skipped=%s tick_advance_p95_ms=%s\n' \
  "${checker_runs_total}" \
  "${checker_runs_successful}" \
  "${checker_runs_failed}" \
  "${checker_runs_skipped}" \
  "${tick_advance_p95_ms}"
printf '%s\n' "${summary_json}" | jq .
echo "tip: increase organizer attack page size to 96 to see more teams in one slice."

if [[ -n "${ATTACK_MAP_LOAD_ARTIFACT_FILE}" ]]; then
  mkdir -p "$(dirname "${ATTACK_MAP_LOAD_ARTIFACT_FILE}")"
  {
    echo "SIM_CHALLENGE_ID=${challenge_id}"
    echo "SIM_TEAM_COUNT=${TEAM_COUNT}"
    echo "SIM_ATTACK_ROUNDS=${ATTACK_MAP_LOAD_ROUNDS}"
    echo "SIM_TEAM_PREFIX=${TEAM_PREFIX}"
    echo "SIM_TOTAL_SUBMISSIONS=${total_submissions}"
    echo "SIM_ATTACK_FEED_TOTAL=${attack_feed_count}"
    echo "SIM_VALIDATION_STATUS=${validation_status}"
    echo "SIM_THRESHOLD_FAILURE_COUNT=${#threshold_failures[@]}"
    echo "SIM_SCENARIO_DURATION_MS=${scenario_duration_ms}"
    echo "SIM_SUBMISSION_WINDOW_DURATION_MS=${submission_window_duration_ms}"
    echo "SIM_SUBMISSIONS_PER_SECOND=${submissions_per_second}"
    echo "SIM_FLAG_FETCH_MIN_MS=${flag_fetch_min_ms}"
    echo "SIM_FLAG_FETCH_P95_MS=${flag_fetch_p95_ms}"
    echo "SIM_FLAG_FETCH_MAX_MS=${flag_fetch_max_ms}"
    echo "SIM_SUBMISSION_MIN_MS=${submission_min_ms}"
    echo "SIM_SUBMISSION_P50_MS=${submission_p50_ms}"
    echo "SIM_SUBMISSION_P95_MS=${submission_p95_ms}"
    echo "SIM_SUBMISSION_MAX_MS=${submission_max_ms}"
    echo "SIM_TICK_ADVANCE_P95_MS=${tick_advance_p95_ms}"
    echo "SIM_TICK_ADVANCE_MAX_MS=${tick_advance_max_ms}"
    echo "SIM_CHECKER_RUNS_TOTAL=${checker_runs_total}"
    echo "SIM_CHECKER_RUNS_SUCCESSFUL=${checker_runs_successful}"
    echo "SIM_CHECKER_RUNS_FAILED=${checker_runs_failed}"
    echo "SIM_CHECKER_RUNS_SKIPPED=${checker_runs_skipped}"
    echo "SIM_SCOREBOARD_RECOMPUTE_MS=${scoreboard_recompute_ms}"
    echo "SIM_ATTACK_FEED_LAG_MS=${attack_feed_lag_ms}"
    for ((i = 0; i < TEAM_COUNT; i++)); do
      idx=$((i + 1))
      printf 'SIM_TEAM_%02d_ID=%s\n' "${idx}" "${team_ids[$i]}"
      printf 'SIM_TEAM_%02d_NAME=%q\n' "${idx}" "${team_names[$i]}"
      printf 'SIM_TEAM_%02d_EMAIL=%q\n' "${idx}" "${player_emails[$i]}"
      printf 'SIM_TEAM_%02d_PASSWORD=%q\n' "${idx}" "${player_passwords[$i]}"
    done
  } > "${ATTACK_MAP_LOAD_ARTIFACT_FILE}"
fi

if [[ -n "${ATTACK_MAP_LOAD_REPORT_FILE}" ]]; then
  mkdir -p "$(dirname "${ATTACK_MAP_LOAD_REPORT_FILE}")"
  printf '%s\n' "${summary_json}" > "${ATTACK_MAP_LOAD_REPORT_FILE}"
fi

if (( ${#threshold_failures[@]} > 0 )); then
  echo "attack-map load validation failed:" >&2
  printf '  - %s\n' "${threshold_failures[@]}" >&2
  exit 1
fi
