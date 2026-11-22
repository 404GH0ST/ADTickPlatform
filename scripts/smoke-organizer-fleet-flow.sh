#!/usr/bin/env bash
# Organizer-created fleet smoke: a full match shaped like a real event.
#
#   - 10 teams, each with a captain player
#   - 6 challenges:
#       2 pwn (tcp) + 1 crypto (tcp)  -> raw-TCP fleet service/checker
#       3 web (http): flask, nodejs, php -> sample-http checker contract
#   - a scheduler-driven tick at the production cadence (default 180s)
#
# It deploys every challenge to every team, runs one authoritative tick, and
# verifies that every checker run succeeds, that one team can capture and submit
# a current enemy flag on all six challenges, and that the scoreboard lists all
# teams. Unlike smoke-organizer-created-flow.sh (2 teams, 1 challenge) its
# assertions are generalized over the fleet instead of hardcoded scores.
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${ROOT_DIR}"
source "${ROOT_DIR}/scripts/lib/common.sh"
load_default_env_files

require_bin curl
require_bin docker
require_bin jq
require_bin psql

API_URL="${AD_PLATFORM_API_URL:-http://127.0.0.1:8080}"
READY_URL="${ORGANIZER_SMOKE_READY_URL:-${API_URL%/}/readyz}"
TEAM_COUNT="${FLEET_SMOKE_TEAM_COUNT:-10}"
TICK_INTERVAL_SECONDS="${FLEET_SMOKE_TICK_INTERVAL_SECONDS:-180}"
TARGET_TICKS="${FLEET_SMOKE_TARGET_TICKS:-1}"
SCHEDULER_WAIT_SECONDS="${FLEET_SMOKE_SCHEDULER_WAIT_SECONDS:-$((TICK_INTERVAL_SECONDS * TARGET_TICKS + 120))}"
ARTIFACT_FILE="${FLEET_SMOKE_ARTIFACT_FILE:-.runtime/last-prod-fleet-match.env}"

PROD_ENV="${PROD_ENV:-deploy/compose/prod.env}"
load_env_file_override "${PROD_ENV}" 2>/dev/null || true
POSTGRES_DSN="${POSTGRES_DSN_HOST_ENFORCEMENT:-${POSTGRES_DSN:-}}"

# Challenge fleet. Columns: key|service_image|checker_image|octet|category|protocol|kind
FLEET_ROWS=(
  "pwn-alpha|local/fleet-tcp:baseline|local/fleet-tcp-checker:latest|51|pwn|tcp|tcp"
  "pwn-bravo|local/fleet-tcp:baseline|local/fleet-tcp-checker:latest|52|pwn|tcp|tcp"
  "crypto-cipher|local/fleet-tcp:baseline|local/fleet-tcp-checker:latest|53|crypto|tcp|tcp"
  "web-flask|local/fleet-web-flask:baseline|local/sample-http-checker:latest|54|web|http|flask"
  "web-node|local/fleet-web-node:baseline|local/sample-http-checker:latest|55|web|http|node"
  "web-php|local/fleet-web-php:baseline|local/sample-http-checker:latest|56|web|http|php"
)

curl_json() {
  local label="$1"; shift
  local response_file status
  response_file="$(mktemp)"
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

postgres_cli() {
  POSTGRES_DSN="${POSTGRES_DSN}" python3 "${ROOT_DIR}/scripts/lib/psql-env.py" "$@"
}

# client_cmd KIND HOST PORT MODE -> shell command string to run inside a service
# container. MODE=health exits 0 when the local service answers; MODE=leak
# prints the current flag disclosed by the deliberately insecure attack path.
client_cmd() {
  local kind="$1" host="$2" port="$3" mode="$4"
  case "${kind}" in
    tcp)
      if [[ "${mode}" == "health" ]]; then
        printf 'python3 -c %s' "\"import socket,sys; s=socket.create_connection(('${host}',${port}),5); s.sendall(b'PING\\n'); sys.exit(0 if s.recv(64).strip()==b'PONG' else 1)\""
      else
        printf 'python3 -c %s' "\"import socket; s=socket.create_connection(('${host}',${port}),5); s.sendall(b'LEAK\\n'); d=s.recv(4096).decode().strip(); print(d.split(' ',1)[1] if d.startswith('FLAG ') else '')\""
      fi
      ;;
    flask)
      if [[ "${mode}" == "health" ]]; then
        printf 'python3 -c %s' "\"import urllib.request,sys; sys.exit(0 if urllib.request.urlopen('http://${host}:${port}/health',timeout=5).status==200 else 1)\""
      else
        printf 'python3 -c %s' "\"import json,urllib.request; print(json.load(urllib.request.urlopen('http://${host}:${port}/leak',timeout=5))['flag'])\""
      fi
      ;;
    node)
      if [[ "${mode}" == "health" ]]; then
        printf 'node -e %s' "\"require('http').get('http://${host}:${port}/health',r=>process.exit(r.statusCode===200?0:1)).on('error',()=>process.exit(1))\""
      else
        printf 'node -e %s' "\"require('http').get('http://${host}:${port}/leak',r=>{let d='';r.on('data',c=>d+=c);r.on('end',()=>{try{console.log(JSON.parse(d).flag)}catch(e){process.exit(1)}})}).on('error',()=>process.exit(1))\""
      fi
      ;;
    php)
      if [[ "${mode}" == "health" ]]; then
        printf 'php -r %s' "'\$x=@file_get_contents(\"http://${host}:${port}/health\"); exit(\$x===false?1:0);'"
      else
        printf 'php -r %s' "'\$x=@file_get_contents(\"http://${host}:${port}/leak\"); \$d=json_decode(\$x,true); echo is_array(\$d)&&isset(\$d[\"flag\"])?\$d[\"flag\"]:\"\";'"
      fi
      ;;
    *)
      echo "unknown client kind: ${kind}" >&2
      return 1
      ;;
  esac
}

container_for() {
  docker ps -q --filter "label=adplatform.team_id=$1" --filter "label=adplatform.challenge_id=$2" | head -n1
}

echo "fleet smoke: teams=${TEAM_COUNT} challenges=${#FLEET_ROWS[@]} interval=${TICK_INTERVAL_SECONDS}s target_ticks=${TARGET_TICKS}"

wait_for_http "${READY_URL}" 60 "fleet smoke api"

echo "building fleet challenge images (skipping any already present)"
build_image() {
  local tag="$1" ctx="$2"
  if docker image inspect "${tag}" >/dev/null 2>&1; then
    echo "  reuse ${tag}"
    return 0
  fi
  docker build -t "${tag}" "${ctx}" >/dev/null
}
build_image local/sample-http-checker:latest examples/sample-http-challenge/checker
build_image local/fleet-tcp:baseline examples/fleet-tcp/service
build_image local/fleet-tcp-checker:latest examples/fleet-tcp/checker
build_image local/fleet-web-flask:baseline examples/fleet-web-flask/service
build_image local/fleet-web-node:baseline examples/fleet-web-node/service
build_image local/fleet-web-php:baseline examples/fleet-web-php/service

echo "resetting platform to an empty organizer-managed state"
BOOTSTRAP_CLEAR_TEAMS=true ./scripts/bootstrap-clean-match.sh >/dev/null
ADMIN_TOKEN="$(resolve_admin_api_token "${ROOT_DIR}/.runtime/backend-stack.env")"

# Deterministic team ids so service octets (team_id - 90) stay inside 1..254.
postgres_cli -c "ALTER SEQUENCE IF EXISTS teams_id_seq RESTART WITH 101" >/dev/null

UNIQUE_SUFFIX="$(date +%s)"
declare -a TEAM_IDS=()
declare -a TEAM_NAMES=()
declare -a TEAM_EMAILS=()
declare -a TEAM_PASSWORDS=()

echo "creating ${TEAM_COUNT} teams with captains"
for i in $(seq 1 "${TEAM_COUNT}"); do
  name="Team $(printf '%02d' "${i}") ${UNIQUE_SUFFIX}"
  contact="team-${i}-${UNIQUE_SUFFIX}@college.local"
  player_email="captain.${i}.${UNIQUE_SUFFIX}@college.local"
  password="$(random_secret)"
  team_response="$(curl_json "create team ${i}" -X POST "${API_URL}/api/v2/admin/teams" \
    -H @<(bearer_header "${ADMIN_TOKEN}") -H 'Content-Type: application/json' \
    -d "$(jq -nc --arg name "${name}" --arg email "${contact}" '{name:$name,contact_email:$email}')")"
  team_id="$(printf '%s' "${team_response}" | jq -er '.id')"
  curl_json "create player ${i}" -X POST "${API_URL}/api/v2/admin/players" \
    -H @<(bearer_header "${ADMIN_TOKEN}") -H 'Content-Type: application/json' \
    --data-binary @<(jq -nc --argjson team_id "${team_id}" \
      --arg display_name "Captain ${i} ${UNIQUE_SUFFIX}" --arg email "${player_email}" \
      --rawfile password <(printf '%s' "${password}") --arg role "captain" \
      '{team_id:$team_id,display_name:$display_name,email:$email,password:$password,role:$role}') >/dev/null
  TEAM_IDS+=("${team_id}")
  TEAM_NAMES+=("${name}")
  TEAM_EMAILS+=("${player_email}")
  TEAM_PASSWORDS+=("${password}")
done

declare -a CHAL_IDS=()
declare -a CHAL_KINDS=()
declare -a CHAL_PORTS=()
echo "creating and deploying ${#FLEET_ROWS[@]} challenges"
for row in "${FLEET_ROWS[@]}"; do
  IFS='|' read -r key service_image checker_image octet category protocol kind <<< "${row}"
  service_port="$((30000 + octet))"
  chal_response="$(curl_json "create challenge ${key}" -X POST "${API_URL}/api/v2/admin/challenges" \
    -H @<(bearer_header "${ADMIN_TOKEN}") -H 'Content-Type: application/json' \
    -d "$(jq -nc --arg name "fleet-${key}-${UNIQUE_SUFFIX}" \
      --arg baseline_image "${service_image}" --arg checker_image "${checker_image}" \
      --argjson service_port "${service_port}" --argjson service_subnet_octet "${octet}" \
      --arg category "${category}" --arg protocol "${protocol}" \
      '{name:$name,baseline_image:$baseline_image,checker_image:$checker_image,service_port:$service_port,service_subnet_octet:$service_subnet_octet,category:$category,protocol:$protocol}')")"
  chal_id="$(printf '%s' "${chal_response}" | jq -er '.id')"
  curl_json "validate ${key}" -X POST "${API_URL}/api/v2/admin/challenges/${chal_id}/validate" \
    -H @<(bearer_header "${ADMIN_TOKEN}") >/dev/null
  curl_json "deploy ${key}" -X POST "${API_URL}/api/v2/admin/challenges/${chal_id}/deploy" \
    -H @<(bearer_header "${ADMIN_TOKEN}") >/dev/null
  CHAL_IDS+=("${chal_id}")
  CHAL_KINDS+=("${kind}")
  CHAL_PORTS+=("${service_port}")
done

echo "reconciling deployments (${TEAM_COUNT} x ${#FLEET_ROWS[@]} instances)"
reconcile_response="$(curl_json "reconcile" -X POST "${API_URL}/api/v2/admin/deployments/reconcile" \
  -H @<(bearer_header "${ADMIN_TOKEN}"))"
printf '%s\n' "${reconcile_response}" | jq -c '{processed_jobs,processed_instances,completed_jobs}'

expected_instances="$((TEAM_COUNT * ${#FLEET_ROWS[@]}))"
echo "waiting for ${expected_instances} service instances to answer locally"
deadline=$((SECONDS + 180))
while true; do
  ready=0
  for ci in "${!CHAL_IDS[@]}"; do
    chal_id="${CHAL_IDS[$ci]}"; kind="${CHAL_KINDS[$ci]}"; port="${CHAL_PORTS[$ci]}"
    for team_id in "${TEAM_IDS[@]}"; do
      container="$(container_for "${team_id}" "${chal_id}")"
      [[ -n "${container}" ]] || continue
      if docker exec "${container}" sh -c "$(client_cmd "${kind}" 127.0.0.1 "${port}" health)" >/dev/null 2>&1; then
        ready=$((ready + 1))
      fi
    done
  done
  echo "  ready ${ready}/${expected_instances}"
  [[ "${ready}" -ge "${expected_instances}" ]] && break
  if (( SECONDS >= deadline )); then
    echo "only ${ready}/${expected_instances} instances became ready" >&2
    exit 1
  fi
  sleep 5
done

echo "starting match at ${TICK_INTERVAL_SECONDS}s cadence"
curl_json "start match" -X POST "${API_URL}/api/v2/admin/game/match/start" \
  -H @<(bearer_header "${ADMIN_TOKEN}") >/dev/null
# A match start may auto-advance tick 1 immediately; record it so we can require
# a genuinely scheduler-produced tick at the configured cadence afterwards.
pre_status="$(curl_json "pre-scheduler status" "${API_URL}/api/v2/admin/game/status" -H @<(bearer_header "${ADMIN_TOKEN}"))"
pre_tick="$(printf '%s' "${pre_status}" | jq -r '.current_tick.id // 0')"
curl_json "set interval" -X PUT "${API_URL}/api/v2/admin/game/scheduler/interval" \
  -H @<(bearer_header "${ADMIN_TOKEN}") -H 'Content-Type: application/json' \
  -d "$(jq -nc --argjson interval_seconds "${TICK_INTERVAL_SECONDS}" '{interval_seconds:$interval_seconds}')" >/dev/null
sched_start=${SECONDS}
curl_json "start scheduler" -X POST "${API_URL}/api/v2/admin/game/scheduler/start" \
  -H @<(bearer_header "${ADMIN_TOKEN}") >/dev/null

# The configured interval must actually be the 3-minute cadence.
sched_interval="$(curl_json "scheduler status" "${API_URL}/api/v2/admin/game/status" -H @<(bearer_header "${ADMIN_TOKEN}") | jq -r '.scheduler.interval_seconds // 0')"
if [[ "${sched_interval}" != "${TICK_INTERVAL_SECONDS}" ]]; then
  echo "scheduler interval is ${sched_interval}s, expected ${TICK_INTERVAL_SECONDS}s" >&2
  exit 1
fi

echo "waiting up to ${SCHEDULER_WAIT_SECONDS}s for a scheduler-produced tick (> ${pre_tick}) at ${TICK_INTERVAL_SECONDS}s"
tick_id=""
deadline=$((SECONDS + SCHEDULER_WAIT_SECONDS))
while (( SECONDS < deadline )); do
  status_json="$(curl_json "game status" "${API_URL}/api/v2/admin/game/status" -H @<(bearer_header "${ADMIN_TOKEN}"))"
  current_tick="$(printf '%s' "${status_json}" | jq -r '.current_tick.id // 0')"
  tick_status="$(printf '%s' "${status_json}" | jq -r '.current_tick.status // ""')"
  if [[ "${current_tick}" =~ ^[0-9]+$ ]] && (( current_tick > pre_tick )) && (( current_tick >= TARGET_TICKS )) && [[ "${tick_status}" == "completed" ]]; then
    tick_id="${current_tick}"
    elapsed=$((SECONDS - sched_start))
    printf '%s\n' "${status_json}" | jq -c --argjson e "${elapsed}" '{match:.match.state,scheduler:.scheduler.state,interval:.scheduler.interval_seconds,current_tick:.current_tick.id,elapsed_s:$e,total_checker_runs}'
    # A scheduler tick must not arrive implausibly early for the configured cadence.
    if (( elapsed < TICK_INTERVAL_SECONDS - 30 )); then
      echo "scheduler tick arrived after ${elapsed}s, far below the ${TICK_INTERVAL_SECONDS}s cadence" >&2
      curl_json "stop scheduler" -X POST "${API_URL}/api/v2/admin/game/scheduler/stop" -H @<(bearer_header "${ADMIN_TOKEN}") >/dev/null
      exit 1
    fi
    break
  fi
  sleep 3
done
curl_json "stop scheduler" -X POST "${API_URL}/api/v2/admin/game/scheduler/stop" \
  -H @<(bearer_header "${ADMIN_TOKEN}") >/dev/null
if [[ -z "${tick_id}" ]]; then
  echo "scheduler did not produce a completed tick beyond ${pre_tick} within ${SCHEDULER_WAIT_SECONDS}s" >&2
  exit 1
fi

echo "verifying every checker run succeeded for tick ${tick_id}"
for ci in "${!CHAL_IDS[@]}"; do
  chal_id="${CHAL_IDS[$ci]}"
  runs="$(curl_json "checker runs ${chal_id}" \
    "${API_URL}/api/v2/admin/game/checker-runs?tick_id=${tick_id}&challenge_id=${chal_id}&limit=200" \
    -H @<(bearer_header "${ADMIN_TOKEN}"))"
  if ! printf '%s' "${runs}" | jq -e --argjson teams "${TEAM_COUNT}" \
    '(.items | length) > 0 and (all(.items[]; .status == "success")) and ((.items | map(.team_id) | unique | length) == $teams)' >/dev/null; then
    echo "challenge ${chal_id} had non-success checker runs or missing teams" >&2
    printf '%s' "${runs}" | jq -c '.items[] | select(.status != "success") | {team_id,phase,status,message}' >&2
    exit 1
  fi
done
echo "  all $((TEAM_COUNT * ${#FLEET_ROWS[@]})) instances passed checks"

# One team captures and submits a current enemy flag on every challenge.
attacker_idx=0
victim_idx=1
attacker_team="${TEAM_IDS[$attacker_idx]}"
victim_team="${TEAM_IDS[$victim_idx]}"
attacker_token="$(curl_json "auth attacker" -X POST "${API_URL}/api/v2/authenticate" \
  -H 'Content-Type: application/json' \
  --data-binary @<(jq -nc --arg email "${TEAM_EMAILS[$attacker_idx]}" \
    --rawfile password <(printf '%s' "${TEAM_PASSWORDS[$attacker_idx]}") '{email:$email,password:$password}') | jq -er '.token')"

public_services="$(curl_json "service targets" "${API_URL}/api/v2/services" -H @<(bearer_header "${attacker_token}"))"

echo "capturing a current enemy flag on all ${#FLEET_ROWS[@]} challenges (team ${attacker_team} vs ${victim_team})"
declare -a STOLEN=()
for ci in "${!CHAL_IDS[@]}"; do
  chal_id="${CHAL_IDS[$ci]}"; kind="${CHAL_KINDS[$ci]}"
  victim_endpoint="$(printf '%s' "${public_services}" | jq -er --arg c "${chal_id}" --arg t "${victim_team}" '.[$c][$t][0]')"
  victim_host="${victim_endpoint%%:*}"; victim_port="${victim_endpoint##*:}"
  attacker_container="$(container_for "${attacker_team}" "${chal_id}")"
  stolen="$(docker exec "${attacker_container}" sh -c "$(client_cmd "${kind}" "${victim_host}" "${victim_port}" leak)" | tr -d '\r\n')"
  if [[ -z "${stolen}" ]]; then
    echo "failed to capture a flag on challenge ${chal_id} (${kind}) from ${victim_endpoint}" >&2
    exit 1
  fi
  STOLEN+=("${stolen}")
done

submit_payload="$(printf '%s\n' "${STOLEN[@]}" | jq -R . | jq -sc '{flags:.}')"
submit_response="$(curl_json "submit stolen flags" -X POST "${API_URL}/api/v2/submit" \
  -H @<(bearer_header "${attacker_token}") -H 'Content-Type: application/json' -d "${submit_payload}")"
accepted="$(printf '%s' "${submit_response}" | jq '[.results[] | select(.status == "accepted")] | length')"
if [[ "${accepted}" != "${#FLEET_ROWS[@]}" ]]; then
  echo "expected ${#FLEET_ROWS[@]} accepted captures, got ${accepted}" >&2
  printf '%s' "${submit_response}" | jq -c '.results[] | {flag,status,detail}' >&2
  exit 1
fi
echo "  ${accepted}/${#FLEET_ROWS[@]} captures accepted"

echo "edge cases: own flag, duplicate, and bogus submissions are all rejected"
reject_count() { printf '%s' "$1" | jq '[.results[] | select(.status != "accepted")] | length'; }

# a) A team cannot score its own current flag (captured from its own instance).
own_endpoint="$(printf '%s' "${public_services}" | jq -er --arg c "${CHAL_IDS[0]}" --arg t "${attacker_team}" '.[$c][$t][0]')"
own_host="${own_endpoint%%:*}"; own_port="${own_endpoint##*:}"
own_container="$(container_for "${attacker_team}" "${CHAL_IDS[0]}")"
own_flag="$(docker exec "${own_container}" sh -c "$(client_cmd "${CHAL_KINDS[0]}" "${own_host}" "${own_port}" leak)" | tr -d '\r\n')"
own_resp="$(curl_json "submit own flag" -X POST "${API_URL}/api/v2/submit" \
  -H @<(bearer_header "${attacker_token}") -H 'Content-Type: application/json' \
  -d "$(jq -nc --arg f "${own_flag}" '{flags:[$f]}')")"
if [[ "$(reject_count "${own_resp}")" != "1" ]]; then
  echo "a team was able to submit its own flag" >&2
  printf '%s' "${own_resp}" | jq -c '.results[] | {status,detail}' >&2
  exit 1
fi

# b) An already-accepted stolen flag is rejected on resubmission (no double score).
dup_resp="$(curl_json "resubmit stolen flag" -X POST "${API_URL}/api/v2/submit" \
  -H @<(bearer_header "${attacker_token}") -H 'Content-Type: application/json' \
  -d "$(jq -nc --arg f "${STOLEN[0]}" '{flags:[$f]}')")"
if [[ "$(reject_count "${dup_resp}")" != "1" ]]; then
  echo "an already-accepted flag was accepted again" >&2
  exit 1
fi

# c) A bogus flag is rejected with a verdict (not silently dropped).
bogus_resp="$(curl_json "submit bogus flag" -X POST "${API_URL}/api/v2/submit" \
  -H @<(bearer_header "${attacker_token}") -H 'Content-Type: application/json' \
  -d '{"flags":["ADPL{this-flag-was-never-issued}"]}')"
if [[ "$(reject_count "${bogus_resp}")" != "1" ]]; then
  echo "a bogus flag was not rejected" >&2
  exit 1
fi
echo "  own/duplicate/bogus submissions rejected as expected"

echo "edge cases: each challenge persisted its category and protocol"
all_challenges="$(curl_json "list challenges" "${API_URL}/api/v2/admin/challenges" \
  -H @<(bearer_header "${ADMIN_TOKEN}"))"
for ci in "${!CHAL_IDS[@]}"; do
  IFS='|' read -r _ _ _ _ want_cat want_proto _ <<< "${FLEET_ROWS[$ci]}"
  got_cat="$(printf '%s' "${all_challenges}" | jq -r --argjson id "${CHAL_IDS[$ci]}" '.[] | select(.id == $id) | .category // ""')"
  got_proto="$(printf '%s' "${all_challenges}" | jq -r --argjson id "${CHAL_IDS[$ci]}" '.[] | select(.id == $id) | .protocol // ""')"
  if [[ "${got_cat}" != "${want_cat}" || "${got_proto}" != "${want_proto}" ]]; then
    echo "challenge ${CHAL_IDS[$ci]} metadata mismatch: category ${got_cat}/${want_cat} protocol ${got_proto}/${want_proto}" >&2
    exit 1
  fi
done
cat_summary="$(printf '%s\n' "${FLEET_ROWS[@]}" | awk -F'|' '{print $5"/"$6}' | sort | uniq -c | tr '\n' ' ')"
echo "  category/protocol mix verified: ${cat_summary}"

echo "recomputing scoreboard"
scoreboard="$(curl_json "recompute scoring" -X POST "${API_URL}/api/v2/admin/game/scoring/recompute" \
  -H @<(bearer_header "${ADMIN_TOKEN}"))"
rows="$(printf '%s' "${scoreboard}" | jq 'length')"
if [[ "${rows}" != "${TEAM_COUNT}" ]]; then
  echo "scoreboard listed ${rows} teams, expected ${TEAM_COUNT}" >&2
  exit 1
fi
printf '%s' "${scoreboard}" | jq -c 'sort_by(-.total) | .[] | {team,attack,defense,sla,total}' | head -n3

printf 'fleet smoke passed: teams=%s challenges=%s tick=%s interval=%ss\n' \
  "${TEAM_COUNT}" "${#FLEET_ROWS[@]}" "${tick_id}" "${TICK_INTERVAL_SECONDS}"

if [[ -n "${ARTIFACT_FILE}" ]]; then
  # Legacy SMOKE_* keys keep go-live-check and host-recovery (which consume the
  # short-match artifact) working now that short-match uses this fleet flow.
  smoke_one_endpoint="$(printf '%s' "${public_services}" | jq -r --arg c "${CHAL_IDS[0]}" --arg t "${attacker_team}" '.[$c][$t][0] // ""')"
  smoke_two_endpoint="$(printf '%s' "${public_services}" | jq -r --arg c "${CHAL_IDS[0]}" --arg t "${victim_team}" '.[$c][$t][0] // ""')"
  write_private_artifact "${ARTIFACT_FILE}" <<EOF
FLEET_TICK_ID=${tick_id}
FLEET_TICK_INTERVAL_SECONDS=${TICK_INTERVAL_SECONDS}
FLEET_TEAM_COUNT=${TEAM_COUNT}
FLEET_CHALLENGE_COUNT=${#FLEET_ROWS[@]}
FLEET_ATTACKER_TEAM_ID=${attacker_team}
FLEET_VICTIM_TEAM_ID=${victim_team}
FLEET_ATTACKER_EMAIL=${TEAM_EMAILS[$attacker_idx]}
FLEET_ATTACKER_PASSWORD=${TEAM_PASSWORDS[$attacker_idx]}
SMOKE_CHALLENGE_ID=${CHAL_IDS[0]}
SMOKE_TEAM_ONE_ID=${attacker_team}
SMOKE_TEAM_TWO_ID=${victim_team}
SMOKE_TEAM_ONE_PLAYER_EMAIL=${TEAM_EMAILS[$attacker_idx]}
SMOKE_TEAM_ONE_PASSWORD=${TEAM_PASSWORDS[$attacker_idx]}
SMOKE_TEAM_TWO_PLAYER_EMAIL=${TEAM_EMAILS[$victim_idx]}
SMOKE_TEAM_TWO_PASSWORD=${TEAM_PASSWORDS[$victim_idx]}
SMOKE_TEAM_ONE_ENDPOINT=${smoke_one_endpoint}
SMOKE_TEAM_TWO_ENDPOINT=${smoke_two_endpoint}
EOF
fi
