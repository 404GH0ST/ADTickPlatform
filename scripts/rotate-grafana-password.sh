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
# Effect: prints the new password to stdout exactly once. Record it in
#         your password manager; the rotation script does not persist it
#         anywhere retrievable after the script exits.

set -euo pipefail

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
if ! docker compose --env-file "${ENV_FILE}" ps grafana --format '{{.State}}' 2>/dev/null | grep -q '^running$'; then
  echo "grafana container is not running; start it first with: make monitoring-up-prod" >&2
  exit 1
fi

NEW_PASSWORD="$(openssl rand -hex 24)"

docker compose --env-file "${ENV_FILE}" exec -T grafana \
  grafana cli admin reset-admin-password "${NEW_PASSWORD}" >/dev/null

awk -v pw="${NEW_PASSWORD}" -F= '
    BEGIN { OFS="=" }
    /^GRAFANA_ADMIN_PASSWORD=/ { $2=pw }
    { print }
  ' "${ENV_FILE}" > "${ENV_FILE}.new"
mv "${ENV_FILE}.new" "${ENV_FILE}"
chmod 600 "${ENV_FILE}"

echo
echo "==============================================================="
echo "  New Grafana admin password (record NOW; not stored elsewhere)"
echo "==============================================================="
echo "${NEW_PASSWORD}"
echo "==============================================================="
echo
echo "Updated deploy/compose/prod.env (GRAFANA_ADMIN_PASSWORD line)."
echo "Volumes grafana-data and prometheus-data were not touched."
