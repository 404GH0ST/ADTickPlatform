#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${ROOT_DIR}"
source "${ROOT_DIR}/scripts/lib/common.sh"

require_bin jq

ARTIFACT_DIR="${RELEASE_CANDIDATE_ARTIFACT_DIR:-}"
NOTE_PATH="${EVENT_READY_NOTE_PATH:-}"
REQUIRE_HEAD_MATCH="${RELEASE_CANDIDATE_REQUIRE_HEAD_MATCH:-true}"
REQUIRE_EVENT_READY_NOTE="${RELEASE_CANDIDATE_REQUIRE_EVENT_READY_NOTE:-false}"

usage() {
  cat <<'EOF'
Usage:
  scripts/verify-release-candidate.sh

Optional:
  RELEASE_CANDIDATE_ARTIFACT_DIR=.runtime/release-candidate-<timestamp>
  EVENT_READY_NOTE_PATH=docs/event-ready-YYYY-MM-DD.md
  RELEASE_CANDIDATE_REQUIRE_HEAD_MATCH=false
  RELEASE_CANDIDATE_REQUIRE_EVENT_READY_NOTE=true
EOF
}

if [[ -z "${ARTIFACT_DIR}" ]]; then
  ARTIFACT_DIR="$(find .runtime -maxdepth 1 -mindepth 1 -type d -name 'release-candidate-*' | sort | tail -n 1)"
fi

if [[ -z "${ARTIFACT_DIR}" || ! -d "${ARTIFACT_DIR}" ]]; then
  usage >&2
  echo "missing release-candidate artifact directory: ${ARTIFACT_DIR}" >&2
  exit 1
fi

summary_file="${ARTIFACT_DIR}/summary.json"
git_revision_file="${ARTIFACT_DIR}/git-revision.txt"

if [[ ! -f "${summary_file}" ]]; then
  echo "missing release-candidate summary: ${summary_file}" >&2
  exit 1
fi

if [[ ! -f "${git_revision_file}" ]]; then
  echo "missing git revision file: ${git_revision_file}" >&2
  exit 1
fi

if [[ "$(jq -r '.validation' "${summary_file}")" != "release-candidate" ]]; then
  echo "unexpected validation type in ${summary_file}" >&2
  exit 1
fi

validated_commit="$(sed -n 's/^commit=//p' "${git_revision_file}")"
validated_short_commit="$(sed -n 's/^short_commit=//p' "${git_revision_file}")"

if [[ -z "${validated_commit}" || -z "${validated_short_commit}" ]]; then
  echo "could not parse validated commit from ${git_revision_file}" >&2
  exit 1
fi

event_ready_rel="$(jq -r '.artifacts.event_ready_note // empty' "${summary_file}")"
event_ready_artifact_file=""
event_ready_filename=""
if [[ -n "${event_ready_rel}" ]]; then
  event_ready_filename="$(basename "${event_ready_rel}")"
  event_ready_artifact_file="${ARTIFACT_DIR}/${event_ready_rel}"
fi

if [[ -z "${event_ready_artifact_file}" || ! -f "${event_ready_artifact_file}" ]]; then
  echo "missing rendered event-ready artifact note for validated commit ${validated_commit}" >&2
  exit 1
fi

if [[ "${REQUIRE_EVENT_READY_NOTE}" == "true" ]]; then
  if [[ -z "${NOTE_PATH}" ]]; then
    if [[ -n "${event_ready_filename}" ]]; then
      NOTE_PATH="docs/${event_ready_filename}"
    else
      NOTE_PATH="$(grep -l "${validated_commit}" docs/event-ready-*.md 2>/dev/null | sort | tail -n 1 || true)"
    fi
  fi

  if [[ -z "${NOTE_PATH}" || ! -f "${NOTE_PATH}" ]]; then
    echo "missing matching checked-in event-ready note for validated commit ${validated_commit}" >&2
    exit 1
  fi
fi

if [[ -n "${NOTE_PATH}" ]]; then
  event_ready_filename="$(basename "${NOTE_PATH}")"
else
  event_ready_filename="$(basename "${event_ready_artifact_file}")"
fi

validation_date="${event_ready_filename#event-ready-}"
validation_date="${validation_date%.md}"

go_live_summary_rel="$(jq -r '.artifacts.go_live_check.summary' "${summary_file}")"
go_live_operations_rel="$(jq -r '.artifacts.go_live_check.operations_status' "${summary_file}")"
attack_map_report_rel="$(jq -r '.artifacts.go_live_check.attack_map_load.report // empty' "${summary_file}")"
go_live_game_core_metrics_rel="$(jq -r '.artifacts.go_live_check.game_core_metrics // empty' "${summary_file}")"
go_live_submission_metrics_rel="$(jq -r '.artifacts.go_live_check.submission_service_metrics // empty' "${summary_file}")"
go_live_controller_metrics_rel="$(jq -r '.artifacts.go_live_check.controller_service_metrics // empty' "${summary_file}")"
go_live_realtime_metrics_rel="$(jq -r '.artifacts.go_live_check.realtime_gateway_metrics // empty' "${summary_file}")"

go_live_summary_file="${ARTIFACT_DIR}/${go_live_summary_rel}"
go_live_operations_file="${ARTIFACT_DIR}/${go_live_operations_rel}"
attack_map_report_file=""
go_live_dir="$(dirname "${go_live_summary_file}")"
go_live_game_core_metrics_file=""
go_live_submission_metrics_file=""
go_live_controller_metrics_file=""
go_live_realtime_metrics_file=""

if [[ -n "${attack_map_report_rel}" ]]; then
  attack_map_report_file="${ARTIFACT_DIR}/${attack_map_report_rel}"
fi
if [[ -n "${go_live_game_core_metrics_rel}" ]]; then
  go_live_game_core_metrics_file="${ARTIFACT_DIR}/${go_live_game_core_metrics_rel}"
fi
if [[ -n "${go_live_submission_metrics_rel}" ]]; then
  go_live_submission_metrics_file="${ARTIFACT_DIR}/${go_live_submission_metrics_rel}"
fi
if [[ -n "${go_live_controller_metrics_rel}" ]]; then
  go_live_controller_metrics_file="${ARTIFACT_DIR}/${go_live_controller_metrics_rel}"
fi
if [[ -n "${go_live_realtime_metrics_rel}" ]]; then
  go_live_realtime_metrics_file="${ARTIFACT_DIR}/${go_live_realtime_metrics_rel}"
fi

if [[ -z "${go_live_game_core_metrics_file}" ]]; then
  go_live_game_core_metrics_file="${go_live_dir}/game-core-metrics.prom"
fi

if [[ -z "${go_live_submission_metrics_file}" ]]; then
  go_live_submission_metrics_file="${go_live_dir}/submission-service-metrics.prom"
fi

if [[ -z "${go_live_controller_metrics_file}" ]]; then
  go_live_controller_metrics_file="${go_live_dir}/controller-service-metrics.prom"
fi

if [[ -z "${go_live_realtime_metrics_file}" ]]; then
  go_live_realtime_metrics_file="${go_live_dir}/realtime-gateway-metrics.prom"
fi

if [[ ! -f "${go_live_summary_file}" ]]; then
  echo "missing go-live summary: ${go_live_summary_file}" >&2
  exit 1
fi

if [[ ! -f "${go_live_operations_file}" ]]; then
  echo "missing go-live operations status: ${go_live_operations_file}" >&2
  exit 1
fi

if [[ "$(jq -r '.validation' "${go_live_summary_file}")" != "go-live-check" ]]; then
  echo "unexpected go-live validation type in ${go_live_summary_file}" >&2
  exit 1
fi

if [[ "$(jq -r '.healthy' "${go_live_operations_file}")" != "true" ]]; then
  echo "go-live operations status is not healthy: ${go_live_operations_file}" >&2
  jq . "${go_live_operations_file}" >&2
  exit 1
fi

if [[ "$(jq -r '.alerts | length' "${go_live_operations_file}")" != "0" ]]; then
  echo "go-live operations status contains active alerts: ${go_live_operations_file}" >&2
  jq . "${go_live_operations_file}" >&2
  exit 1
fi

if [[ -z "${go_live_game_core_metrics_file}" || ! -f "${go_live_game_core_metrics_file}" ]]; then
  echo "missing go-live game-core metrics snapshot: ${go_live_game_core_metrics_file}" >&2
  exit 1
fi

if [[ -z "${go_live_realtime_metrics_file}" || ! -f "${go_live_realtime_metrics_file}" ]]; then
  echo "missing go-live realtime-gateway metrics snapshot: ${go_live_realtime_metrics_file}" >&2
  exit 1
fi

if [[ -z "${go_live_submission_metrics_file}" || ! -f "${go_live_submission_metrics_file}" ]]; then
  echo "missing go-live submission-service metrics snapshot: ${go_live_submission_metrics_file}" >&2
  exit 1
fi

if [[ -z "${go_live_controller_metrics_file}" || ! -f "${go_live_controller_metrics_file}" ]]; then
  echo "missing go-live controller-service metrics snapshot: ${go_live_controller_metrics_file}" >&2
  exit 1
fi

if ! grep -q 'adplatform_game_core_scheduler_running' "${go_live_game_core_metrics_file}"; then
  echo "game-core metrics snapshot is missing scheduler metrics: ${go_live_game_core_metrics_file}" >&2
  exit 1
fi

if ! grep -q 'adplatform_game_core_checker_runs_total' "${go_live_game_core_metrics_file}"; then
  echo "game-core metrics snapshot is missing checker-run metrics: ${go_live_game_core_metrics_file}" >&2
  exit 1
fi

if ! grep -q 'adplatform_realtime_gateway_subscribers' "${go_live_realtime_metrics_file}"; then
  echo "realtime-gateway metrics snapshot is missing subscriber metrics: ${go_live_realtime_metrics_file}" >&2
  exit 1
fi

if ! grep -q 'adplatform_realtime_gateway_last_sync_success' "${go_live_realtime_metrics_file}"; then
  echo "realtime-gateway metrics snapshot is missing sync-health metrics: ${go_live_realtime_metrics_file}" >&2
  exit 1
fi

if ! grep -q 'adplatform_submission_service_submit_requests_total' "${go_live_submission_metrics_file}"; then
  echo "submission-service metrics snapshot is missing submit metrics: ${go_live_submission_metrics_file}" >&2
  exit 1
fi

if ! grep -q 'adplatform_submission_service_attack_feed_requests_total' "${go_live_submission_metrics_file}"; then
  echo "submission-service metrics snapshot is missing attack-feed metrics: ${go_live_submission_metrics_file}" >&2
  exit 1
fi

if ! grep -q 'adplatform_controller_service_operation_requests_total' "${go_live_controller_metrics_file}"; then
  echo "controller-service metrics snapshot is missing operation metrics: ${go_live_controller_metrics_file}" >&2
  exit 1
fi

if ! grep -q 'adplatform_controller_service_access_policies_total' "${go_live_controller_metrics_file}"; then
  echo "controller-service metrics snapshot is missing access status metrics: ${go_live_controller_metrics_file}" >&2
  exit 1
fi

if [[ -n "${attack_map_report_file}" ]]; then
  if [[ ! -f "${attack_map_report_file}" ]]; then
    echo "missing attack-map load report: ${attack_map_report_file}" >&2
    exit 1
  fi

  if [[ "$(jq -r '.validation_status' "${attack_map_report_file}")" != "passed" ]]; then
    echo "attack-map load validation did not pass: ${attack_map_report_file}" >&2
    jq . "${attack_map_report_file}" >&2
    exit 1
  fi
fi

if [[ "${REQUIRE_EVENT_READY_NOTE}" == "true" ]]; then
  EVENT_READY_ARTIFACT_DIR="${ARTIFACT_DIR}" \
  EVENT_READY_NOTE_PATH="${NOTE_PATH}" \
  EVENT_READY_DATE="${validation_date}" \
    "${ROOT_DIR}/scripts/verify-event-ready-note.sh"
fi

current_head="$(git rev-parse HEAD)"
if [[ "${REQUIRE_HEAD_MATCH}" == "true" && "${current_head}" != "${validated_commit}" ]]; then
  echo "git HEAD does not match the validated release-candidate commit" >&2
  printf '  validated: %s (%s)\n' "${validated_short_commit}" "${validated_commit}" >&2
  printf '  current:   %s (%s)\n' "$(git rev-parse --short HEAD)" "${current_head}" >&2
  exit 1
fi

echo "release-candidate verification passed:"
printf '  %s\n' \
  "${ARTIFACT_DIR}" \
  "${event_ready_artifact_file}" \
  "${go_live_operations_file}" \
  "${go_live_game_core_metrics_file}" \
  "${go_live_submission_metrics_file}" \
  "${go_live_controller_metrics_file}" \
  "${go_live_realtime_metrics_file}"
