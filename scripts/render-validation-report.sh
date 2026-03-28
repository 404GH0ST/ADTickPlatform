#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${ROOT_DIR}"
source "${ROOT_DIR}/scripts/lib/common.sh"

require_bin jq

artifact_dir="${VALIDATION_REPORT_ARTIFACT_DIR:-}"
output_file="${VALIDATION_REPORT_OUTPUT:-}"

usage() {
  cat <<'EOF'
Usage:
  scripts/render-validation-report.sh

Optional:
  VALIDATION_REPORT_ARTIFACT_DIR=.runtime/release-candidate-<timestamp>
  VALIDATION_REPORT_OUTPUT=.runtime/release-candidate-<timestamp>/operator-report.html
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

if [[ -z "${artifact_dir}" ]]; then
  artifact_dir="$(pick_default_artifact_dir || true)"
fi

if [[ -z "${artifact_dir}" || ! -d "${artifact_dir}" ]]; then
  usage >&2
  echo "missing validation artifact directory: ${artifact_dir}" >&2
  exit 1
fi

summary_file="${artifact_dir}/summary.json"
operator_summary_file="${artifact_dir}/operator-summary.json"
if [[ ! -f "${summary_file}" ]]; then
  usage >&2
  echo "missing summary file: ${summary_file}" >&2
  exit 1
fi

if [[ ! -f "${operator_summary_file}" ]]; then
  VALIDATION_SUMMARY_ARTIFACT_DIR="${artifact_dir}" \
  VALIDATION_SUMMARY_OUTPUT="${operator_summary_file}" \
    "${ROOT_DIR}/scripts/summarize-validation-artifacts.sh" >/dev/null
fi

if [[ -z "${output_file}" ]]; then
  output_file="${artifact_dir}/operator-report.html"
fi

mkdir -p "$(dirname "${output_file}")"

jq -rn \
  --slurpfile summary "${summary_file}" \
  --slurpfile operator "${operator_summary_file}" \
  '
  def fmt_num:
    if . == null then "n/a"
    elif (type == "number") and (floor == .) then tostring
    else tostring
    end;
  def fmt_bool:
    if . == null then "n/a"
    elif . == 1 then "yes"
    elif . == 0 then "no"
    elif . == true then "yes"
    elif . == false then "no"
    else tostring
    end;
  def alert_rows($alerts):
    if (($alerts | length) == 0) then
      "<tr><td colspan=\"4\">No alerts.</td></tr>"
    else
      ($alerts | map(
        "<tr><td>" + (.severity | @html) + "</td><td><code>" + (.id | @html) + "</code></td><td>" + (.summary | @html) + "</td><td>" + ((.detail // "") | @html) + "</td></tr>"
      ) | join(""))
    end;
  ($summary[0]) as $s |
  ($operator[0]) as $o |
  ($s.artifacts // {}) as $artifacts |
  "<!DOCTYPE html>\n"
  + "<html lang=\"en\">\n<head>\n<meta charset=\"utf-8\">\n<meta name=\"viewport\" content=\"width=device-width, initial-scale=1\">\n"
  + "<title>Validation Report</title>\n"
  + "<style>\n"
  + ":root{color-scheme:light;font-family:ui-sans-serif,system-ui,-apple-system,BlinkMacSystemFont,\"Segoe UI\",sans-serif;--bg:#f6f7fb;--panel:#ffffff;--border:#d9deea;--text:#172033;--muted:#5a6478;--good:#1f7a45;--warn:#9a6700;--bad:#b42318;--accent:#2357ff;}\n"
  + "*{box-sizing:border-box}body{margin:0;background:var(--bg);color:var(--text)}main{max-width:1180px;margin:0 auto;padding:32px 20px 48px}h1,h2,h3{margin:0 0 12px}p{margin:0 0 12px;line-height:1.5;color:var(--muted)}code{font-family:ui-monospace,SFMono-Regular,Menlo,monospace;font-size:.95em}a{color:var(--accent);text-decoration:none}a:hover{text-decoration:underline}.hero{display:flex;justify-content:space-between;gap:16px;align-items:flex-start;margin-bottom:24px}.badge{display:inline-block;padding:6px 10px;border-radius:999px;font-weight:700;font-size:12px;letter-spacing:.04em;text-transform:uppercase}.badge.healthy{background:#e7f6ec;color:var(--good)}.badge.attention{background:#fff1e5;color:var(--bad)}.meta{display:grid;grid-template-columns:repeat(auto-fit,minmax(220px,1fr));gap:12px;margin-top:18px}.card{background:var(--panel);border:1px solid var(--border);border-radius:16px;padding:18px;box-shadow:0 6px 20px rgba(26,39,68,.05)}.grid{display:grid;grid-template-columns:repeat(auto-fit,minmax(220px,1fr));gap:14px;margin:18px 0 24px}.metric{font-size:28px;font-weight:700;margin:8px 0 4px}.metric-label{font-size:13px;color:var(--muted);text-transform:uppercase;letter-spacing:.04em}.section{margin-top:28px}.section-grid{display:grid;grid-template-columns:repeat(auto-fit,minmax(280px,1fr));gap:14px}.kv{display:grid;grid-template-columns:1fr auto;gap:10px 12px}.kv div:nth-child(odd){color:var(--muted)}table{width:100%;border-collapse:collapse;background:var(--panel);border:1px solid var(--border);border-radius:16px;overflow:hidden}th,td{padding:12px 14px;text-align:left;border-bottom:1px solid var(--border);vertical-align:top}th{font-size:12px;text-transform:uppercase;letter-spacing:.04em;color:var(--muted);background:#f9fbff}.footnote{margin-top:18px;font-size:13px;color:var(--muted)}ul.links{margin:0;padding-left:18px}.links li{margin:6px 0}\n"
  + "</style>\n</head>\n<body>\n<main>\n"
  + "<section class=\"hero\">"
  + "<div><h1>Validation Report</h1><p>Operator-facing summary for the latest validated artifact tree.</p></div>"
  + "<span class=\"badge " + ($o.operator_status | @html) + "\">" + ($o.operator_status | @html) + "</span>"
  + "</section>\n"
  + "<section class=\"card\"><div class=\"meta\">"
  + "<div><div class=\"metric-label\">Validation</div><div><code>" + ($s.validation | @html) + "</code></div></div>"
  + "<div><div class=\"metric-label\">Generated</div><div><code>" + ($s.generated_at | @html) + "</code></div></div>"
  + "<div><div class=\"metric-label\">Validated Commit</div><div><code>" + (($o.validated_commit.short // "n/a") | @html) + "</code></div></div>"
  + "<div><div class=\"metric-label\">Artifact Dir</div><div><code>" + ($s.artifact_dir | @html) + "</code></div></div>"
  + "</div></section>\n"
  + "<section class=\"grid\">"
  + "<div class=\"card\"><div class=\"metric-label\">Runtime Alerts</div><div class=\"metric\">" + ($o.runtime_alerts.alerts_count | fmt_num | @html) + "</div><p>Organizer runtime alert surface at the end of validation.</p></div>"
  + "<div class=\"card\"><div class=\"metric-label\">Derived Alerts</div><div class=\"metric\">" + (($o.derived_alerts | length) | tostring | @html) + "</div><p>Summary-derived conditions from checker, submission, realtime, controller, and WireGuard metrics.</p></div>"
  + "<div class=\"card\"><div class=\"metric-label\">Attack Throughput</div><div class=\"metric\">" + ($o.attack_map_load.submissions_per_second | fmt_num | @html) + "/s</div><p>Measured during the active attack submission window.</p></div>"
  + "<div class=\"card\"><div class=\"metric-label\">Submission p95</div><div class=\"metric\">" + ($o.attack_map_load.submission_p95_ms | fmt_num | @html) + " ms</div><p>Submission-service path latency from the load gate.</p></div>"
  + "</section>\n"
  + "<section class=\"section\"><h2>Alerts</h2><div class=\"section-grid\">"
  + "<div><h3>Runtime</h3><table><thead><tr><th>Severity</th><th>ID</th><th>Summary</th><th>Detail</th></tr></thead><tbody>"
  + alert_rows($o.runtime_alerts.alerts)
  + "</tbody></table></div>"
  + "<div><h3>Derived</h3><table><thead><tr><th>Severity</th><th>ID</th><th>Summary</th><th>Detail</th></tr></thead><tbody>"
  + alert_rows($o.derived_alerts)
  + "</tbody></table></div>"
  + "</div></section>\n"
  + "<section class=\"section\"><h2>Service Metrics</h2><div class=\"section-grid\">"
  + "<div class=\"card\"><h3>Game Core</h3><div class=\"kv\"><div>Match state</div><div><code>" + ($o.game_core.match_state | @html) + "</code></div><div>Total ticks</div><div>" + ($o.game_core.total_ticks | fmt_num | @html) + "</div><div>Checker runs</div><div>" + ($o.game_core.checker_runs_total | fmt_num | @html) + "</div><div>Checker failures</div><div>" + ($o.game_core.checker_runs_failed | fmt_num | @html) + "</div><div>Scheduler running</div><div>" + ($o.game_core.scheduler_running | fmt_bool | @html) + "</div></div></div>"
  + "<div class=\"card\"><h3>Submission Service</h3><div class=\"kv\"><div>Submit requests</div><div>" + ($o.submission_service.submit_requests_total | fmt_num | @html) + "</div><div>Submit failures</div><div>" + ($o.submission_service.submit_failures_total | fmt_num | @html) + "</div><div>Attack-feed requests</div><div>" + ($o.submission_service.attack_feed_requests_total | fmt_num | @html) + "</div><div>Correct verdicts</div><div>" + ($o.submission_service.verdicts.correct | fmt_num | @html) + "</div><div>Duplicate verdicts</div><div>" + ($o.submission_service.verdicts.duplicate | fmt_num | @html) + "</div><div>Invalid verdicts</div><div>" + ($o.submission_service.verdicts.invalid | fmt_num | @html) + "</div></div></div>"
  + "<div class=\"card\"><h3>Controller Service</h3><div class=\"kv\"><div>Deployment reconciles</div><div>" + ($o.controller_service.deployment_reconcile_requests | fmt_num | @html) + "</div><div>Access reconciles</div><div>" + ($o.controller_service.access_reconcile_requests | fmt_num | @html) + "</div><div>Service access reconciles</div><div>" + ($o.controller_service.service_access_reconcile_requests | fmt_num | @html) + "</div><div>SSH credentials issued</div><div>" + ($o.controller_service.ssh_credential_requests | fmt_num | @html) + "</div><div>Access policies</div><div>" + ($o.controller_service.access_policies_total | fmt_num | @html) + "</div><div>Last apply success</div><div>" + ($o.controller_service.access_last_apply_success | fmt_bool | @html) + "</div></div></div>"
  + "<div class=\"card\"><h3>Realtime Gateway</h3><div class=\"kv\"><div>Last sync success</div><div>" + ($o.realtime_gateway.last_sync_success | fmt_bool | @html) + "</div><div>Sync errors</div><div>" + ($o.realtime_gateway.sync_errors_total | fmt_num | @html) + "</div><div>Subscribers</div><div>" + ($o.realtime_gateway.subscribers_total | fmt_num | @html) + "</div><div>Attack-feed lag</div><div>" + ($o.attack_map_load.attack_feed_lag_ms | fmt_num | @html) + " ms</div></div></div>"
  + "<div class=\"card\"><h3>WireGuard Gateway</h3><div class=\"kv\"><div>Reconcile requests</div><div>" + ($o.wireguard_gateway.reconcile_requests | fmt_num | @html) + "</div><div>Total peers</div><div>" + ($o.wireguard_gateway.peers_total | fmt_num | @html) + "</div><div>Active peers</div><div>" + ($o.wireguard_gateway.peers_active | fmt_num | @html) + "</div><div>Revoked peers</div><div>" + ($o.wireguard_gateway.peers_revoked | fmt_num | @html) + "</div><div>Last apply success</div><div>" + ($o.wireguard_gateway.last_apply_success | fmt_bool | @html) + "</div></div></div>"
  + "</div></section>\n"
  + "<section class=\"section\"><h2>Evidence</h2><div class=\"card\"><ul class=\"links\">"
  + "<li><a href=\"summary.json\">summary.json</a></li>"
  + "<li><a href=\"operator-summary.json\">operator-summary.json</a></li>"
  + (if ($artifacts.operator_report // "") != "" then "<li><a href=\"" + ($artifacts.operator_report | @html) + "\">operator-report.html</a></li>" else "" end)
  + (if ($artifacts.go_live_check.summary // "") != "" then "<li><a href=\"" + ($artifacts.go_live_check.summary | @html) + "\">go-live summary</a></li>" else "" end)
  + (if ($artifacts.go_live_check.operator_summary // "") != "" then "<li><a href=\"" + ($artifacts.go_live_check.operator_summary | @html) + "\">go-live operator summary</a></li>" else "" end)
  + (if ($artifacts.event_ready_note // "") != "" then "<li><a href=\"" + ($artifacts.event_ready_note | @html) + "\">event-ready note</a></li>" else "" end)
  + "</ul><p class=\"footnote\">This report is generated from the machine-readable validation artifacts and is safe to regenerate.</p></div></section>\n"
  + "</main>\n</body>\n</html>\n"
  ' > "${output_file}"

echo "validation report written:"
printf '  %s\n' "${output_file}"
