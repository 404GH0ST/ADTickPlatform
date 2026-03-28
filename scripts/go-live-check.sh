#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${ROOT_DIR}"
source "${ROOT_DIR}/scripts/lib/common.sh"

require_bin curl
require_bin jq

timestamp="$(date -u +%Y%m%dT%H%M%SZ)"
artifact_dir="${GO_LIVE_CHECK_OUTPUT_DIR:-.runtime/go-live-check-${timestamp}}"
match_artifact_file="${artifact_dir}/short-match.env"
attack_map_load_output_dir="${artifact_dir}/attack-map-load"
prod_env="${PROD_ENV:-deploy/compose/prod.env}"
include_attack_map_load="${GO_LIVE_CHECK_INCLUDE_ATTACK_MAP_LOAD:-true}"
operations_status_file="${artifact_dir}/operations-status.json"
game_core_metrics_file="${artifact_dir}/game-core-metrics.prom"
submission_metrics_file="${artifact_dir}/submission-service-metrics.prom"
controller_metrics_file="${artifact_dir}/controller-service-metrics.prom"
realtime_metrics_file="${artifact_dir}/realtime-gateway-metrics.prom"
summary_file="${artifact_dir}/summary.json"

load_env_file "${prod_env}"
edge_base_url="${GO_LIVE_CHECK_BASE_URL:-$(derive_edge_base_url)}"

mkdir -p "${artifact_dir}"

echo "go-live check: artifacts=${artifact_dir} edge=${edge_base_url}"

ORGANIZER_SMOKE_ARTIFACT_FILE="${match_artifact_file}" "${ROOT_DIR}/scripts/smoke-prod-short-match.sh"
load_env_file_override "${match_artifact_file}"
export AD_PLATFORM_EMAIL="${SMOKE_TEAM_ONE_PLAYER_EMAIL}"
export AD_PLATFORM_PASSWORD="${SMOKE_TEAM_ONE_PASSWORD}"
export AD_PLATFORM_TEAM_ID="${SMOKE_TEAM_ONE_ID}"
"${ROOT_DIR}/scripts/smoke-prod-host-recovery.sh"

if [[ "${include_attack_map_load}" == "true" ]]; then
  ATTACK_MAP_LOAD_OUTPUT_DIR="${attack_map_load_output_dir}" "${ROOT_DIR}/scripts/validate-attack-map-load.sh"
fi

BASELINE_OUTPUT_DIR="${artifact_dir}" "${ROOT_DIR}/scripts/capture-prod-host-baseline.sh"
GO_LIVE_METRICS_OUTPUT_DIR="${artifact_dir}" "${ROOT_DIR}/scripts/capture-go-live-metrics.sh"

curl -fsS -H "Authorization: Bearer ${ADMIN_API_TOKEN}" \
  "${edge_base_url}/api/v2/admin/operations/status" | jq '.data' > "${operations_status_file}"
if ! jq -e '.healthy == true and ((.alerts | length) == 0)' "${operations_status_file}" >/dev/null; then
  echo "go-live check finished with active runtime alerts" >&2
  cat "${operations_status_file}" >&2
  exit 1
fi

{
  echo "commit=$(git rev-parse HEAD)"
  echo "short_commit=$(git rev-parse --short HEAD)"
  echo "tag=$(git describe --tags --exact-match 2>/dev/null || true)"
  echo "generated_at=${timestamp}"
} > "${artifact_dir}/git-revision.txt"

if command -v sha256sum >/dev/null 2>&1 && [[ -f "${prod_env}" ]]; then
  sha256sum "${prod_env}" > "${artifact_dir}/prod-env.sha256"
fi

attack_map_command=""
attack_map_artifacts=""
if [[ "${include_attack_map_load}" == "true" ]]; then
  attack_map_command='- make validate-attack-map-load'
  attack_map_artifacts='- attack-map-load/README.txt
- attack-map-load/attack-map-load.env
- attack-map-load/attack-map-load-report.json'
fi

cat > "${artifact_dir}/README.txt" <<EOF
Go-live check completed at ${timestamp}.

Commands run:
- make smoke-prod-short-match
- make smoke-prod-host-recovery
${attack_map_command}
- make capture-prod-host-baseline

Artifacts in this directory:
- short-match.env
${attack_map_artifacts}
- operations-status.json
- git-revision.txt
- prod-env.sha256
- game-core-metrics.prom
- submission-service-metrics.prom
- controller-service-metrics.prom
- realtime-gateway-metrics.prom
- final-iptables-filter.txt
- final-iptables-raw.txt
- final-nft-ruleset.txt
- final-wg-show.txt
- final-compose-ps.txt
EOF

jq -nc \
  --arg generated_at "${timestamp}" \
  --arg edge_base_url "${edge_base_url}" \
  --arg artifact_dir "${artifact_dir}" \
  --arg short_match_env "short-match.env" \
  --arg operations_status "operations-status.json" \
  --arg git_revision "git-revision.txt" \
  --arg prod_env_sha256 "prod-env.sha256" \
  --arg game_core_metrics "game-core-metrics.prom" \
  --arg submission_metrics "submission-service-metrics.prom" \
  --arg controller_metrics "controller-service-metrics.prom" \
  --arg realtime_metrics "realtime-gateway-metrics.prom" \
  --arg final_iptables_filter "final-iptables-filter.txt" \
  --arg final_iptables_raw "final-iptables-raw.txt" \
  --arg final_nft_ruleset "final-nft-ruleset.txt" \
  --arg final_wg_show "final-wg-show.txt" \
  --arg final_compose_ps "final-compose-ps.txt" \
  --argjson include_attack_map_load "$( [[ "${include_attack_map_load}" == "true" ]] && printf 'true' || printf 'false' )" \
  '{
    validation: "go-live-check",
    generated_at: $generated_at,
    edge_base_url: $edge_base_url,
    artifact_dir: $artifact_dir,
    include_attack_map_load: $include_attack_map_load,
    commands: [
      "make smoke-prod-short-match",
      "make smoke-prod-host-recovery",
      "make capture-prod-host-baseline"
    ] + (if $include_attack_map_load then ["make validate-attack-map-load"] else [] end),
    artifacts: {
      short_match_env: $short_match_env,
      operations_status: $operations_status,
      git_revision: $git_revision,
      prod_env_sha256: $prod_env_sha256,
      game_core_metrics: $game_core_metrics,
      submission_service_metrics: $submission_metrics,
      controller_service_metrics: $controller_metrics,
      realtime_gateway_metrics: $realtime_metrics,
      final_iptables_filter: $final_iptables_filter,
      final_iptables_raw: $final_iptables_raw,
      final_nft_ruleset: $final_nft_ruleset,
      final_wg_show: $final_wg_show,
      final_compose_ps: $final_compose_ps,
      attack_map_load: (if $include_attack_map_load then {
        readme: "attack-map-load/README.txt",
        env: "attack-map-load/attack-map-load.env",
        report: "attack-map-load/attack-map-load-report.json",
        operations_status: "attack-map-load/operations-status.json"
      } else null end)
    }
  }' > "${summary_file}"

echo "go-live check passed:"
printf '  %s\n' \
  "${artifact_dir}" \
  "${artifact_dir}/README.txt" \
  "${summary_file}"
