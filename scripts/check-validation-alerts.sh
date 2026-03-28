#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${ROOT_DIR}"
source "${ROOT_DIR}/scripts/lib/common.sh"

require_bin jq

artifact_dir="${VALIDATION_ALERT_ARTIFACT_DIR:-}"
summary_file="${VALIDATION_ALERT_SUMMARY_FILE:-}"

usage() {
  cat <<'EOF'
Usage:
  scripts/check-validation-alerts.sh

Optional:
  VALIDATION_ALERT_ARTIFACT_DIR=.runtime/release-candidate-<timestamp>
  VALIDATION_ALERT_SUMMARY_FILE=.runtime/release-candidate-<timestamp>/operator-summary.json
EOF
}

pick_default_artifact_dir() {
  local release_candidate=""
  local go_live=""

  release_candidate="$(find .runtime -maxdepth 1 -mindepth 1 -type d -name 'release-candidate-*' | sort | tail -n 1)"
  if [[ -n "${release_candidate}" ]]; then
    printf '%s\n' "${release_candidate}"
    return 0
  fi

  go_live="$(find .runtime -maxdepth 1 -mindepth 1 -type d -name 'go-live-check-*' | sort | tail -n 1)"
  if [[ -n "${go_live}" ]]; then
    printf '%s\n' "${go_live}"
    return 0
  fi

  return 1
}

if [[ -z "${summary_file}" ]]; then
  if [[ -z "${artifact_dir}" ]]; then
    artifact_dir="$(pick_default_artifact_dir || true)"
  fi
  if [[ -z "${artifact_dir}" || ! -d "${artifact_dir}" ]]; then
    usage >&2
    echo "missing validation artifact directory: ${artifact_dir}" >&2
    exit 1
  fi
  summary_file="${artifact_dir}/operator-summary.json"
  VALIDATION_SUMMARY_ARTIFACT_DIR="${artifact_dir}" \
  VALIDATION_SUMMARY_OUTPUT="${summary_file}" \
    "${ROOT_DIR}/scripts/summarize-validation-artifacts.sh" >/dev/null
fi

if [[ ! -f "${summary_file}" ]]; then
  usage >&2
  echo "missing validation operator summary: ${summary_file}" >&2
  exit 1
fi

if [[ "$(jq -r '.operator_status' "${summary_file}")" == "healthy" ]]; then
  echo "validation alerts check passed:"
  printf '  %s\n' "${summary_file}"
  exit 0
fi

echo "validation alerts require attention:" >&2
printf '  %s\n' "${summary_file}" >&2

runtime_alert_count="$(jq -r '.runtime_alerts.alerts_count // 0' "${summary_file}")"
if [[ "${runtime_alert_count}" != "0" ]]; then
  echo "runtime alerts:" >&2
  jq -r '.runtime_alerts.alerts[] | "  - [\(.severity)] \(.id): \(.summary)\(if (.detail // "") != "" then " " + .detail else "" end)"' "${summary_file}" >&2
fi

derived_alert_count="$(jq -r '.derived_alerts | length' "${summary_file}")"
if [[ "${derived_alert_count}" != "0" ]]; then
  echo "derived alerts:" >&2
  jq -r '.derived_alerts[] | "  - [\(.severity)] \(.id): \(.summary)\(if (.detail // "") != "" then " " + .detail else "" end)"' "${summary_file}" >&2
fi

exit 1
