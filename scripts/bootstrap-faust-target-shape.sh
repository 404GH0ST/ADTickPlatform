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
ADMIN_TOKEN="$(resolve_admin_api_token "${ROOT_DIR}/.runtime/backend-stack.env")"

if [[ "${API_URL}" == *"api-gateway"* ]] && [[ -n "${PUBLIC_URL}" ]]; then
  API_URL="${PUBLIC_URL}"
fi

TARGET_TEAM_COUNT="${TARGET_TEAM_COUNT:-10}"
TARGET_SERVICE_COUNT="${TARGET_SERVICE_COUNT:-5}"
TARGET_TICK_INTERVAL_SECONDS="${TARGET_TICK_INTERVAL_SECONDS:-300}"
TARGET_MATCH_DURATION_HOURS="${TARGET_MATCH_DURATION_HOURS:-24}"
TARGET_TEAM_PREFIX="${TARGET_TEAM_PREFIX:-Team}"
TARGET_TEAM_EMAIL_DOMAIN="${TARGET_TEAM_EMAIL_DOMAIN:-teams.local}"
TARGET_TEAM_START_INDEX="${TARGET_TEAM_START_INDEX:-1}"
TARGET_CHALLENGE_PREFIX="${TARGET_CHALLENGE_PREFIX:-faust-service}"
TARGET_RESET_STATE="${TARGET_RESET_STATE:-true}"
TARGET_START_MATCH="${TARGET_START_MATCH:-true}"
TARGET_START_SCHEDULER="${TARGET_START_SCHEDULER:-true}"
TARGET_BASELINE_IMAGE="${TARGET_BASELINE_IMAGE:-adplatform/sample-lfi:baseline}"
TARGET_CHECKER_IMAGE="${TARGET_CHECKER_IMAGE:-adplatform/sample-lfi-checker:latest}"
TARGET_BASELINE_IMAGES="${TARGET_BASELINE_IMAGES:-}"
TARGET_CHECKER_IMAGES="${TARGET_CHECKER_IMAGES:-}"
TARGET_SERVICE_NAMES="${TARGET_SERVICE_NAMES:-}"
TARGET_ARTIFACT_FILE="${TARGET_ARTIFACT_FILE:-.runtime/faust-target-shape.json}"

usage() {
  cat <<'EOF'
Usage: scripts/bootstrap-faust-target-shape.sh

Creates a contest layout shaped like a Faust-style event:
  - teams
  - published/deployed services
  - scheduler interval
  - 24h match window

Environment overrides:
  TARGET_TEAM_COUNT=10
  TARGET_SERVICE_COUNT=5
  TARGET_TICK_INTERVAL_SECONDS=300
  TARGET_MATCH_DURATION_HOURS=24
  TARGET_TEAM_PREFIX=Team
  TARGET_TEAM_EMAIL_DOMAIN=teams.local
  TARGET_TEAM_START_INDEX=1
  TARGET_CHALLENGE_PREFIX=faust-service
  TARGET_RESET_STATE=true|false
  TARGET_START_MATCH=true|false
  TARGET_START_SCHEDULER=true|false
  TARGET_BASELINE_IMAGE=adplatform/sample-lfi:baseline
  TARGET_CHECKER_IMAGE=adplatform/sample-lfi-checker:latest
  TARGET_BASELINE_IMAGES=image1,image2,...
  TARGET_CHECKER_IMAGES=image1,image2,...
  TARGET_SERVICE_NAMES=name-1,name-2,...
  TARGET_ARTIFACT_FILE=.runtime/faust-target-shape.json
  AD_PLATFORM_API_URL
  AD_PLATFORM_PUBLIC_BASE_URL
  ADMIN_API_TOKEN
EOF
}

if [[ "${1:-}" == "--help" ]]; then
  usage
  exit 0
fi

validate_positive_integer() {
  local label="$1"
  local value="$2"
  if ! [[ "${value}" =~ ^[0-9]+$ ]] || (( value < 1 )); then
    echo "Error: ${label} must be a positive integer." >&2
    exit 1
  fi
}

validate_positive_integer "TARGET_TEAM_COUNT" "${TARGET_TEAM_COUNT}"
validate_positive_integer "TARGET_SERVICE_COUNT" "${TARGET_SERVICE_COUNT}"
validate_positive_integer "TARGET_TICK_INTERVAL_SECONDS" "${TARGET_TICK_INTERVAL_SECONDS}"
validate_positive_integer "TARGET_MATCH_DURATION_HOURS" "${TARGET_MATCH_DURATION_HOURS}"
validate_positive_integer "TARGET_TEAM_START_INDEX" "${TARGET_TEAM_START_INDEX}"

mkdir -p "$(dirname "${TARGET_ARTIFACT_FILE}")"

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

curl_json_retry() {
  local label="$1"
  local attempts="$2"
  shift 2

  local response_file status attempt
  response_file="$(mktemp)"
  for ((attempt = 1; attempt <= attempts; attempt++)); do
    status="$(curl -sS -o "${response_file}" -w '%{http_code}' "$@")"
    if [[ "${status}" -ge 200 && "${status}" -lt 300 ]]; then
      cat "${response_file}"
      rm -f "${response_file}"
      return 0
    fi

    if (( attempt == attempts )); then
      echo "${label} failed (status=${status}):" >&2
      cat "${response_file}" >&2
      rm -f "${response_file}"
      exit 1
    fi

    sleep 2
  done
}

wait_for_operator_surface() {
  local base_url="$1"

  if wait_for_http "${base_url}/healthz" 60 "edge healthz"; then
    wait_for_http "${base_url}/api/v2/challenges" 60 "public challenges"
    return 0
  fi

  wait_for_http "${base_url}/api/v2/challenges" 60 "public challenges"
}

split_csv_to_array() {
  local value="$1"
  local array_name="$2"
  local -n ref="${array_name}"
  ref=()
  [[ -n "${value}" ]] || return 0
  IFS=',' read -r -a ref <<< "${value}"
  local i
  for i in "${!ref[@]}"; do
    ref[$i]="$(printf '%s' "${ref[$i]}" | sed 's/^[[:space:]]*//; s/[[:space:]]*$//')"
  done
}

resolve_series_value() {
  local index="$1"
  local fallback="$2"
  shift 2
  local values=("$@")
  if (( ${#values[@]} > 0 )); then
    printf '%s\n' "${values[$index]}"
    return 0
  fi
  printf '%s\n' "${fallback}"
}

split_csv_to_array "${TARGET_BASELINE_IMAGES}" baseline_images
split_csv_to_array "${TARGET_CHECKER_IMAGES}" checker_images
split_csv_to_array "${TARGET_SERVICE_NAMES}" service_names

if (( ${#baseline_images[@]} > 0 && ${#baseline_images[@]} != TARGET_SERVICE_COUNT )); then
  echo "Error: TARGET_BASELINE_IMAGES must have ${TARGET_SERVICE_COUNT} comma-separated values." >&2
  exit 1
fi
if (( ${#checker_images[@]} > 0 && ${#checker_images[@]} != TARGET_SERVICE_COUNT )); then
  echo "Error: TARGET_CHECKER_IMAGES must have ${TARGET_SERVICE_COUNT} comma-separated values." >&2
  exit 1
fi
if (( ${#service_names[@]} > 0 && ${#service_names[@]} != TARGET_SERVICE_COUNT )); then
  echo "Error: TARGET_SERVICE_NAMES must have ${TARGET_SERVICE_COUNT} comma-separated values." >&2
  exit 1
fi

wait_for_operator_surface "${API_URL}"

if [[ "${TARGET_RESET_STATE}" == "true" ]]; then
  echo "resetting platform state"
  BOOTSTRAP_CLEAR_TEAMS=true "${ROOT_DIR}/scripts/bootstrap-clean-match.sh" >/dev/null
fi

echo "creating ${TARGET_TEAM_COUNT} team(s)"
prefix_slug="$(slug_name "${TARGET_TEAM_PREFIX}")"
end_index=$((TARGET_TEAM_START_INDEX + TARGET_TEAM_COUNT - 1))
pad_width="${#end_index}"
(( pad_width < 2 )) && pad_width=2

declare -a team_records=()
for ((i = 0; i < TARGET_TEAM_COUNT; i++)); do
  team_index=$((TARGET_TEAM_START_INDEX + i))
  team_suffix="$(printf "%0${pad_width}d" "${team_index}")"
  team_name="${TARGET_TEAM_PREFIX} ${team_suffix}"
  contact_email="${prefix_slug}-${team_suffix}@${TARGET_TEAM_EMAIL_DOMAIN}"
  player_email="captain.${prefix_slug}.${team_suffix}@${TARGET_TEAM_EMAIL_DOMAIN}"
  player_password="${prefix_slug}-${team_suffix}-secret"

  team_response="$(
    curl_json "create team ${team_name}" \
      -X POST "${API_URL}/api/v2/admin/teams" \
      -H "Authorization: Bearer ${ADMIN_TOKEN}" \
      -H 'Content-Type: application/json' \
      -d "$(jq -nc --arg name "${team_name}" --arg email "${contact_email}" '{name:$name,contact_email:$email}')"
  )"
  team_id="$(printf '%s' "${team_response}" | jq -er '.id')"

  curl_json "create player ${player_email}" \
    -X POST "${API_URL}/api/v2/admin/players" \
    -H "Authorization: Bearer ${ADMIN_TOKEN}" \
    -H 'Content-Type: application/json' \
    -d "$(jq -nc \
      --argjson team_id "${team_id}" \
      --arg display_name "${team_name} Captain" \
      --arg email "${player_email}" \
      --arg password "${player_password}" \
      --arg role "captain" \
      '{team_id:$team_id,display_name:$display_name,email:$email,password:$password,role:$role}')"

  team_records+=(
    "$(jq -nc \
      --argjson id "${team_id}" \
      --arg name "${team_name}" \
      --arg contact_email "${contact_email}" \
      --arg player_email "${player_email}" \
      --arg player_password "${player_password}" \
      '{id:$id,name:$name,contact_email:$contact_email,player_email:$player_email,player_password:$player_password}')"
  )
done

existing_challenges="$(
  curl_json "list challenges" \
    "${API_URL}/api/v2/admin/challenges" \
    -H "Authorization: Bearer ${ADMIN_TOKEN}"
)"
used_subnets="$(printf '%s' "${existing_challenges}" | jq -r '.[].service_subnet_octet // empty')"
ALLOCATED_SUBNET_OCTET=""

allocate_subnet_octet() {
  local candidate
  for candidate in $(seq 50 254); do
    if ! printf '%s\n' "${used_subnets}" | grep -qx "${candidate}"; then
      ALLOCATED_SUBNET_OCTET="${candidate}"
      used_subnets="${used_subnets}"$'\n'"${candidate}"
      return 0
    fi
  done
  return 1
}

echo "creating and deploying ${TARGET_SERVICE_COUNT} service(s)"
declare -a challenge_records=()
for ((i = 0; i < TARGET_SERVICE_COUNT; i++)); do
  challenge_suffix="$(printf "%02d" "$((i + 1))")"
  challenge_name="$(resolve_series_value "${i}" "${TARGET_CHALLENGE_PREFIX}-${challenge_suffix}" "${service_names[@]}")"
  baseline_image="$(resolve_series_value "${i}" "${TARGET_BASELINE_IMAGE}" "${baseline_images[@]}")"
  checker_image="$(resolve_series_value "${i}" "${TARGET_CHECKER_IMAGE}" "${checker_images[@]}")"
  allocate_subnet_octet
  subnet_octet="${ALLOCATED_SUBNET_OCTET}"
  if [[ -z "${subnet_octet}" ]]; then
    echo "Error: unable to allocate a free service_subnet_octet." >&2
    exit 1
  fi
  service_port="$((30000 + subnet_octet))"

  challenge_response="$(
    curl_json "create challenge ${challenge_name}" \
      -X POST "${API_URL}/api/v2/admin/challenges" \
      -H "Authorization: Bearer ${ADMIN_TOKEN}" \
      -H 'Content-Type: application/json' \
      -d "$(jq -nc \
        --arg name "${challenge_name}" \
        --arg baseline_image "${baseline_image}" \
        --arg checker_image "${checker_image}" \
        --argjson service_port "${service_port}" \
        --argjson service_subnet_octet "${subnet_octet}" \
        '{name:$name,baseline_image:$baseline_image,checker_image:$checker_image,service_port:$service_port,service_subnet_octet:$service_subnet_octet}')"
  )"
  challenge_id="$(printf '%s' "${challenge_response}" | jq -er '.id')"

  curl_json "validate challenge ${challenge_name}" \
    -X POST "${API_URL}/api/v2/admin/challenges/${challenge_id}/validate" \
    -H "Authorization: Bearer ${ADMIN_TOKEN}" >/dev/null

  curl_json "deploy challenge ${challenge_name}" \
    -X POST "${API_URL}/api/v2/admin/challenges/${challenge_id}/deploy" \
    -H "Authorization: Bearer ${ADMIN_TOKEN}" >/dev/null

  challenge_records+=(
    "$(jq -nc \
      --argjson id "${challenge_id}" \
      --arg name "${challenge_name}" \
      --arg baseline_image "${baseline_image}" \
      --arg checker_image "${checker_image}" \
      --argjson service_port "${service_port}" \
      --argjson service_subnet_octet "${subnet_octet}" \
      '{id:$id,name:$name,baseline_image:$baseline_image,checker_image:$checker_image,service_port:$service_port,service_subnet_octet:$service_subnet_octet}')"
  )
done

echo "reconciling deployments"
reconcile_response="$(
  curl_json_retry "reconcile deployments" 5 \
    -X POST "${API_URL}/api/v2/admin/deployments/reconcile" \
    -H "Authorization: Bearer ${ADMIN_TOKEN}"
)"

deployment_rows="$(
  curl_json "list deployments" \
    "${API_URL}/api/v2/admin/deployments" \
    -H "Authorization: Bearer ${ADMIN_TOKEN}"
)"

expected_ready="${TARGET_TEAM_COUNT}"
for challenge_json in "${challenge_records[@]}"; do
  challenge_id="$(printf '%s' "${challenge_json}" | jq -r '.id')"
  if ! printf '%s\n' "${deployment_rows}" | jq -e \
    --argjson challenge_id "${challenge_id}" \
    --argjson expected_ready "${expected_ready}" \
    'map(select(.challenge_id == $challenge_id and .status == "completed" and .ready_team_count == $expected_ready)) | length > 0' >/dev/null; then
    echo "Deployment for challenge ${challenge_id} did not reach completed/${expected_ready} ready teams." >&2
    printf '%s\n' "${deployment_rows}" >&2
    exit 1
  fi
done

echo "configuring scheduler interval=${TARGET_TICK_INTERVAL_SECONDS}s"
scheduler_response="$(
  curl_json "update scheduler interval" \
    -X PUT "${API_URL}/api/v2/admin/game/scheduler/interval" \
    -H "Authorization: Bearer ${ADMIN_TOKEN}" \
    -H 'Content-Type: application/json' \
    -d "$(jq -nc --argjson interval_seconds "${TARGET_TICK_INTERVAL_SECONDS}" '{interval_seconds:$interval_seconds}')"
)"

match_start_at="$(date -u +"%Y-%m-%dT%H:%M:%SZ")"
match_end_at="$(date -u -d "+${TARGET_MATCH_DURATION_HOURS} hours" +"%Y-%m-%dT%H:%M:%SZ")"
echo "configuring match window ${match_start_at} -> ${match_end_at}"
match_schedule_response="$(
  curl_json "update match schedule" \
    -X PUT "${API_URL}/api/v2/admin/game/match/schedule" \
    -H "Authorization: Bearer ${ADMIN_TOKEN}" \
    -H 'Content-Type: application/json' \
    -d "$(jq -nc --arg scheduled_start_at "${match_start_at}" --arg scheduled_end_at "${match_end_at}" '{scheduled_start_at:$scheduled_start_at,scheduled_end_at:$scheduled_end_at}')"
)"

match_response='null'
scheduler_start_response='null'
if [[ "${TARGET_START_MATCH}" == "true" ]]; then
  echo "starting match"
  match_response="$(
    curl_json "start match" \
      -X POST "${API_URL}/api/v2/admin/game/match/start" \
      -H "Authorization: Bearer ${ADMIN_TOKEN}"
  )"
fi

if [[ "${TARGET_START_SCHEDULER}" == "true" ]]; then
  echo "starting scheduler"
  scheduler_start_response="$(
    curl_json "start scheduler" \
      -X POST "${API_URL}/api/v2/admin/game/scheduler/start" \
      -H "Authorization: Bearer ${ADMIN_TOKEN}"
  )"
fi

game_status="$(
  curl_json "game status" \
    "${API_URL}/api/v2/admin/game/status" \
    -H "Authorization: Bearer ${ADMIN_TOKEN}"
)"

jq -n \
  --arg api_url "${API_URL}" \
  --arg generated_at "$(date -u +"%Y-%m-%dT%H:%M:%SZ")" \
  --argjson target_team_count "${TARGET_TEAM_COUNT}" \
  --argjson target_service_count "${TARGET_SERVICE_COUNT}" \
  --argjson target_tick_interval_seconds "${TARGET_TICK_INTERVAL_SECONDS}" \
  --argjson target_match_duration_hours "${TARGET_MATCH_DURATION_HOURS}" \
  --argjson reconcile "$(printf '%s' "${reconcile_response}")" \
  --argjson scheduler "$(printf '%s' "${scheduler_response}")" \
  --argjson match_schedule "$(printf '%s' "${match_schedule_response}")" \
  --argjson match_start "${match_response}" \
  --argjson scheduler_start "${scheduler_start_response}" \
  --argjson game_status "$(printf '%s' "${game_status}")" \
  --argjson teams "$(printf '%s\n' "${team_records[@]}" | jq -s '.')" \
  --argjson challenges "$(printf '%s\n' "${challenge_records[@]}" | jq -s '.')" \
  '{
    generated_at: $generated_at,
    api_url: $api_url,
    target_shape: {
      team_count: $target_team_count,
      service_count: $target_service_count,
      tick_interval_seconds: $target_tick_interval_seconds,
      match_duration_hours: $target_match_duration_hours
    },
    teams: $teams,
    challenges: $challenges,
    deployment_reconcile: {
      processed_jobs: $reconcile.processed_jobs,
      processed_instances: $reconcile.processed_instances,
      completed_jobs: $reconcile.completed_jobs
    },
    scheduler: $scheduler,
    match_schedule: $match_schedule,
    match_start: $match_start,
    scheduler_start: $scheduler_start,
    game_status: $game_status
  }' > "${TARGET_ARTIFACT_FILE}"

echo "faust target-shape bootstrap completed"
printf '  %s\n' \
  "teams: ${TARGET_TEAM_COUNT}" \
  "services: ${TARGET_SERVICE_COUNT}" \
  "scheduler interval: ${TARGET_TICK_INTERVAL_SECONDS}s" \
  "match window: ${match_start_at} -> ${match_end_at}" \
  "artifact: ${TARGET_ARTIFACT_FILE}"
