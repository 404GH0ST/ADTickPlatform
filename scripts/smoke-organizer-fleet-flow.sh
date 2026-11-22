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

# --- Dynamic multi-tick phase: several more ticks with evolving gameplay ------
DYN_TICKS="${FLEET_SMOKE_DYNAMIC_TICKS:-5}"
DYN_INTERVAL="${FLEET_SMOKE_DYNAMIC_INTERVAL_SECONDS:-20}"

team_token() { # <team index> -> token
  curl_json "auth team $1" -X POST "${API_URL}/api/v2/authenticate" -H 'Content-Type: application/json' \
    --data-binary @<(jq -nc --arg e "${TEAM_EMAILS[$1]}" --rawfile p <(printf '%s' "${TEAM_PASSWORDS[$1]}") '{email:$e,password:$p}') | jq -er '.token'
}
leak_current() { # <attacker team id> <challenge index> <victim team id> -> current flag
  local at="$1" ci="$2" vt="$3" cid kind ep host port cont
  cid="${CHAL_IDS[$ci]}"; kind="${CHAL_KINDS[$ci]}"
  ep="$(printf '%s' "${public_services}" | jq -er --arg c "${cid}" --arg t "${vt}" '.[$c][$t][0]')"
  host="${ep%%:*}"; port="${ep##*:}"; cont="$(container_for "${at}" "${cid}")"
  docker exec "${cont}" sh -c "$(client_cmd "${kind}" "${host}" "${port}" leak)" | tr -d '\r\n'
}
submit_one() { # <token> <flag> -> verdict status
  curl_json "submit" -X POST "${API_URL}/api/v2/submit" -H @<(bearer_header "$1") \
    -H 'Content-Type: application/json' -d "$(jq -nc --arg f "$2" '{flags:[$f]}')" | jq -r '.results[0].status'
}

echo "dynamic phase: >= ${DYN_TICKS} more ticks at ${DYN_INTERVAL}s with multi-team activity"
# Hold team D's CURRENT flag (challenge 0) without submitting, to prove rotation.
hold_attacker_idx=2; hold_victim_idx=3
held_flag="$(leak_current "${TEAM_IDS[$hold_attacker_idx]}" 0 "${TEAM_IDS[$hold_victim_idx]}")"
[[ -n "${held_flag}" ]] || { echo "failed to leak a flag to hold" >&2; exit 1; }

dyn_start_tick="${tick_id}"
# Run one tick PAST the target so there is always a fully-completed last tick to
# verify: stopping the scheduler cancels whatever tick is in flight (its runs
# are legitimately "skipped: tick cancelled"), so we never assert on that one.
dyn_target=$((dyn_start_tick + DYN_TICKS))
dyn_wait=$((dyn_target + 1))
curl_json "set dyn interval" -X PUT "${API_URL}/api/v2/admin/game/scheduler/interval" \
  -H @<(bearer_header "${ADMIN_TOKEN}") -H 'Content-Type: application/json' \
  -d "$(jq -nc --argjson i "${DYN_INTERVAL}" '{interval_seconds:$i}')" >/dev/null
curl_json "start dyn scheduler" -X POST "${API_URL}/api/v2/admin/game/scheduler/start" \
  -H @<(bearer_header "${ADMIN_TOKEN}") >/dev/null
dyn_deadline=$((SECONDS + DYN_INTERVAL * (DYN_TICKS + 2) + 300))
final_tick="${dyn_start_tick}"
while (( SECONDS < dyn_deadline )); do
  st="$(curl_json "dyn status" "${API_URL}/api/v2/admin/game/status" -H @<(bearer_header "${ADMIN_TOKEN}"))"
  final_tick="$(printf '%s' "${st}" | jq -r '.current_tick.id // 0')"
  cs="$(printf '%s' "${st}" | jq -r '.current_tick.status // ""')"
  (( final_tick >= dyn_wait )) && [[ "${cs}" == "completed" ]] && break
  sleep 3
done
curl_json "stop dyn scheduler" -X POST "${API_URL}/api/v2/admin/game/scheduler/stop" \
  -H @<(bearer_header "${ADMIN_TOKEN}") >/dev/null
# Verify only through the last tick that finished before we stopped the scheduler.
verify_to=$((final_tick - 1))
if (( verify_to < dyn_target )); then
  echo "dynamic phase produced only ${verify_to} fully-completed ticks, expected >= ${dyn_target}" >&2
  exit 1
fi
echo "  ran to tick ${final_tick}; verifying fully-completed ticks through ${verify_to} (>= ${DYN_TICKS} dynamic ticks)"

echo "verifying every checker run succeeded across dynamic ticks $((dyn_start_tick + 1))..${verify_to}"
for t in $(seq $((dyn_start_tick + 1)) "${verify_to}"); do
  for ci in "${!CHAL_IDS[@]}"; do
    runs="$(curl_json "runs t${t} c${CHAL_IDS[$ci]}" \
      "${API_URL}/api/v2/admin/game/checker-runs?tick_id=${t}&challenge_id=${CHAL_IDS[$ci]}&limit=200" \
      -H @<(bearer_header "${ADMIN_TOKEN}"))"
    if ! printf '%s' "${runs}" | jq -e --argjson teams "${TEAM_COUNT}" \
      '(.items | length) > 0 and (all(.items[]; .status == "success")) and ((.items | map(.team_id) | unique | length) == $teams)' >/dev/null; then
      echo "tick ${t} challenge ${CHAL_IDS[$ci]} had non-success or missing teams" >&2
      printf '%s' "${runs}" | jq -c '.items[] | select(.status != "success") | {team_id,phase,status,message}' >&2
      exit 1
    fi
  done
done
echo "  all ticks x challenges x teams passed checks"

# Advance one fresh, fully-completed tick: the scheduler was stopped on an
# in-flight (cancelled) tick, so services may hold flags from mixed ticks near
# expiry. A synchronous advance re-issues a current flag to every instance, and
# with the scheduler stopped it stays valid for the rotation + mesh below.
mesh_tick="$(curl_json "advance for capture round" -X POST "${API_URL}/api/v2/admin/game/ticks/advance" \
  -H @<(bearer_header "${ADMIN_TOKEN}") | jq -r '.id')"
echo "  advanced to fresh tick ${mesh_tick}; current flags synchronized across the fleet"

echo "flag rotation: the held flag has expired, the new current flag is capturable"
hold_token="$(team_token "${hold_attacker_idx}")"
held_status="$(submit_one "${hold_token}" "${held_flag}")"
if [[ "${held_status}" == "accepted" ]]; then
  echo "a flag held across ${DYN_TICKS}+ ticks was still accepted (no rotation)" >&2
  exit 1
fi
new_flag="$(leak_current "${TEAM_IDS[$hold_attacker_idx]}" 0 "${TEAM_IDS[$hold_victim_idx]}")"
new_status="$(submit_one "${hold_token}" "${new_flag}")"
if [[ "${new_flag}" == "${held_flag}" || "${new_status}" != "accepted" ]]; then
  echo "post-rotation current flag was not freshly capturable (status=${new_status})" >&2
  exit 1
fi
echo "  held flag rejected (${held_status}); rotated current flag accepted"

echo "dynamic attacks: dense mesh — every team attacks several enemies across challenges"
# Scheduler stays stopped since the fresh advance above, so the current flags
# are stable and valid while the mesh runs.
# Each team attacks a ring of enemies on a TCP and two web challenges; distinct
# (attacker, victim, challenge) triples so every one is a fresh capture.
declare -A mesh_attackers=() mesh_victims=()
mesh_try=0; mesh_ok=0
mesh_challets=(0 4 5)   # pwn/tcp, web/node, web/php
for a in $(seq 0 $((TEAM_COUNT - 1))); do
  a_tok="$(team_token "${a}")"
  for dv in 1 2 4; do
    v=$(( (a + dv) % TEAM_COUNT ))
    [[ "${v}" == "${a}" ]] && continue
    for ci in "${mesh_challets[@]}"; do
      f="$(leak_current "${TEAM_IDS[$a]}" "${ci}" "${TEAM_IDS[$v]}")"
      [[ -n "${f}" ]] || continue
      mesh_try=$((mesh_try + 1))
      if [[ "$(submit_one "${a_tok}" "${f}")" == "accepted" ]]; then
        mesh_ok=$((mesh_ok + 1)); mesh_attackers[$a]=1; mesh_victims[$v]=1
      fi
    done
  done
done
echo "  mesh: ${mesh_ok}/${mesh_try} captures accepted; ${#mesh_attackers[@]} distinct attackers, ${#mesh_victims[@]} distinct victims"
if (( mesh_try == 0 )) || (( mesh_ok < mesh_try * 9 / 10 )); then
  echo "attack mesh accepted too few captures (${mesh_ok}/${mesh_try})" >&2
  exit 1
fi
if (( ${#mesh_attackers[@]} < TEAM_COUNT )) || (( ${#mesh_victims[@]} < TEAM_COUNT )); then
  echo "attack mesh was not broad enough (attackers=${#mesh_attackers[@]}, victims=${#mesh_victims[@]}, need ${TEAM_COUNT} each)" >&2
  exit 1
fi

echo "attack feed reflects the dense activity"
feed="$(curl_json "attacks feed" "${API_URL}/api/v2/attacks?limit=2000")"
feed_attackers="$(printf '%s' "${feed}" | jq '[.items[]?.attacker] | unique | length')"
feed_victims="$(printf '%s' "${feed}" | jq '[.items[]?.victim] | unique | length')"
feed_pairs="$(printf '%s' "${feed}" | jq '[.items[]? | "\(.attacker)->\(.victim)"] | unique | length')"
echo "  attack feed: ${feed_attackers} attackers, ${feed_victims} victims, ${feed_pairs} distinct attacker->victim pairs"
if (( feed_attackers < TEAM_COUNT )) || (( feed_pairs < TEAM_COUNT * 2 )); then
  echo "attack feed does not show broad multi-team activity (attackers=${feed_attackers}, pairs=${feed_pairs})" >&2
  exit 1
fi

# --- Event-scenario phase: organizer + fault situations from a real match -----
submit_http() { # <token> <flagsjson> -> "<http_code> <accepted_count>"
  local rf code acc; rf="$(mktemp)"
  code="$(curl -sS -o "${rf}" -w '%{http_code}' -X POST "${API_URL}/api/v2/submit" \
    -H @<(bearer_header "$1") -H 'Content-Type: application/json' -d "$2" </dev/null)"
  acc="$(jq '[.results[]? | select(.status == "accepted")] | length' "${rf}" 2>/dev/null || echo 0)"
  rm -f "${rf}"; echo "${code} ${acc:-0}"
}
advance_tick() { # -> tick id
  curl_json "advance tick" -X POST "${API_URL}/api/v2/admin/game/ticks/advance" \
    -H @<(bearer_header "${ADMIN_TOKEN}") | jq -r '.id'
}
runs_status_for() { # <tick> <challenge id> <team id> -> status of that team's run
  curl_json "runs" "${API_URL}/api/v2/admin/game/checker-runs?tick_id=$1&challenge_id=$2&limit=200" \
    -H @<(bearer_header "${ADMIN_TOKEN}") | jq -r --argjson t "$3" '.items[] | select(.team_id == $t) | .status' | head -1
}
success_count_for() { # <tick> <challenge id> -> number of DISTINCT teams with a success run
  curl_json "runs" "${API_URL}/api/v2/admin/game/checker-runs?tick_id=$1&challenge_id=$2&limit=200" \
    -H @<(bearer_header "${ADMIN_TOKEN}") | jq '[.items[] | select(.status == "success") | .team_id] | unique | length'
}

echo "scenario: an accepted capture opens SSH and records first blood"
curl_json "team services" "${API_URL}/api/v2/team/services" -H @<(bearer_header "${attacker_token}") \
  | jq -e --argjson id "${CHAL_IDS[0]}" 'any(.[]; .challenge_id == $id and .unlocked == true)' >/dev/null \
  || { echo "attacker's SSH was not unlocked after capturing challenge ${CHAL_IDS[0]}" >&2; exit 1; }
curl_json "ssh session" -X POST "${API_URL}/api/v2/services/${CHAL_IDS[0]}/ssh-session" \
  -H @<(bearer_header "${attacker_token}") | jq -e '.username == "root" and (.password | length > 0)' >/dev/null \
  || { echo "ssh-session did not return a root credential" >&2; exit 1; }
fb="$(curl_json "attacks" "${API_URL}/api/v2/attacks?limit=500" | jq '[.items[]? | select(.first_blood == true)] | length')"
(( fb >= 1 )) || { echo "expected at least one first-blood attack event, got ${fb}" >&2; exit 1; }
echo "  SSH unlocked + root credential issued; first-blood events: ${fb}"

echo "scenario: a downed service loses its own SLA tick without affecting other teams"
down_idx=5; down_cid="${CHAL_IDS[0]}"
down_cont="$(container_for "${TEAM_IDS[$down_idx]}" "${down_cid}")"
docker stop "${down_cont}" >/dev/null
down_tick="$(advance_tick)"
down_status="$(runs_status_for "${down_tick}" "${down_cid}" "${TEAM_IDS[$down_idx]}")"
others_ok="$(curl_json "down runs" "${API_URL}/api/v2/admin/game/checker-runs?tick_id=${down_tick}&challenge_id=${down_cid}&limit=200" \
  -H @<(bearer_header "${ADMIN_TOKEN}") | jq --argjson t "${TEAM_IDS[$down_idx]}" '[.items[] | select(.team_id != $t)] | (length > 0 and all(.[]; .status == "success"))')"
[[ "${down_status}" != "success" ]] || { echo "a stopped service still passed its checker" >&2; exit 1; }
[[ "${others_ok}" == "true" ]] || { echo "one team's outage regressed other teams' checks" >&2; exit 1; }
docker start "${down_cont}" >/dev/null
for _ in $(seq 1 20); do docker exec "${down_cont}" sh -c "$(client_cmd "${CHAL_KINDS[0]}" 127.0.0.1 "${CHAL_PORTS[0]}" health)" >/dev/null 2>&1 && break; sleep 2; done
rec_tick="$(advance_tick)"
rec_status="$(runs_status_for "${rec_tick}" "${down_cid}" "${TEAM_IDS[$down_idx]}")"
[[ "${rec_status}" == "success" ]] || { echo "service did not recover after restart (status=${rec_status})" >&2; exit 1; }
echo "  downed team check=${down_status} (others success); recovered=${rec_status}"

echo "scenario: a challenge under maintenance is skipped and others keep running"
maint_cid="${CHAL_IDS[1]}"; other_cid="${CHAL_IDS[2]}"
curl_json "maintenance on" -X POST "${API_URL}/api/v2/admin/challenges/${maint_cid}/maintenance" \
  -H @<(bearer_header "${ADMIN_TOKEN}") >/dev/null
maint_tick="$(advance_tick)"
maint_succ="$(success_count_for "${maint_tick}" "${maint_cid}")"
other_succ="$(success_count_for "${maint_tick}" "${other_cid}")"
(( maint_succ == 0 )) || { echo "maintenance challenge still ran ${maint_succ} successful checks" >&2; exit 1; }
(( other_succ == TEAM_COUNT )) || { echo "a healthy challenge regressed during maintenance (${other_succ}/${TEAM_COUNT})" >&2; exit 1; }
curl_json "maintenance off" -X POST "${API_URL}/api/v2/admin/challenges/${maint_cid}/resume" \
  -H @<(bearer_header "${ADMIN_TOKEN}") >/dev/null
echo "  maintenance challenge skipped (0 checks); other challenge ${other_succ}/${TEAM_COUNT}"

echo "scenario: pausing the match blocks submissions; resume restores play"
pause_tok="$(team_token 9)"
pause_flag="$(leak_current "${TEAM_IDS[0]}" 4 "${TEAM_IDS[1]}")"
[[ -n "${pause_flag}" ]] || { echo "could not leak a flag for the pause scenario" >&2; exit 1; }
curl_json "pause match" -X POST "${API_URL}/api/v2/admin/game/match/pause" -H @<(bearer_header "${ADMIN_TOKEN}") >/dev/null
read -r paused_code paused_acc < <(submit_http "${pause_tok}" "$(jq -nc --arg f "${pause_flag}" '{flags:[$f]}')")
(( paused_acc == 0 )) || { echo "a submission was accepted while the match was paused (code=${paused_code})" >&2; exit 1; }
curl_json "resume match" -X POST "${API_URL}/api/v2/admin/game/match/resume" -H @<(bearer_header "${ADMIN_TOKEN}") >/dev/null
read -r resumed_code resumed_acc < <(submit_http "${pause_tok}" "$(jq -nc --arg f "${pause_flag}" '{flags:[$f]}')")
(( resumed_acc == 1 )) || { echo "the same capture was not accepted after resume (code=${resumed_code} acc=${resumed_acc})" >&2; exit 1; }
echo "  paused submission rejected; identical capture accepted after resume"

echo "recomputing scoreboard"
scoreboard="$(curl_json "recompute scoring" -X POST "${API_URL}/api/v2/admin/game/scoring/recompute" \
  -H @<(bearer_header "${ADMIN_TOKEN}"))"
rows="$(printf '%s' "${scoreboard}" | jq 'length')"
if [[ "${rows}" != "${TEAM_COUNT}" ]]; then
  echo "scoreboard listed ${rows} teams, expected ${TEAM_COUNT}" >&2
  exit 1
fi
# Dynamic gameplay must show up: several teams scored attack, and SLA accrued
# across the multi-tick span for defended services.
attack_teams="$(printf '%s' "${scoreboard}" | jq '[.[] | select(.attack > 0)] | length')"
sla_teams="$(printf '%s' "${scoreboard}" | jq '[.[] | select(.sla > 0)] | length')"
if (( attack_teams < TEAM_COUNT - 1 )); then
  echo "expected nearly all teams to have attack points after the mesh, got ${attack_teams}/${TEAM_COUNT}" >&2
  exit 1
fi
if (( sla_teams < TEAM_COUNT / 2 )); then
  echo "expected SLA to accrue for most teams over ${DYN_TICKS}+ ticks, got ${sla_teams}" >&2
  exit 1
fi
echo "  ${attack_teams} teams scored attack, ${sla_teams}/${TEAM_COUNT} accrued SLA over ${final_tick} ticks"
printf '%s' "${scoreboard}" | jq -c 'sort_by(-.total) | .[] | {team,attack,defense,sla,total}' | head -n3

printf 'fleet smoke passed: teams=%s challenges=%s ticks=%s (cadence=%ss, dynamic=%ss)\n' \
  "${TEAM_COUNT}" "${#FLEET_ROWS[@]}" "${final_tick}" "${TICK_INTERVAL_SECONDS}" "${DYN_INTERVAL}"

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
