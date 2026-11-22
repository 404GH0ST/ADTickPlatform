#!/usr/bin/env bash
# rotate-grafana-password.sh
#
# Rotate the Grafana admin password without losing data. Resets the
# password via the in-container `grafana cli admin reset-admin-password`
# command (so the change persists in the grafana-data sqlite DB) and
# then writes the new value back to deploy/compose/prod.env so the next
# `make up-prod` redeploy uses the same password.
#
# Volumes (grafana-data, prometheus-data) are NOT touched, so dashboards,
# datasources, alert configurations, and the entire TSDB history survive
# intact.
#
# Usage:  make rotate-grafana-password-prod
# The new password is stored in the private production environment file.

set -euo pipefail
umask 077

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
ENV_FILE="${ROOT_DIR}/deploy/compose/prod.env"
COMPOSE_DIR="${ROOT_DIR}/deploy/compose"

if [[ ! -f "${ENV_FILE}" ]]; then
  echo "missing ${ENV_FILE}; copy from prod.env.example first" >&2
  exit 1
fi

if ! command -v openssl >/dev/null 2>&1; then
  echo "missing required binary: openssl" >&2
  exit 1
fi

cd "${COMPOSE_DIR}"
compose=(docker compose --env-file "${ENV_FILE}" -f "${COMPOSE_DIR}/prod.yml")
if ! "${compose[@]}" ps grafana --format '{{.State}}' 2>/dev/null | grep -q '^running$'; then
  echo "grafana container is not running; start it first with: make monitoring-up-prod" >&2
  exit 1
fi

mkdir -p -- "${ROOT_DIR}/.runtime"
exec {rotation_lock}>>"${ROOT_DIR}/.runtime/grafana-password-rotation.lock"
flock -x "${rotation_lock}"

NEW_PASSWORD="$(openssl rand -hex 24)"
replacement="$(mktemp -- "${ENV_FILE}.XXXXXXXX")"
trap 'rm -f -- "${replacement}"' EXIT
GRAFANA_ROTATION_PASSWORD="${NEW_PASSWORD}" awk '
    BEGIN { assignment = "GRAFANA_ADMIN_PASSWORD=" ENVIRON["GRAFANA_ROTATION_PASSWORD"] }
    /^GRAFANA_ADMIN_PASSWORD=/ { $0 = assignment; found = 1 }
    { print }
    END { if (!found) print assignment }
  ' "${ENV_FILE}" > "${replacement}"

if ! reset_result="$(printf '%s\n' "${NEW_PASSWORD}" | "${compose[@]}" exec -T grafana \
  grafana cli admin reset-admin-password --password-from-stdin 2>&1)" ||
  [[ "${reset_result}" != *"Admin password changed successfully"* ]]; then
  echo 'Grafana did not confirm the password change.' >&2
  exit 1
fi

if ! mv -fT -- "${replacement}" "${ENV_FILE}"; then
  trap - EXIT
  echo "The password changed. Recover the replacement environment file at ${replacement}." >&2
  exit 1
fi
echo "Updated deploy/compose/prod.env (GRAFANA_ADMIN_PASSWORD line)."
echo "Volumes grafana-data and prometheus-data were not touched."
