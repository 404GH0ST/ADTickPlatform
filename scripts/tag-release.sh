#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${ROOT_DIR}"
source "${ROOT_DIR}/scripts/lib/common.sh"

ARTIFACT_DIR="${RELEASE_CANDIDATE_ARTIFACT_DIR:-}"
NOTE_PATH="${EVENT_READY_NOTE_PATH:-}"
RELEASE_TAG="${RELEASE_TAG:-}"
DRY_RUN="${RELEASE_TAG_DRY_RUN:-false}"

usage() {
  cat <<'EOF'
Usage:
  RELEASE_TAG=vYYYY.MM.DD \
  scripts/tag-release.sh

Optional:
  RELEASE_CANDIDATE_ARTIFACT_DIR=.runtime/release-candidate-<timestamp>
  EVENT_READY_NOTE_PATH=docs/event-ready-YYYY-MM-DD.md
  RELEASE_TAG_DRY_RUN=true
EOF
}

if [[ -z "${RELEASE_TAG}" ]]; then
  usage >&2
  exit 1
fi

if [[ -z "${ARTIFACT_DIR}" ]]; then
  ARTIFACT_DIR="$(find .runtime -maxdepth 1 -mindepth 1 -type d -name 'release-candidate-*' | sort | tail -n 1)"
fi

if [[ -z "${ARTIFACT_DIR}" || ! -d "${ARTIFACT_DIR}" ]]; then
  echo "missing release-candidate artifact directory: ${ARTIFACT_DIR}" >&2
  exit 1
fi

if [[ -z "${NOTE_PATH}" ]]; then
  note_name="$(find docs -maxdepth 1 -type f -name 'event-ready-*.md' | sort | tail -n 1)"
  NOTE_PATH="${note_name}"
fi

if [[ -z "${NOTE_PATH}" || ! -f "${NOTE_PATH}" ]]; then
  echo "missing event-ready note: ${NOTE_PATH}" >&2
  exit 1
fi

RELEASE_CANDIDATE_ARTIFACT_DIR="${ARTIFACT_DIR}" \
EVENT_READY_NOTE_PATH="${NOTE_PATH}" \
RELEASE_CANDIDATE_REQUIRE_EVENT_READY_NOTE=true \
  "${ROOT_DIR}/scripts/verify-release-candidate.sh"

git_revision_file="${ARTIFACT_DIR}/git-revision.txt"
validated_commit="$(sed -n 's/^commit=//p' "${git_revision_file}")"
validated_short_commit="$(sed -n 's/^short_commit=//p' "${git_revision_file}")"

if [[ -z "${validated_commit}" || -z "${validated_short_commit}" ]]; then
  echo "could not parse validated commit from ${git_revision_file}" >&2
  exit 1
fi

if git rev-parse -q --verify "refs/tags/${RELEASE_TAG}" >/dev/null 2>&1; then
  echo "tag already exists: ${RELEASE_TAG}" >&2
  exit 1
fi

validation_date="$(basename "${NOTE_PATH}")"
validation_date="${validation_date#event-ready-}"
validation_date="${validation_date%.md}"

tag_message_file="$(mktemp "${TMPDIR:-/tmp}/release-tag-message.XXXXXX.txt")"
trap 'rm -f "${tag_message_file}"' EXIT

cat > "${tag_message_file}" <<EOF
Release ${RELEASE_TAG}

Validated commit: ${validated_commit}
Validation date: ${validation_date}
Release-candidate artifact: ${ARTIFACT_DIR}
Event-ready note: ${NOTE_PATH}
EOF

if [[ "${DRY_RUN}" == "true" ]]; then
  echo "release tag dry-run passed:"
  printf '  %s\n' \
    "tag=${RELEASE_TAG}" \
    "commit=${validated_short_commit} (${validated_commit})" \
    "artifact_dir=${ARTIFACT_DIR}" \
    "event_ready_note=${NOTE_PATH}"
  exit 0
fi

git tag -a "${RELEASE_TAG}" "${validated_commit}" -F "${tag_message_file}"

echo "release tag created:"
printf '  %s\n' \
  "${RELEASE_TAG}" \
  "${validated_short_commit} (${validated_commit})"
