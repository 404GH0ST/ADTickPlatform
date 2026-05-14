#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${ROOT_DIR}"
source "${ROOT_DIR}/scripts/lib/common.sh"

require_bin jq

artifact_dir="${VALIDATION_SUMMARY_ARTIFACT_DIR:-}"
output_file="${VALIDATION_SUMMARY_OUTPUT:-}"

usage() {
  cat <<'EOF'
Usage:
  scripts/summarize-validation-artifacts.sh

Optional:
  VALIDATION_SUMMARY_ARTIFACT_DIR=.runtime/release-candidate-<timestamp>
  VALIDATION_SUMMARY_OUTPUT=.runtime/release-candidate-<timestamp>/operator-summary.json
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

metric_value() {
  local file="$1"
  local metric="$2"
  local labels="${3:-}"

  if [[ ! -f "${file}" ]]; then
    return 0
  fi

  if [[ -n "${labels}" ]]; then
    grep -F "${metric}{${labels}}" "${file}" | awk '{print $2}' | tail -n 1
    return 0
  fi

  grep -E "^${metric}( |$)" "${file}" | awk '{print $2}' | tail -n 1
}

metric_sum_by_prefix() {
  local file="$1"
  local metric="$2"

  if [[ ! -f "${file}" ]]; then
    printf '0\n'
    return 0
  fi

  awk -v metric="${metric}" '
    index($1, metric "{") == 1 { sum += $2 }
    END { printf "%.0f\n", sum + 0 }
  ' "${file}"
}

json_number_or_null() {
  local value="${1:-}"
  if [[ -z "${value}" ]]; then
    printf 'null'
  else
    printf '%s' "${value}"
  fi
}

if [[ -z "${artifact_dir}" ]]; then
  artifact_dir="$(pick_default_artifact_dir || true)"
fi

if [[ -z "${artifact_dir}" || ! -d "${artifact_dir}" ]]; then
  usage >&2
  echo "missing validation artifact directory: ${artifact_dir}" >&2
  exit 1
fi

summary_file="${artifact_dir}/summary.json"
if [[ ! -f "${summary_file}" ]]; then
  usage >&2
  echo "missing summary file: ${summary_file}" >&2
  exit 1
fi

validation_type="$(jq -r '.validation' "${summary_file}")"
go_live_dir="${artifact_dir}"
go_live_summary_file="${summary_file}"

if [[ "${validation_type}" == "release-candidate" ]]; then
  go_live_summary_file="${artifact_dir}/$(jq -r '.artifacts.go_live_check.summary' "${summary_file}")"
  go_live_dir="$(dirname "${go_live_summary_file}")"
fi

if [[ ! -f "${go_live_summary_file}" ]]; then
  echo "missing go-live summary file: ${go_live_summary_file}" >&2
  exit 1
fi

go_live_operations_file="${go_live_dir}/$(jq -r '.artifacts.operations_status' "${go_live_summary_file}")"
scoring_audit_rel="$(jq -r '.artifacts.scoring_audit // empty' "${go_live_summary_file}")"
scoring_audit_file=""
attack_map_report_rel="$(jq -r '.artifacts.attack_map_load.report // empty' "${go_live_summary_file}")"
attack_map_report_file=""
if [[ -n "${scoring_audit_rel}" ]]; then
  scoring_audit_file="${go_live_dir}/${scoring_audit_rel}"
elif [[ -f "${go_live_dir}/scoring-audit.json" ]]; then
  scoring_audit_file="${go_live_dir}/scoring-audit.json"
fi
if [[ -n "${attack_map_report_rel}" ]]; then
  attack_map_report_file="${go_live_dir}/${attack_map_report_rel}"
fi

game_core_metrics_file="${go_live_dir}/$(jq -r '.artifacts.game_core_metrics // "game-core-metrics.prom"' "${go_live_summary_file}")"
submission_metrics_file="${go_live_dir}/$(jq -r '.artifacts.submission_service_metrics // "submission-service-metrics.prom"' "${go_live_summary_file}")"
controller_metrics_file="${go_live_dir}/$(jq -r '.artifacts.controller_service_metrics // "controller-service-metrics.prom"' "${go_live_summary_file}")"
realtime_metrics_file="${go_live_dir}/$(jq -r '.artifacts.realtime_gateway_metrics // "realtime-gateway-metrics.prom"' "${go_live_summary_file}")"
wireguard_metrics_file="${go_live_dir}/$(jq -r '.artifacts.wireguard_gateway_metrics // "wireguard-gateway-metrics.prom"' "${go_live_summary_file}")"

git_revision_file="${artifact_dir}/git-revision.txt"
validated_commit="$(sed -n 's/^commit=//p' "${git_revision_file}" 2>/dev/null || true)"
validated_short_commit="$(sed -n 's/^short_commit=//p' "${git_revision_file}" 2>/dev/null || true)"

operations_healthy="$(jq -r '.healthy // false' "${go_live_operations_file}")"
operations_alerts_count="$(jq -r '.alerts | length' "${go_live_operations_file}")"

scoring_audit_status=""
scoring_audit_stored_rows=""
scoring_audit_replayed_rows=""
scoring_audit_mismatch_count=""
if [[ -n "${scoring_audit_file}" && -f "${scoring_audit_file}" ]]; then
  scoring_audit_status="$(jq -r '.status // empty' "${scoring_audit_file}")"
  scoring_audit_stored_rows="$(jq -r '.stored_rows // empty' "${scoring_audit_file}")"
  scoring_audit_replayed_rows="$(jq -r '.replayed_rows // empty' "${scoring_audit_file}")"
  scoring_audit_mismatch_count="$(jq -r '.mismatch_count // empty' "${scoring_audit_file}")"
fi

attack_map_validation_status=""
attack_map_submissions_per_second=""
attack_map_submission_p95_ms=""
attack_map_attack_feed_lag_ms=""
if [[ -n "${attack_map_report_file}" && -f "${attack_map_report_file}" ]]; then
  attack_map_validation_status="$(jq -r '.validation_status // empty' "${attack_map_report_file}")"
  attack_map_submissions_per_second="$(jq -r '.submissions_per_second // empty' "${attack_map_report_file}")"
  attack_map_submission_p95_ms="$(jq -r '.latencies_ms.submission.p95 // empty' "${attack_map_report_file}")"
  attack_map_attack_feed_lag_ms="$(jq -r '.latencies_ms.attack_feed_visibility // empty' "${attack_map_report_file}")"
fi

game_core_total_ticks="$(metric_value "${game_core_metrics_file}" "adplatform_game_core_total_ticks")"
game_core_checker_runs_total="$(metric_value "${game_core_metrics_file}" "adplatform_game_core_checker_runs_total" 'status="all"')"
game_core_checker_runs_failed="$(metric_value "${game_core_metrics_file}" "adplatform_game_core_checker_runs_total" 'status="failed"')"
game_core_scheduler_running="$(metric_value "${game_core_metrics_file}" "adplatform_game_core_scheduler_running")"
game_core_match_not_started="$(metric_value "${game_core_metrics_file}" "adplatform_game_core_match_state" 'state="not_started"')"
game_core_match_running="$(metric_value "${game_core_metrics_file}" "adplatform_game_core_match_state" 'state="running"')"
game_core_match_finished="$(metric_value "${game_core_metrics_file}" "adplatform_game_core_match_state" 'state="finished"')"
game_core_match_state="unknown"
if [[ "${game_core_match_running}" == "1" ]]; then
  game_core_match_state="running"
elif [[ "${game_core_match_finished}" == "1" ]]; then
  game_core_match_state="finished"
elif [[ "${game_core_match_not_started}" == "1" ]]; then
  game_core_match_state="not_started"
fi

submission_requests_total="$(metric_value "${submission_metrics_file}" "adplatform_submission_service_submit_requests_total")"
submission_failures_total="$(metric_value "${submission_metrics_file}" "adplatform_submission_service_submit_failures_total")"
submission_attack_feed_requests_total="$(metric_value "${submission_metrics_file}" "adplatform_submission_service_attack_feed_requests_total")"
submission_verdict_correct="$(metric_value "${submission_metrics_file}" "adplatform_submission_service_submit_verdicts_total" 'class="correct"')"
submission_verdict_duplicate="$(metric_value "${submission_metrics_file}" "adplatform_submission_service_submit_verdicts_total" 'class="duplicate"')"
submission_verdict_invalid="$(metric_value "${submission_metrics_file}" "adplatform_submission_service_submit_verdicts_total" 'class="invalid"')"

controller_deployment_reconcile_requests="$(metric_value "${controller_metrics_file}" "adplatform_controller_service_operation_requests_total" 'operation="deployment_reconcile"')"
controller_access_reconcile_requests="$(metric_value "${controller_metrics_file}" "adplatform_controller_service_operation_requests_total" 'operation="access_reconcile"')"
controller_service_access_reconcile_requests="$(metric_value "${controller_metrics_file}" "adplatform_controller_service_operation_requests_total" 'operation="service_access_reconcile"')"
controller_ssh_credential_requests="$(metric_value "${controller_metrics_file}" "adplatform_controller_service_operation_requests_total" 'operation="ssh_credential"')"
controller_access_policies_total="$(metric_value "${controller_metrics_file}" "adplatform_controller_service_access_policies_total")"
controller_access_last_apply_success="$(metric_value "${controller_metrics_file}" "adplatform_controller_service_access_last_apply_success")"

realtime_last_sync_success="$(metric_value "${realtime_metrics_file}" "adplatform_realtime_gateway_last_sync_success")"
realtime_sync_errors_total="$(metric_value "${realtime_metrics_file}" "adplatform_realtime_gateway_sync_errors_total")"
realtime_subscribers_total="$(metric_sum_by_prefix "${realtime_metrics_file}" "adplatform_realtime_gateway_subscribers")"

wireguard_reconcile_requests="$(metric_value "${wireguard_metrics_file}" "adplatform_wireguard_gateway_operation_requests_total" 'operation="reconcile"')"
wireguard_peer_total="$(metric_value "${wireguard_metrics_file}" "adplatform_wireguard_gateway_peer_counts" 'status="total"')"
wireguard_peer_active="$(metric_value "${wireguard_metrics_file}" "adplatform_wireguard_gateway_peer_counts" 'status="active"')"
wireguard_peer_revoked="$(metric_value "${wireguard_metrics_file}" "adplatform_wireguard_gateway_peer_counts" 'status="revoked"')"
wireguard_last_apply_success="$(metric_value "${wireguard_metrics_file}" "adplatform_wireguard_gateway_last_apply_success")"

summary_json="$(
  jq -nc \
    --arg validation "${validation_type}" \
    --arg artifact_dir "${artifact_dir}" \
    --arg go_live_artifact_dir "${go_live_dir}" \
    --arg generated_at "$(jq -r '.generated_at' "${summary_file}")" \
    --arg validated_commit "${validated_commit}" \
    --arg validated_short_commit "${validated_short_commit}" \
    --arg operations_healthy "${operations_healthy}" \
    --argjson operations_alerts_count "$(json_number_or_null "${operations_alerts_count}")" \
    --arg scoring_audit_status "${scoring_audit_status}" \
    --argjson scoring_audit_stored_rows "$(json_number_or_null "${scoring_audit_stored_rows}")" \
    --argjson scoring_audit_replayed_rows "$(json_number_or_null "${scoring_audit_replayed_rows}")" \
    --argjson scoring_audit_mismatch_count "$(json_number_or_null "${scoring_audit_mismatch_count}")" \
    --arg attack_map_validation_status "${attack_map_validation_status}" \
    --arg game_core_match_state "${game_core_match_state}" \
    --argjson game_core_total_ticks "$(json_number_or_null "${game_core_total_ticks}")" \
    --argjson game_core_checker_runs_total "$(json_number_or_null "${game_core_checker_runs_total}")" \
    --argjson game_core_checker_runs_failed "$(json_number_or_null "${game_core_checker_runs_failed}")" \
    --argjson game_core_scheduler_running "$(json_number_or_null "${game_core_scheduler_running}")" \
    --argjson submission_requests_total "$(json_number_or_null "${submission_requests_total}")" \
    --argjson submission_failures_total "$(json_number_or_null "${submission_failures_total}")" \
    --argjson submission_attack_feed_requests_total "$(json_number_or_null "${submission_attack_feed_requests_total}")" \
    --argjson submission_verdict_correct "$(json_number_or_null "${submission_verdict_correct}")" \
    --argjson submission_verdict_duplicate "$(json_number_or_null "${submission_verdict_duplicate}")" \
    --argjson submission_verdict_invalid "$(json_number_or_null "${submission_verdict_invalid}")" \
    --argjson controller_deployment_reconcile_requests "$(json_number_or_null "${controller_deployment_reconcile_requests}")" \
    --argjson controller_access_reconcile_requests "$(json_number_or_null "${controller_access_reconcile_requests}")" \
    --argjson controller_service_access_reconcile_requests "$(json_number_or_null "${controller_service_access_reconcile_requests}")" \
    --argjson controller_ssh_credential_requests "$(json_number_or_null "${controller_ssh_credential_requests}")" \
    --argjson controller_access_policies_total "$(json_number_or_null "${controller_access_policies_total}")" \
    --argjson controller_access_last_apply_success "$(json_number_or_null "${controller_access_last_apply_success}")" \
    --argjson realtime_last_sync_success "$(json_number_or_null "${realtime_last_sync_success}")" \
    --argjson realtime_sync_errors_total "$(json_number_or_null "${realtime_sync_errors_total}")" \
    --argjson realtime_subscribers_total "$(json_number_or_null "${realtime_subscribers_total}")" \
    --argjson wireguard_reconcile_requests "$(json_number_or_null "${wireguard_reconcile_requests}")" \
    --argjson wireguard_peer_total "$(json_number_or_null "${wireguard_peer_total}")" \
    --argjson wireguard_peer_active "$(json_number_or_null "${wireguard_peer_active}")" \
    --argjson wireguard_peer_revoked "$(json_number_or_null "${wireguard_peer_revoked}")" \
    --argjson wireguard_last_apply_success "$(json_number_or_null "${wireguard_last_apply_success}")" \
    --argjson attack_map_submissions_per_second "$(json_number_or_null "${attack_map_submissions_per_second}")" \
    --argjson attack_map_submission_p95_ms "$(json_number_or_null "${attack_map_submission_p95_ms}")" \
    --argjson attack_map_attack_feed_lag_ms "$(json_number_or_null "${attack_map_attack_feed_lag_ms}")" \
    --slurpfile operations_status "${go_live_operations_file}" \
    'def derived_alerts:
      ( []
        + (if $attack_map_validation_status != "" and $attack_map_validation_status != "passed" then [{
            id: "attack-map-load-validation",
            severity: "critical",
            summary: "Attack-map load validation did not pass.",
            detail: ("Status was " + $attack_map_validation_status + ".")
          }] else [] end)
        + (if $scoring_audit_status != "" and $scoring_audit_status != "ok" then [{
            id: "scoring-audit-status",
            severity: "critical",
            summary: "Scoring replay audit did not pass.",
            detail: ("Status was " + $scoring_audit_status + ".")
          }] else [] end)
        + (if $scoring_audit_mismatch_count != null and $scoring_audit_mismatch_count > 0 then [{
            id: "scoring-audit-mismatches",
            severity: "critical",
            summary: "Scoring replay audit found scoreboard mismatches.",
            detail: ("Mismatches: " + ($scoring_audit_mismatch_count | tostring) + ".")
          }] else [] end)
        + (if $game_core_checker_runs_failed != null and $game_core_checker_runs_failed > 0 then [{
            id: "checker-failures",
            severity: "critical",
            summary: "Checker failures were recorded in the validated run.",
            detail: ("Failed checker runs: " + ($game_core_checker_runs_failed | tostring) + ".")
          }] else [] end)
        + (if $submission_failures_total != null and $submission_failures_total > 0 then [{
            id: "submission-failures",
            severity: "warning",
            summary: "Submission-service reported failed submit requests.",
            detail: ("Failed submit requests: " + ($submission_failures_total | tostring) + ".")
          }] else [] end)
        + (if $realtime_last_sync_success != null and $realtime_last_sync_success != 1 then [{
            id: "realtime-last-sync-failed",
            severity: "warning",
            summary: "Realtime gateway last sync did not report success."
          }] else [] end)
        + (if $realtime_sync_errors_total != null and $realtime_sync_errors_total > 0 then [{
            id: "realtime-sync-errors",
            severity: "warning",
            summary: "Realtime gateway reported sync errors.",
            detail: ("Sync errors: " + ($realtime_sync_errors_total | tostring) + ".")
          }] else [] end)
        + (if $controller_access_last_apply_success != null and $controller_access_last_apply_success != 1 then [{
            id: "controller-access-apply-failed",
            severity: "warning",
            summary: "Controller access status did not report a successful last apply."
          }] else [] end)
        + (if $wireguard_last_apply_success != null and $wireguard_last_apply_success != 1 then [{
            id: "wireguard-last-apply-failed",
            severity: "warning",
            summary: "WireGuard gateway status did not report a successful last apply."
          }] else [] end)
      );
    {
      validation: $validation,
      generated_at: $generated_at,
      artifact_dir: $artifact_dir,
      go_live_artifact_dir: $go_live_artifact_dir,
      validated_commit: {
        short: $validated_short_commit,
        full: $validated_commit
      },
      operator_status: (
        if $operations_healthy == "true"
          and $operations_alerts_count == 0
          and ((derived_alerts | length) == 0)
        then "healthy"
        else "attention"
        end
      ),
      derived_alerts: derived_alerts,
      runtime_alerts: {
        healthy: ($operations_healthy == "true"),
        alerts_count: $operations_alerts_count,
        alerts: ($operations_status[0].alerts // [])
      },
      scoring_audit: {
        status: (if $scoring_audit_status == "" then null else $scoring_audit_status end),
        stored_rows: $scoring_audit_stored_rows,
        replayed_rows: $scoring_audit_replayed_rows,
        mismatch_count: $scoring_audit_mismatch_count
      },
      attack_map_load: {
        validation_status: (if $attack_map_validation_status == "" then null else $attack_map_validation_status end),
        submissions_per_second: $attack_map_submissions_per_second,
        submission_p95_ms: $attack_map_submission_p95_ms,
        attack_feed_lag_ms: $attack_map_attack_feed_lag_ms
      },
      game_core: {
        match_state: $game_core_match_state,
        total_ticks: $game_core_total_ticks,
        checker_runs_total: $game_core_checker_runs_total,
        checker_runs_failed: $game_core_checker_runs_failed,
        scheduler_running: $game_core_scheduler_running
      },
      submission_service: {
        submit_requests_total: $submission_requests_total,
        submit_failures_total: $submission_failures_total,
        attack_feed_requests_total: $submission_attack_feed_requests_total,
        verdicts: {
          correct: $submission_verdict_correct,
          duplicate: $submission_verdict_duplicate,
          invalid: $submission_verdict_invalid
        }
      },
      controller_service: {
        deployment_reconcile_requests: $controller_deployment_reconcile_requests,
        access_reconcile_requests: $controller_access_reconcile_requests,
        service_access_reconcile_requests: $controller_service_access_reconcile_requests,
        ssh_credential_requests: $controller_ssh_credential_requests,
        access_policies_total: $controller_access_policies_total,
        access_last_apply_success: $controller_access_last_apply_success
      },
      realtime_gateway: {
        last_sync_success: $realtime_last_sync_success,
        sync_errors_total: $realtime_sync_errors_total,
        subscribers_total: $realtime_subscribers_total
      },
      wireguard_gateway: {
        reconcile_requests: $wireguard_reconcile_requests,
        peers_total: $wireguard_peer_total,
        peers_active: $wireguard_peer_active,
        peers_revoked: $wireguard_peer_revoked,
        last_apply_success: $wireguard_last_apply_success
      }
    }'
)"

if [[ -n "${output_file}" ]]; then
  mkdir -p "$(dirname "${output_file}")"
  printf '%s\n' "${summary_json}" > "${output_file}"
  echo "validation operator summary written:"
  printf '  %s\n' "${output_file}"
else
  printf '%s\n' "${summary_json}" | jq .
fi
