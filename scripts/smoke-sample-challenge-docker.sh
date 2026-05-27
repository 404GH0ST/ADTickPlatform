#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${ROOT_DIR}"
source "${ROOT_DIR}/scripts/lib/common.sh"
load_default_env_files

require_bin curl
require_bin docker
require_bin jq

NETWORK="${CONTROLLER_DOCKER_NETWORK:-adplatform_game}"
SUBNET="${AD_PLATFORM_GAME_NETWORK_SUBNET:-10.80.0.0/16}"
NETWORK_LAYOUT="${AD_PLATFORM_NETWORK_LAYOUT:-per-service}"
BASELINE_IMAGE="${SAMPLE_CHALLENGE_BASELINE_IMAGE:-adplatform/sample-http:baseline}"
CHECKER_IMAGE="${SAMPLE_CHALLENGE_CHECKER_IMAGE:-adplatform/sample-http-checker:latest}"
CHALLENGE_NAME="sample-http-$(date +%s)"
STACK_LOG="${ROOT_DIR}/.runtime/logs/sample-docker-stack.log"
STACK_PID=""
STACK_OWNED="false"
ADMIN_TOKEN=""
EMAIL="${AD_PLATFORM_EMAIL:-alpha.captain@example.com}"
PASSWORD="${AD_PLATFORM_PASSWORD:-alpha-secret}"
STACK_ENV_FILE="${ROOT_DIR}/.runtime/backend-stack.env"

cleanup() {
  local exit_code=$?
  trap - EXIT INT TERM
  if [[ "${STACK_OWNED}" == "true" && -n "${STACK_PID}" ]]; then
    ./scripts/bootstrap-clean-match.sh >/dev/null 2>&1 || true
    kill -- "-${STACK_PID}" 2>/dev/null || kill "${STACK_PID}" 2>/dev/null || true
    wait "${STACK_PID}" 2>/dev/null || true
  fi
  exit "${exit_code}"
}

trap cleanup EXIT INT TERM

mkdir -p .runtime/logs

if [[ "${NETWORK_LAYOUT}" == "shared" ]] && ! docker network inspect "${NETWORK}" >/dev/null 2>&1; then
  docker network create --subnet "${SUBNET}" "${NETWORK}" >/dev/null
  echo "created docker network ${NETWORK} (${SUBNET})"
fi

echo "building sample challenge images"
docker build -t "${BASELINE_IMAGE}" examples/sample-http-challenge/service >/dev/null
docker build -t "${CHECKER_IMAGE}" examples/sample-http-challenge/checker >/dev/null

if http_ready "http://127.0.0.1:8080/readyz" && http_ready "http://127.0.0.1:8081/readyz"; then
  if [[ -f "${STACK_ENV_FILE}" ]]; then
    load_env_file_override "${STACK_ENV_FILE}"
    if [[ "${AD_PLATFORM_STACK_MODE:-}" != "postgres" ]]; then
      echo "existing backend stack is not postgres-backed; sample docker smoke requires postgres mode" >&2
      exit 1
    fi
    if [[ "${CONTROLLER_RUNTIME_MODE:-}" != "docker" || "${CHECKER_RUNNER_MODE:-}" != "docker" ]]; then
      echo "existing backend stack is not running controller/checker in docker mode; restart it with CONTROLLER_RUNTIME_MODE=docker CHECKER_RUNNER_MODE=docker" >&2
      exit 1
    fi
    echo "reusing active backend stack from ${STACK_ENV_FILE}"
  else
    echo "reusing active backend stack on default ports without ${STACK_ENV_FILE}; assuming local defaults"
  fi
else
  echo "starting backend stack in docker mode"
  if command -v setsid >/dev/null 2>&1; then
    (
      cd "${ROOT_DIR}"
      exec setsid env \
        CONTROLLER_RUNTIME_MODE=docker \
        CHECKER_RUNNER_MODE=docker \
        AD_PLATFORM_NETWORK_LAYOUT="${NETWORK_LAYOUT}" \
        CONTROLLER_DOCKER_NETWORK="${NETWORK}" \
        CHECKER_RUNNER_DOCKER_NETWORK="${NETWORK}" \
        AD_PLATFORM_GAME_NETWORK_SUBNET="${SUBNET}" \
        ./scripts/run-backend-stack.sh postgres >>"${STACK_LOG}" 2>&1
    ) &
  else
    (
      cd "${ROOT_DIR}"
      exec env \
        CONTROLLER_RUNTIME_MODE=docker \
        CHECKER_RUNNER_MODE=docker \
        AD_PLATFORM_NETWORK_LAYOUT="${NETWORK_LAYOUT}" \
        CONTROLLER_DOCKER_NETWORK="${NETWORK}" \
        CHECKER_RUNNER_DOCKER_NETWORK="${NETWORK}" \
        AD_PLATFORM_GAME_NETWORK_SUBNET="${SUBNET}" \
        ./scripts/run-backend-stack.sh postgres >>"${STACK_LOG}" 2>&1
    ) &
  fi
  STACK_PID=$!
  STACK_OWNED="true"

  wait_for_http "http://127.0.0.1:8080/readyz" 60
  wait_for_http "http://127.0.0.1:8081/readyz" 60

  if [[ ! -f "${STACK_ENV_FILE}" ]]; then
    echo "backend stack started but ${STACK_ENV_FILE} was not written" >&2
    exit 1
  fi

  load_env_file_override "${STACK_ENV_FILE}"
fi
ADMIN_TOKEN="$(resolve_admin_api_token "${STACK_ENV_FILE}")"

echo "resetting postgres state to a clean match baseline"
./scripts/bootstrap-clean-match.sh >/dev/null

echo "running organizer runtime smoke"
runtime_output="$(
  env \
    RUNTIME_SMOKE_CHALLENGE_NAME="${CHALLENGE_NAME}" \
    RUNTIME_SMOKE_BASELINE_IMAGE="${BASELINE_IMAGE}" \
    RUNTIME_SMOKE_CHECKER_IMAGE="${CHECKER_IMAGE}" \
    RUNTIME_SMOKE_KEEP_CHALLENGE=1 \
    ./scripts/smoke-admin-runtime-flow.sh
)"
printf '%s\n' "${runtime_output}"

if printf '%s\n' "${runtime_output}" | grep -q "dry-run runtime mode"; then
  echo "runtime smoke ran against a dry-run controller/checker path; restart the stack in docker mode before running the sample smoke" >&2
  exit 1
fi

challenge_id="$(printf '%s\n' "${runtime_output}" | sed -n 's/.*challenge_id=\([0-9][0-9]*\).*/\1/p' | tail -n1)"
endpoint="$(printf '%s\n' "${runtime_output}" | sed -n 's/.*endpoint=\([^[:space:]]*\).*/\1/p' | tail -n1)"

if [[ -z "${challenge_id}" || -z "${endpoint}" ]]; then
  echo "failed to parse challenge_id or endpoint from runtime smoke output" >&2
  exit 1
fi

auth_payload="$(jq -nc --arg email "${EMAIL}" --arg password "${PASSWORD}" '{email:$email,password:$password}')"
team_token="$(
  curl -fsS -X POST "http://127.0.0.1:8080/api/v2/authenticate" \
    -H 'Content-Type: application/json' \
    -d "${auth_payload}" |
    jq -er '.token'
)"

echo "fetching unlock proof from deployed service"
container_name="svc-$(slug_name "${CHALLENGE_NAME}")-team-101"
if ! docker ps -a --format '{{.Names}}' | grep -qx "${container_name}"; then
  echo "expected runtime container ${container_name} was not created; deployment did not reach the real docker runtime" >&2
  exit 1
fi
if ! wait_for_http "http://${endpoint}/health" 30; then
  docker exec -i "${container_name}" python3 - <<'PY' >/dev/null
import os
from urllib.request import urlopen
port = os.environ.get("AD_PLATFORM_SERVICE_PORT", os.environ.get("PORT", "8080"))
with urlopen(f"http://127.0.0.1:{port}/health", timeout=5) as response:
    if response.status != 200:
        raise SystemExit(1)
PY
fi
unlock_proof="$(
  curl -fsS "http://${endpoint}/unlock" 2>/dev/null | jq -er '.proof' 2>/dev/null || \
  docker exec -i "${container_name}" python3 - <<'PY'
import json
import os
from urllib.request import urlopen
port = os.environ.get("AD_PLATFORM_SERVICE_PORT", os.environ.get("PORT", "8080"))
with urlopen(f"http://127.0.0.1:{port}/unlock", timeout=5) as response:
    body = json.loads(response.read().decode("utf-8"))
    print(body["proof"])
PY
)"

echo "unlocking service through participant API"
curl -fsS -X POST "http://127.0.0.1:8080/api/v2/services/${challenge_id}/unlock" \
  -H "Authorization: Bearer ${team_token}" \
  -H 'Content-Type: application/json' \
  -d "$(jq -nc --arg proof "${unlock_proof}" '{proof:$proof}')" |
  jq -c '{challenge_id,team_id,unlocked}'

echo "requesting ssh credential"
curl -fsS -X POST "http://127.0.0.1:8080/api/v2/services/${challenge_id}/ssh-session" \
  -H "Authorization: Bearer ${team_token}" |
  jq -c '{host,port,username,password_present:(.password | length > 0)}'

echo "starting match"
curl -fsS -X POST "http://127.0.0.1:8080/api/v2/admin/game/match/start" \
  -H "Authorization: Bearer ${ADMIN_TOKEN}" |
  jq -c '{state,accepting_submissions,started_at}'

echo "advancing authoritative game tick"
tick_response="$(
  curl -fsS -X POST "http://127.0.0.1:8080/api/v2/admin/game/ticks/advance" \
    -H "Authorization: Bearer ${ADMIN_TOKEN}"
)"
printf '%s\n' "${tick_response}" | jq -c '{id,status,total_checker_runs,successful_checker_runs,failed_checker_runs,skipped_checker_runs,message}'

tick_id="$(printf '%s\n' "${tick_response}" | jq -er '.id')"

echo "reading persisted checker runs for deployed challenge"
checker_runs_response="$(
  curl -fsS "http://127.0.0.1:8080/api/v2/admin/game/checker-runs?tick_id=${tick_id}&challenge_id=${challenge_id}&limit=20" \
    -H "Authorization: Bearer ${ADMIN_TOKEN}"
)"
printf '%s\n' "${checker_runs_response}" | jq -c '{total_count,has_next}'

checker_run_count="$(printf '%s\n' "${checker_runs_response}" | jq -er '.total_count')"
if [[ "${checker_run_count}" -ne 12 ]]; then
  echo "unexpected checker run count for clean sample tick: got ${checker_run_count}, want 12" >&2
  exit 1
fi

if ! printf '%s\n' "${checker_runs_response}" | jq -e '.items | length == 12 and all(.[]; .status == "success")' >/dev/null; then
  echo "checker runs were not all successful for the clean sample tick" >&2
  printf '%s\n' "${checker_runs_response}" | jq -c '.items[] | {team_id,phase,status,message}' >&2
  exit 1
fi

printf '%s\n' "${checker_runs_response}" | jq -c '.items[] | {team_id,phase,status,target}'

echo "recomputing scoreboard from authoritative tick state"
scoreboard_response="$(
  curl -fsS -X POST "http://127.0.0.1:8080/api/v2/admin/game/scoring/recompute" \
    -H "Authorization: Bearer ${ADMIN_TOKEN}"
)"
printf '%s\n' "${scoreboard_response}" | jq -c '.[] | {rank,team,attack,defense,sla,total,delta}'

if ! printf '%s\n' "${scoreboard_response}" | jq -e 'length == 4 and all(.[]; .attack == 0 and .defense == 0 and .sla == 2 and .total == 2)' >/dev/null; then
  echo "unexpected scoreboard after clean sample tick" >&2
  exit 1
fi

echo "stealing one live enemy flag through the sample vulnerability"
public_services="$(
  curl -fsS "http://127.0.0.1:8080/api/v2/services" \
    -H "Authorization: Bearer ${team_token}"
)"
victim_endpoint="$(
  printf '%s\n' "${public_services}" |
    jq -er --arg challenge_id "${challenge_id}" '.[$challenge_id]["102"][0]'
)"
stolen_flag="$(
  docker exec -i "${container_name}" python3 - <<PY
import json
from urllib.request import urlopen

endpoint = "${victim_endpoint}"
with urlopen(f"http://{endpoint}/leak", timeout=5) as response:
    body = json.loads(response.read().decode("utf-8"))
    print(body["flag"])
PY
)"
printf '  victim_endpoint=%s\n' "${victim_endpoint}"
printf '  stolen_flag_prefix=%s...\n' "${stolen_flag:0:20}"

echo "submitting the stolen flag through participant API"
submit_response="$(
  curl -fsS -X POST "http://127.0.0.1:8080/api/v2/submit" \
    -H "Authorization: Bearer ${team_token}" \
    -H 'Content-Type: application/json' \
    -d "$(jq -nc --arg flag "${stolen_flag}" '{flags:[$flag]}')"
)"
printf '%s\n' "${submit_response}" | jq -c '.results[] | {flag,status,detail}'
if ! printf '%s\n' "${submit_response}" | jq -e '.results | length == 1 and .[0].status == "accepted" and .[0].detail == "flag is correct."' >/dev/null; then
  echo "stolen flag submission did not return the expected success verdict" >&2
  exit 1
fi

echo "verifying duplicate submission handling"
duplicate_submit_response="$(
  curl -fsS -X POST "http://127.0.0.1:8080/api/v2/submit" \
    -H "Authorization: Bearer ${team_token}" \
    -H 'Content-Type: application/json' \
    -d "$(jq -nc --arg flag "${stolen_flag}" '{flags:[$flag]}')"
)"
printf '%s\n' "${duplicate_submit_response}" | jq -c '.results[] | {flag,status,detail}'
if ! printf '%s\n' "${duplicate_submit_response}" | jq -e '.results | length == 1 and .[0].status == "duplicate" and .[0].detail == "flag already submitted."' >/dev/null; then
  echo "duplicate stolen flag submission did not return the expected duplicate verdict" >&2
  exit 1
fi

echo "reading public attack feed for the accepted attack"
attack_feed_response="$(
  curl -fsS "http://127.0.0.1:8080/api/v2/attacks?service=${CHALLENGE_NAME}&tick_from=${tick_id}&tick_to=${tick_id}" 
)"
printf '%s\n' "${attack_feed_response}" | jq -c '.items[] | {attacker,victim,service,tick,verdict}'
if ! printf '%s\n' "${attack_feed_response}" | jq -e '
  .total_count == 1 and
  .items[0].attacker == "Team Alpha" and
  .items[0].victim == "Team Delta" and
  .items[0].service == "'"${CHALLENGE_NAME}"'" and
  .items[0].tick == '"${tick_id}"'
' >/dev/null; then
  echo "accepted attack feed did not match the expected stolen-flag event" >&2
  exit 1
fi

echo "recomputing scoreboard after the accepted attack"
post_submit_scoreboard="$(
  curl -fsS -X POST "http://127.0.0.1:8080/api/v2/admin/game/scoring/recompute" \
    -H "Authorization: Bearer ${ADMIN_TOKEN}"
)"
printf '%s\n' "${post_submit_scoreboard}" | jq -c '.[] | {rank,team,attack,defense,sla,total,delta}'

if ! printf '%s\n' "${post_submit_scoreboard}" | jq -e '
  length == 4 and
  any(.[]; .team == "Team Alpha" and .attack == 2 and .defense == 0 and .sla == 2 and .total == 4) and
  any(.[]; .team == "Team Delta" and .attack == 0 and .defense == -1 and .sla == 2 and .total == 1) and
  any(.[]; .team == "Team Orchid" and .attack == 0 and .defense == 0 and .sla == 2 and .total == 2) and
  any(.[]; .team == "Team Sigma" and .attack == 0 and .defense == 0 and .sla == 2 and .total == 2)
' >/dev/null; then
  echo "scoreboard did not reflect the accepted enemy-flag attack" >&2
  exit 1
fi

echo "sample docker smoke passed:"
printf '  challenge_id=%s tick_id=%s endpoint=%s image=%s checker=%s\n' "${challenge_id}" "${tick_id}" "${endpoint}" "${BASELINE_IMAGE}" "${CHECKER_IMAGE}"
