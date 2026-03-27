#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${ROOT_DIR}"
source "${ROOT_DIR}/scripts/lib/common.sh"

timestamp="$(date -u +%Y%m%dT%H%M%SZ)"
artifact_dir="${GO_LIVE_CHECK_OUTPUT_DIR:-.runtime/go-live-check-${timestamp}}"
match_artifact_file="${artifact_dir}/short-match.env"
attack_map_load_output_dir="${artifact_dir}/attack-map-load"
prod_env="${PROD_ENV:-deploy/compose/prod.env}"
include_attack_map_load="${GO_LIVE_CHECK_INCLUDE_ATTACK_MAP_LOAD:-true}"

mkdir -p "${artifact_dir}"

echo "go-live check: artifacts=${artifact_dir}"

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
- git-revision.txt
- prod-env.sha256
- final-iptables-filter.txt
- final-iptables-raw.txt
- final-nft-ruleset.txt
- final-wg-show.txt
- final-compose-ps.txt
EOF

echo "go-live check passed:"
printf '  %s\n' \
  "${artifact_dir}" \
  "${artifact_dir}/README.txt"
