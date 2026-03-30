#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${ROOT_DIR}"
source "${ROOT_DIR}/scripts/lib/common.sh"

require_bin bunx
require_bin git

PRESET="${RELEASE_NOTES_PRESET:-conventionalcommits}"
MODE="${RELEASE_NOTES_MODE:-latest}"
OUTPUT_PATH="${RELEASE_NOTES_OUTPUT:-}"
TAG_PREFIX="${RELEASE_NOTES_TAG_PREFIX:-v}"
VERSION="${RELEASE_NOTES_VERSION:-}"
CURRENT_TAG="${RELEASE_NOTES_CURRENT_TAG:-}"
PREVIOUS_TAG="${RELEASE_NOTES_PREVIOUS_TAG:-}"

usage() {
  cat <<'EOF'
Usage:
  make release-notes

Optional:
  RELEASE_NOTES_PRESET=conventionalcommits
  RELEASE_NOTES_MODE=latest
  RELEASE_NOTES_OUTPUT=.runtime/release-notes-v1.0.0.md
  RELEASE_NOTES_TAG_PREFIX=v

Examples:
  make release-notes
  RELEASE_NOTES_MODE=all make release-notes
  RELEASE_NOTES_MODE=unreleased make release-notes
  RELEASE_NOTES_OUTPUT=.runtime/release-notes.md make release-notes
EOF
}

if [[ "${1:-}" == "--help" ]]; then
  usage
  exit 0
fi

if [[ -n "${OUTPUT_PATH}" ]]; then
  mkdir -p "$(dirname "${OUTPUT_PATH}")"
fi

case "${MODE}" in
  latest|all|unreleased) ;;
  *)
    echo "unsupported RELEASE_NOTES_MODE: ${MODE}" >&2
    exit 1
    ;;
esac

if [[ -z "${CURRENT_TAG}" ]]; then
  CURRENT_TAG="$(git describe --tags --abbrev=0 2>/dev/null || true)"
fi

if [[ -z "${PREVIOUS_TAG}" && -n "${CURRENT_TAG}" ]]; then
  PREVIOUS_TAG="$(git describe --tags --abbrev=0 "${CURRENT_TAG}^" 2>/dev/null || true)"
fi

if [[ -z "${VERSION}" && -n "${CURRENT_TAG}" ]]; then
  VERSION="${CURRENT_TAG#${TAG_PREFIX}}"
fi

context_file=""
changelog_file=""
trap 'rm -f "${context_file}" "${changelog_file}"' EXIT

if [[ -n "${VERSION}" || -n "${CURRENT_TAG}" || -n "${PREVIOUS_TAG}" ]]; then
  context_file="$(mktemp "${TMPDIR:-/tmp}/release-notes-context.XXXXXX.json")"
  cat > "${context_file}" <<EOF
{
  "version": "${VERSION}",
  "currentTag": "${CURRENT_TAG}",
  "previousTag": "${PREVIOUS_TAG}"
}
EOF
fi

cmd=(
  bunx conventional-changelog
  --preset "${PRESET}"
  --release-count 0
  --pkg "${ROOT_DIR}/package.json"
  --tag-prefix "${TAG_PREFIX}"
)

if [[ "${MODE}" == "unreleased" ]]; then
  cmd+=(--output-unreleased)
fi

if [[ -n "${context_file}" ]]; then
  cmd+=(--context "${context_file}")
fi

changelog_file="$(mktemp "${TMPDIR:-/tmp}/release-notes-output.XXXXXX.md")"
cmd+=(--outfile "${changelog_file}")

"${cmd[@]}"

case "${MODE}" in
  latest)
    awk '
      BEGIN { printing = 0; found = 0 }
      /^## / {
        if (found) {
          exit
        }
        printing = 1
        found = 1
      }
      printing {
        print
      }
    ' "${changelog_file}" > "${changelog_file}.latest"
    mv "${changelog_file}.latest" "${changelog_file}"
    ;;
  all|unreleased)
    ;;
esac

if [[ -n "${OUTPUT_PATH}" ]]; then
  cp "${changelog_file}" "${OUTPUT_PATH}"
  echo "release notes written:"
  printf '  %s\n' "${OUTPUT_PATH}"
else
  cat "${changelog_file}"
fi
