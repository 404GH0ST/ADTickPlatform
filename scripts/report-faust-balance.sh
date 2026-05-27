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

BALANCE_REPORT_OUTPUT="${BALANCE_REPORT_OUTPUT:-.runtime/faust-balance-report-$(date -u +"%Y%m%dT%H%M%SZ").json}"

usage() {
  cat <<'EOF'
Usage: scripts/report-faust-balance.sh

Build a balance snapshot for the current Faust-style match using live API data.

Environment overrides:
  BALANCE_REPORT_OUTPUT=.runtime/faust-balance-report-<timestamp>.json
  AD_PLATFORM_API_URL
  AD_PLATFORM_PUBLIC_BASE_URL
  ADMIN_API_TOKEN
EOF
}

if [[ "${1:-}" == "--help" ]]; then
  usage
  exit 0
fi

mkdir -p "$(dirname "${BALANCE_REPORT_OUTPUT}")"

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

scoreboard="$(
  curl_json "scoreboard" \
    "${API_URL}/api/v2/admin/game/scoreboard" \
    -H "Authorization: Bearer ${ADMIN_TOKEN}"
)"
attacks="$(
  curl_json "attacks" \
    "${API_URL}/api/v2/attacks?limit=1"
)"
game_status="$(
  curl_json "game status" \
    "${API_URL}/api/v2/game/status"
)"

jq -n \
  --arg generated_at "$(date -u +"%Y-%m-%dT%H:%M:%SZ")" \
  --arg api_url "${API_URL}" \
  --argjson scoreboard "$(printf '%s' "${scoreboard}")" \
  --argjson attacks "$(printf '%s' "${attacks}")" \
  --argjson game_status "$(printf '%s' "${game_status}")" '
  def parse_delta:
    if . == null or . == "" then 0 else (sub("^\\+"; "") | tonumber? // 0) end;
  def absnum:
    if . < 0 then -. else . end;
  def service_rows:
    [ $scoreboard[]? | .services[]? ];
  def service_totals:
    service_rows
    | sort_by(.challenge_id, .service)
    | group_by(.challenge_id)
    | map({
        challenge_id: .[0].challenge_id,
        service: .[0].service,
        attack_total: (map(.attack) | add // 0),
        defense_total: (map(.defense) | add // 0),
        sla_total: (map(.sla) | add // 0),
        total: (map(.total) | add // 0)
      })
    | sort_by(-.attack_total, .challenge_id);
  def elapsed_hours:
    if (($game_status.total_ticks // 0) > 0) and (($game_status.scheduler.interval_seconds // 0) > 0)
    then (($game_status.total_ticks * $game_status.scheduler.interval_seconds) / 3600)
    else null
    end;
  def attack_total:
    ($scoreboard | map(.attack) | add // 0);
  def defense_total:
    ($scoreboard | map(.defense) | add // 0);
  def sla_total:
    ($scoreboard | map(.sla) | add // 0);
  def positive_total:
    (attack_total + sla_total);
  def top_service_attack_share:
    if attack_total <= 0 then null
    else ((service_totals | map(.attack_total) | max // 0) / attack_total)
    end;
  def attack_hhi:
    if attack_total <= 0 then null
    else (service_totals | map((.attack_total / attack_total) * (.attack_total / attack_total)) | add)
    end;
  def attacks_per_hour:
    if elapsed_hours == null or elapsed_hours <= 0 then null
    else (($attacks.total_count // 0) / elapsed_hours)
    end;
  def rank_delta_values:
    ($scoreboard | map(.delta | parse_delta));
  def signals:
    [
      if positive_total > 0 and (attack_total / positive_total) < 0.10
      then "SLA currently dominates positive scoring." else empty end,
      if attacks_per_hour != null and attacks_per_hour < 1
      then "Accepted attacks per hour are currently low." else empty end,
      if top_service_attack_share != null and top_service_attack_share > 0.60
      then "Offense is concentrated in one service." else empty end,
      if (($scoreboard | length) > 0) and ((rank_delta_values | map(select(. != 0)) | length) == 0)
      then "No current rank churn is visible in scoreboard deltas." else empty end
    ];
  {
    generated_at: $generated_at,
    api_url: $api_url,
    match: {
      state: ($game_status.match.state // "unknown"),
      total_ticks: ($game_status.total_ticks // 0),
      scheduler_interval_seconds: ($game_status.scheduler.interval_seconds // 0),
      elapsed_hours: elapsed_hours
    },
    totals: {
      teams: ($scoreboard | length),
      services: ([service_rows[]?.challenge_id] | unique | length),
      attack: attack_total,
      defense: defense_total,
      defense_loss_magnitude: (defense_total | absnum),
      sla: sla_total,
      attack_share_of_positive_points: (if positive_total > 0 then attack_total / positive_total else null end),
      sla_share_of_positive_points: (if positive_total > 0 then sla_total / positive_total else null end),
      attack_to_sla_ratio: (if sla_total > 0 then attack_total / sla_total else null end)
    },
    attacks: {
      accepted_total: ($attacks.total_count // 0),
      accepted_per_hour: attacks_per_hour,
      page_limit: ($attacks.limit // 0)
    },
    rank_churn: {
      teams_with_nonzero_delta: (rank_delta_values | map(select(. != 0)) | length),
      max_abs_delta: (rank_delta_values | map(absnum) | max // 0)
    },
    service_attack_distribution: {
      top_service_attack_share: top_service_attack_share,
      attack_hhi: attack_hhi,
      services: service_totals
    },
    team_snapshot: ($scoreboard | map({
      rank,
      team,
      attack,
      defense,
      sla,
      total,
      delta
    })),
    balance_signals: signals
  }' > "${BALANCE_REPORT_OUTPUT}"

echo "faust balance report written:"
printf '  %s\n' "${BALANCE_REPORT_OUTPUT}"
printf '  %s\n' "accepted attacks: $(jq -r '.attacks.accepted_total' "${BALANCE_REPORT_OUTPUT}")"
printf '  %s\n' "attack share: $(jq -r '.totals.attack_share_of_positive_points // "null"' "${BALANCE_REPORT_OUTPUT}")"
printf '  %s\n' "balance signals: $(jq -r 'if (.balance_signals | length) == 0 then "none" else (.balance_signals | join("; ")) end' "${BALANCE_REPORT_OUTPUT}")"
