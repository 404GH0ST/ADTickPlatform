#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${ROOT_DIR}"
source "${ROOT_DIR}/scripts/lib/common.sh"

timestamp="$(date -u +%Y%m%dT%H%M%SZ)"
artifact_dir="${RELEASE_CANDIDATE_OUTPUT_DIR:-.runtime/release-candidate-${timestamp}}"
prod_env="${PROD_ENV:-deploy/compose/prod.env}"

mkdir -p "${artifact_dir}"

echo "release-candidate validation: artifacts=${artifact_dir}"

"${ROOT_DIR}/scripts/preflight-prod-host.sh"
docker compose --env-file "${prod_env}" -f deploy/compose/prod.yml config >/dev/null
docker compose --env-file "${prod_env}" -f deploy/compose/prod.yml -f "${PROD_HOST_OVERRIDE:-deploy/compose/prod.host-enforcement.yml}" config >/dev/null

RESTORE_DRILL_OUTPUT_DIR="${artifact_dir}/prod-db-restore" "${ROOT_DIR}/scripts/smoke-prod-db-restore.sh"
GO_LIVE_CHECK_OUTPUT_DIR="${artifact_dir}/go-live-check" "${ROOT_DIR}/scripts/go-live-check.sh"

{
  echo "commit=$(git rev-parse HEAD)"
  echo "short_commit=$(git rev-parse --short HEAD)"
  echo "tag=$(git describe --tags --exact-match 2>/dev/null || true)"
  echo "generated_at=${timestamp}"
} > "${artifact_dir}/git-revision.txt"

if command -v sha256sum >/dev/null 2>&1 && [[ -f "${prod_env}" ]]; then
  sha256sum "${prod_env}" > "${artifact_dir}/prod-env.sha256"
fi

cat > "${artifact_dir}/README.txt" <<EOF
Release-candidate validation completed at ${timestamp}.

Commands run:
- make preflight-prod-host
- make prod-config
- make prod-host-config
- make smoke-prod-db-restore
- make go-live-check

Artifacts in this directory:
- prod-db-restore/README.txt
- prod-db-restore/postgres-backup.sql
- prod-db-restore/postgres-backup.sql.sha256
- go-live-check/README.txt
- go-live-check/short-match.env
- go-live-check/attack-map-load/README.txt
- go-live-check/attack-map-load/attack-map-load.env
- go-live-check/attack-map-load/attack-map-load-report.json
- git-revision.txt
- prod-env.sha256
EOF

echo "release-candidate validation passed:"
printf '  %s\n' \
  "${artifact_dir}" \
  "${artifact_dir}/README.txt"
