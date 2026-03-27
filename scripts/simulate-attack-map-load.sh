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
  AD_PLATFORM_API_URL
  AD_PLATFORM_PUBLIC_BASE_URL
  ADMIN_API_TOKEN
EOF
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

participant_auth() {
  local email="$1"
  local password="$2"

  curl_json "authenticate ${email}" -X POST "${API_URL}/api/v2/authenticate" \
    -H 'Content-Type: application/json' \
    -d "$(jq -nc --arg email "${email}" --arg password "${password}" '{email:$email,password:$password}')" |
    jq -er '.data'
}

prefix_slug="$(slug_name "${TEAM_PREFIX}")"
unique_suffix="$(date +%s)"
end_index=$((TEAM_START_INDEX + TEAM_COUNT - 1))
pad_width="${#end_index}"
(( pad_width < 2 )) && pad_width=2

declare -a team_ids=()
declare -a team_names=()
declare -a player_emails=()
declare -a player_passwords=()
declare -a participant_tokens=()

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
used_subnets="$(printf '%s' "${existing_challenges}" | jq -r '.data[].service_subnet_octet')"
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
  team_id="$(printf '%s' "${team_response}" | jq -er '.data.id')"

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
challenge_id="$(printf '%s' "${challenge_response}" | jq -er '.data.id')"

curl_json "validate challenge" -X POST "${API_URL}/api/v2/admin/challenges/${challenge_id}/validate" \
  -H "Authorization: Bearer ${ADMIN_TOKEN}" >/dev/null

curl_json "deploy challenge" -X POST "${API_URL}/api/v2/admin/challenges/${challenge_id}/deploy" \
  -H "Authorization: Bearer ${ADMIN_TOKEN}" >/dev/null

reconcile_response="$(
  curl_json "reconcile deployments" -X POST "${API_URL}/api/v2/admin/deployments/reconcile" \
    -H "Authorization: Bearer ${ADMIN_TOKEN}"
)"
printf '%s\n' "${reconcile_response}" | jq -c '.data | {processed_jobs,processed_instances,completed_jobs}'

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

for ((round = 1; round <= ATTACK_MAP_LOAD_ROUNDS; round++)); do
  shift_offset=$(( ((round - 1) % (TEAM_COUNT - 1)) + 1 ))

  tick_response="$(
    curl_json "advance tick round ${round}" -X POST "${API_URL}/api/v2/admin/game/ticks/advance" \
      -H "Authorization: Bearer ${ADMIN_TOKEN}"
  )"
  tick_id="$(printf '%s\n' "${tick_response}" | jq -er '.data.id')"
  printf '%s\n' "${tick_response}" | jq -c '.data | {id,status,total_checker_runs}'

  echo "submitting ring attacks for tick=${tick_id} shift=${shift_offset}"
  for ((i = 0; i < TEAM_COUNT; i++)); do
    attacker_id="${team_ids[$i]}"
    attacker_name="${team_names[$i]}"
    victim_index=$(( (i + shift_offset) % TEAM_COUNT ))
    victim_id="${team_ids[$victim_index]}"
    victim_name="${team_names[$victim_index]}"
    attacker_token="${participant_tokens[$i]}"

    victim_endpoint="$(
      printf '%s\n' "${public_services}" |
        jq -er --arg challenge_id "${challenge_id}" --arg team_id "${victim_id}" '.data[$challenge_id][$team_id][0]'
    )"

    stolen_flag="$(
      curl -fsS "http://${victim_endpoint}/leak" | jq -er '.flag'
    )"

    submit_response="$(
      curl_json "submit stolen flag ${attacker_name} -> ${victim_name}" -X POST "${API_URL}/api/v2/submit" \
        -H "Authorization: Bearer ${attacker_token}" \
        -H 'Content-Type: application/json' \
        -d "$(jq -nc --arg flag "${stolen_flag}" '{flags:[$flag]}')"
    )"

    if ! printf '%s\n' "${submit_response}" | jq -e '.data | length == 1 and .[0].verdict == "flag is correct."' >/dev/null; then
      echo "attack submission failed for ${attacker_name} -> ${victim_name}" >&2
      printf '%s\n' "${submit_response}" >&2
      exit 1
    fi

    total_submissions=$((total_submissions + 1))
  done
done

scoreboard_response="$(
  curl_json "recompute scoring" -X POST "${API_URL}/api/v2/admin/game/scoring/recompute" \
    -H "Authorization: Bearer ${ADMIN_TOKEN}"
)"
scoreboard_rows="$(printf '%s\n' "${scoreboard_response}" | jq '.data | length')"

attack_feed_response="$(
  curl_json "attack feed summary" "${API_URL}/api/v2/attacks?service=${CHALLENGE_NAME}&limit=200" \
    -H "Authorization: Bearer ${participant_tokens[0]}"
)"
attack_count="$(printf '%s\n' "${attack_feed_response}" | jq -r '.data.total_count')"

printf 'attack-map load ready: challenge_id=%s teams=%s rounds=%s accepted_attacks=%s scoreboard_rows=%s\n' \
  "${challenge_id}" "${TEAM_COUNT}" "${ATTACK_MAP_LOAD_ROUNDS}" "${attack_count}" "${scoreboard_rows}"
echo "tip: increase organizer attack page size to 96 to see more teams in one slice."

if [[ -n "${ATTACK_MAP_LOAD_ARTIFACT_FILE}" ]]; then
  mkdir -p "$(dirname "${ATTACK_MAP_LOAD_ARTIFACT_FILE}")"
  {
    echo "SIM_CHALLENGE_ID=${challenge_id}"
    echo "SIM_TEAM_COUNT=${TEAM_COUNT}"
    echo "SIM_ATTACK_ROUNDS=${ATTACK_MAP_LOAD_ROUNDS}"
    echo "SIM_TEAM_PREFIX=${TEAM_PREFIX}"
    echo "SIM_TOTAL_SUBMISSIONS=${total_submissions}"
    echo "SIM_ATTACK_FEED_TOTAL=${attack_count}"
    for ((i = 0; i < TEAM_COUNT; i++)); do
      idx=$((i + 1))
      printf 'SIM_TEAM_%02d_ID=%s\n' "${idx}" "${team_ids[$i]}"
      printf 'SIM_TEAM_%02d_NAME=%q\n' "${idx}" "${team_names[$i]}"
      printf 'SIM_TEAM_%02d_EMAIL=%q\n' "${idx}" "${player_emails[$i]}"
      printf 'SIM_TEAM_%02d_PASSWORD=%q\n' "${idx}" "${player_passwords[$i]}"
    done
  } > "${ATTACK_MAP_LOAD_ARTIFACT_FILE}"
fi
