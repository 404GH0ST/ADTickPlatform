#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${ROOT_DIR}"

require_bin() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "missing required binary: $1" >&2
    exit 1
  fi
}

usage() {
  cat <<'EOF'
Usage: scripts/audit-checker-contracts.sh

Verifies that every repository checker package advertises the explicit Faust
service-state contract:
  - validate emits ADPLATFORM_CHECKER_CAPABILITIES={"service_state":true}
  - check can emit ADPLATFORM_SERVICE_STATE=...
EOF
}

if [[ "${1:-}" == "--help" ]]; then
  usage
  exit 0
fi

require_bin rg

mapfile -t checker_files < <(rg --files examples -g 'checker.py' | sort)

if [[ "${#checker_files[@]}" -eq 0 ]]; then
  echo "no checker packages found under examples/" >&2
  exit 1
fi

missing=0

for checker_file in "${checker_files[@]}"; do
  if ! rg -q 'ADPLATFORM_CHECKER_CAPABILITIES=\{"service_state":true\}' "${checker_file}"; then
    echo "missing service-state capability marker: ${checker_file}" >&2
    missing=1
  fi

  if ! rg -q 'ADPLATFORM_SERVICE_STATE=' "${checker_file}"; then
    echo "missing canonical service-state output: ${checker_file}" >&2
    missing=1
  fi
done

if [[ "${missing}" -ne 0 ]]; then
  exit 1
fi

echo "checker contract audit passed"
printf '  %s\n' "${checker_files[@]}"
