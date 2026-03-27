#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${ROOT_DIR}"
source "${ROOT_DIR}/scripts/lib/common.sh"

ARTIFACT_DIR="${EVENT_READY_ARTIFACT_DIR:-}"
VALIDATION_DATE="${EVENT_READY_DATE:-}"
NOTE_PATH="${EVENT_READY_NOTE_PATH:-}"

usage() {
  cat <<'EOF'
Usage:
  EVENT_READY_DATE=YYYY-MM-DD \
  scripts/verify-event-ready-note.sh

Optional:
  EVENT_READY_ARTIFACT_DIR=.runtime/release-candidate-<timestamp>
  EVENT_READY_NOTE_PATH=docs/event-ready-YYYY-MM-DD.md
EOF
}

if [[ -z "${VALIDATION_DATE}" ]]; then
  usage >&2
  exit 1
fi

if [[ -z "${NOTE_PATH}" ]]; then
  NOTE_PATH="docs/event-ready-${VALIDATION_DATE}.md"
fi

if [[ ! -f "${NOTE_PATH}" ]]; then
  echo "missing event-ready note: ${NOTE_PATH}" >&2
  exit 1
fi

tmp_render="$(mktemp "${TMPDIR:-/tmp}/event-ready-note.XXXXXX.md")"
trap 'rm -f "${tmp_render}"' EXIT

EVENT_READY_ARTIFACT_DIR="${ARTIFACT_DIR}" \
EVENT_READY_OUTPUT="${tmp_render}" \
EVENT_READY_DATE="${VALIDATION_DATE}" \
  "${ROOT_DIR}/scripts/render-event-ready-note.sh" >/dev/null

if ! diff -u "${NOTE_PATH}" "${tmp_render}"; then
  echo "event-ready note mismatch: ${NOTE_PATH}" >&2
  exit 1
fi

echo "event-ready note verified:"
printf '  %s\n' "${NOTE_PATH}"
