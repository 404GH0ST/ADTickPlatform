#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
TEMPLATE="${PROD_ENV_TEMPLATE:-${ROOT_DIR}/deploy/compose/prod.env.example}"
OUTPUT="${1:-${PROD_ENV:-${ROOT_DIR}/deploy/compose/prod.env}}"

if [[ ! -f "${TEMPLATE}" ]]; then
  echo "missing template: ${TEMPLATE}" >&2
  exit 1
fi

if [[ -e "${OUTPUT}" && "${FORCE:-false}" != "true" ]]; then
  echo "refusing to overwrite ${OUTPUT}; set FORCE=true to replace it" >&2
  exit 1
fi

if ! command -v openssl >/dev/null 2>&1; then
  echo "missing required binary: openssl" >&2
  exit 1
fi

random_hex() {
  openssl rand -hex "${1:-32}"
}

# URL-safe password without ambiguous punctuation for easy copy/paste into login forms.
random_password() {
  openssl rand -base64 "${1:-24}" | tr -d '/+=' | head -c 32
}

wireguard_private_key() {
  if command -v wg >/dev/null 2>&1; then
    wg genkey
    return
  fi
  openssl rand -base64 32
}

postgres_password="$(random_hex 24)"
admin_token="$(random_hex 32)"
controller_token="$(random_hex 32)"
game_core_token="$(random_hex 32)"
submission_token="$(random_hex 32)"
scoring_token="$(random_hex 32)"
checker_token="$(random_hex 32)"
wireguard_token="$(random_hex 32)"
realtime_token="$(random_hex 32)"
team_jwt_secret="$(random_hex 32)"
unlock_secret="$(random_hex 32)"
ssh_secret="$(random_hex 32)"
flag_secret="$(random_hex 32)"
grafana_admin_password="$(random_hex 24)"
wg_private_key="$(wireguard_private_key)"

# Organizer account for make create-admin (override via env when generating).
admin_display_name="${ADMIN_DISPLAY_NAME:-Organizer}"
admin_email="${ADMIN_EMAIL:-organizer@example.com}"
admin_password="${ADMIN_PASSWORD:-$(random_password 24)}"

# Optional host path override for challenge source bundles.
challenge_source_host_path="${CHALLENGE_SOURCE_HOST_PATH:-}"

postgres_dsn="postgres://adplatform:${postgres_password}@postgres:5432/adplatform?sslmode=disable"
postgres_host_dsn="postgres://adplatform:${postgres_password}@127.0.0.1:15432/adplatform?sslmode=disable"

mkdir -p "$(dirname "${OUTPUT}")"
tmp_file="$(mktemp "${OUTPUT}.tmp.XXXXXX")"
cleanup() {
  rm -f "${tmp_file}"
}
trap cleanup EXIT

while IFS= read -r line || [[ -n "${line}" ]]; do
  if [[ "${line}" != *=* || "${line}" =~ ^[[:space:]]*# ]]; then
    printf '%s\n' "${line}" >>"${tmp_file}"
    continue
  fi

  key="${line%%=*}"
  case "${key}" in
    POSTGRES_PASSWORD) value="${postgres_password}" ;;
    POSTGRES_DSN) value="${postgres_dsn}" ;;
    POSTGRES_DSN_HOST_ENFORCEMENT) value="${postgres_host_dsn}" ;;
    ADMIN_API_TOKEN) value="${admin_token}" ;;
    CONTROLLER_INTERNAL_TOKEN) value="${controller_token}" ;;
    GAME_CORE_INTERNAL_TOKEN) value="${game_core_token}" ;;
    SUBMISSION_SERVICE_INTERNAL_TOKEN) value="${submission_token}" ;;
    SCORING_WORKER_INTERNAL_TOKEN) value="${scoring_token}" ;;
    CHECKER_RUNNER_INTERNAL_TOKEN) value="${checker_token}" ;;
    WIREGUARD_GATEWAY_INTERNAL_TOKEN) value="${wireguard_token}" ;;
    REALTIME_ADMIN_TOKEN) value="${realtime_token}" ;;
    TEAM_JWT_SECRET) value="${team_jwt_secret}" ;;
    UNLOCK_PROOF_SECRET) value="${unlock_secret}" ;;
    SSH_CREDENTIAL_SECRET) value="${ssh_secret}" ;;
    GAME_CORE_FLAG_SECRET) value="${flag_secret}" ;;
    GRAFANA_ADMIN_PASSWORD) value="${grafana_admin_password}" ;;
    WIREGUARD_SERVER_PRIVATE_KEY) value="${wg_private_key}" ;;
    ADMIN_DISPLAY_NAME) value="${admin_display_name}" ;;
    ADMIN_EMAIL) value="${admin_email}" ;;
    ADMIN_PASSWORD) value="${admin_password}" ;;
    CHALLENGE_SOURCE_HOST_PATH)
      if [[ -n "${challenge_source_host_path}" ]]; then
        value="${challenge_source_host_path}"
      else
        printf '%s\n' "${line}" >>"${tmp_file}"
        continue
      fi
      ;;
    *)
      printf '%s\n' "${line}" >>"${tmp_file}"
      continue
      ;;
  esac
  printf '%s=%s\n' "${key}" "${value}" >>"${tmp_file}"
done <"${TEMPLATE}"

mv "${tmp_file}" "${OUTPUT}"
chmod 600 "${OUTPUT}"
trap - EXIT

echo "generated ${OUTPUT}"
echo
echo "Organizer login (bootstrap with: make create-admin after the stack is up)"
echo "  ADMIN_DISPLAY_NAME=${admin_display_name}"
echo "  ADMIN_EMAIL=${admin_email}"
echo "  ADMIN_PASSWORD=${admin_password}"
if [[ -n "${challenge_source_host_path}" ]]; then
  echo "  CHALLENGE_SOURCE_HOST_PATH=${challenge_source_host_path}"
fi
echo
echo "Next:"
echo "  make setup-prod-env DOMAIN=localhost SCHEME=http   # or your public host"
echo "  make up-prod-host && make create-admin"
echo "Review WireGuard endpoint and public URLs before going live."
