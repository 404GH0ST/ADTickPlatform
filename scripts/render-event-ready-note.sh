#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${ROOT_DIR}"
source "${ROOT_DIR}/scripts/lib/common.sh"

require_bin jq

ARTIFACT_DIR="${EVENT_READY_ARTIFACT_DIR:-}"
OUTPUT_PATH="${EVENT_READY_OUTPUT:-}"
VALIDATION_DATE="${EVENT_READY_DATE:-}"

usage() {
  cat <<'EOF'
Usage:
  EVENT_READY_OUTPUT=docs/event-ready-YYYY-MM-DD.md \
  EVENT_READY_DATE=YYYY-MM-DD \
  scripts/render-event-ready-note.sh

Optional:
  EVENT_READY_ARTIFACT_DIR=.runtime/release-candidate-<timestamp>
EOF
}

if [[ -z "${OUTPUT_PATH}" || -z "${VALIDATION_DATE}" ]]; then
  usage >&2
  exit 1
fi

if [[ -z "${ARTIFACT_DIR}" ]]; then
  ARTIFACT_DIR="$(find .runtime -maxdepth 1 -mindepth 1 -type d -name 'release-candidate-*' | sort | tail -n 1)"
fi

if [[ ! -d "${ARTIFACT_DIR}" ]]; then
  echo "missing artifact directory: ${ARTIFACT_DIR}" >&2
  exit 1
fi

mkdir -p "$(dirname "${OUTPUT_PATH}")"

summary_file="${ARTIFACT_DIR}/summary.json"
git_revision_file="${ARTIFACT_DIR}/git-revision.txt"
go_live_summary_file="${ARTIFACT_DIR}/go-live-check/summary.json"

if [[ ! -f "${summary_file}" ]]; then
  echo "missing release-candidate summary: ${summary_file}" >&2
  exit 1
fi

if [[ ! -f "${git_revision_file}" ]]; then
  echo "missing git revision file: ${git_revision_file}" >&2
  exit 1
fi

if [[ ! -f "${go_live_summary_file}" ]]; then
  echo "missing go-live summary: ${go_live_summary_file}" >&2
  exit 1
fi

release_commit="$(sed -n 's/^commit=//p' "${git_revision_file}")"
release_short_commit="$(sed -n 's/^short_commit=//p' "${git_revision_file}")"

if [[ -z "${release_commit}" || -z "${release_short_commit}" ]]; then
  echo "could not parse git revision from ${git_revision_file}" >&2
  exit 1
fi

release_rc_dir="$(jq -r '.artifact_dir' "${summary_file}")"
go_live_dir="$(jq -r '.artifacts.go_live_check.summary' "${summary_file}")"
go_live_dir="${go_live_dir%/summary.json}"
go_live_dir="${release_rc_dir}/${go_live_dir}"

cat > "${OUTPUT_PATH}" <<EOF
# Event Ready ${VALIDATION_DATE}

Validation date:

- \`${VALIDATION_DATE}\`

Validated git revision:

- \`${release_short_commit}\` \`${release_commit}\`

Important:

- the latest ${VALIDATION_DATE} validation was rerun from the release-candidate wrapper with machine-readable summaries
- the release candidate should point at commit \`${release_commit}\`

Validated host commands:

\`\`\`bash
make validate-prod-release-candidate
\`\`\`

What passed:

- Host preflight and compose validation for the host-enforcement profile
- Postgres backup and restore drill against the host stack
- Organizer-managed short-match rehearsal
- Host restart recovery drill for \`controller-service\`, \`wireguard-gateway\`, and \`game-core\`
- Attack-map load validation with checker-health validation and zero active runtime alerts after cleanup
- Final host baseline capture after the successful go-live run
- Machine-readable summary emission for the release-candidate and go-live artifact trees

Event-day references:

- \`docs/operator-cheatsheet.md\`
- \`docs/final-rehearsal-checklist.md\`
- \`docs/deployment-host.md\`

Validated artifact directories:

- \`${release_rc_dir}\`
- \`${release_rc_dir}/prod-db-restore\`
- \`${go_live_dir}\`
- \`${go_live_dir}/attack-map-load\`

Expected evidence from the release-candidate run:

- \`${release_rc_dir}/README.txt\`
- \`${release_rc_dir}/summary.json\`
- \`${release_rc_dir}/git-revision.txt\`
- \`${release_rc_dir}/prod-env.sha256\`

Expected evidence from the nested restore drill:

- \`${release_rc_dir}/prod-db-restore/README.txt\`
- \`${release_rc_dir}/prod-db-restore/postgres-backup.sql\`
- \`${release_rc_dir}/prod-db-restore/postgres-backup.sql.sha256\`
- \`${release_rc_dir}/prod-db-restore/pre-restore-game-status.json\`
- \`${release_rc_dir}/prod-db-restore/post-restore-game-status.json\`
- \`${release_rc_dir}/prod-db-restore/pre-restore-scoreboard.json\`
- \`${release_rc_dir}/prod-db-restore/post-restore-scoreboard.json\`
- \`${release_rc_dir}/prod-db-restore/pre-restore-attacks.json\`
- \`${release_rc_dir}/prod-db-restore/post-restore-attacks.json\`

Expected evidence from the nested go-live run:

- \`${go_live_dir}/README.txt\`
- \`${go_live_dir}/summary.json\`
- \`${go_live_dir}/short-match.env\`
- \`${go_live_dir}/operations-status.json\`
- \`${go_live_dir}/git-revision.txt\`
- \`${go_live_dir}/prod-env.sha256\`
- \`${go_live_dir}/final-iptables-filter.txt\`
- \`${go_live_dir}/final-iptables-raw.txt\`
- \`${go_live_dir}/final-nft-ruleset.txt\`
- \`${go_live_dir}/final-wg-show.txt\`
- \`${go_live_dir}/final-compose-ps.txt\`

Expected evidence from the nested attack-map load validation:

- \`${go_live_dir}/attack-map-load/README.txt\`
- \`${go_live_dir}/attack-map-load/attack-map-load.env\`
- \`${go_live_dir}/attack-map-load/attack-map-load-report.json\`
- \`${go_live_dir}/attack-map-load/operations-status.json\`

Release note:

- create the release tag from commit \`${release_commit}\` after committing this readiness note update
EOF

echo "event-ready note rendered:"
printf '  %s\n' "${OUTPUT_PATH}"
