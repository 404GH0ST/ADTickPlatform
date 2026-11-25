#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${ROOT_DIR}"
source "${ROOT_DIR}/scripts/lib/common.sh"

prod_env="${1:-${PROD_ENV:-deploy/compose/prod.env}}"
if [[ ! -f "${prod_env}" ]]; then
  echo "missing production environment file: ${prod_env}" >&2
  exit 1
fi
load_env_file "${prod_env}"

require_https_url() {
  local name="$1"
  local value="${!name:-}"
  if [[ ! "${value}" =~ ^https://[^[:space:]]+$ ]]; then
    echo "${name} must be a non-empty https:// URL with no whitespace in ${prod_env}" >&2
    exit 1
  fi
}

require_https_url EDGE_SITE_ADDRESS
require_https_url AD_PLATFORM_PUBLIC_BASE_URL

# Demo migrations install captain accounts with passwords published in the docs.
for seed_flag in API_GATEWAY_AUTO_SEED DATABASE_INCLUDE_SEEDS; do
  if [[ -n "${!seed_flag:-}" && "${!seed_flag}" != false ]]; then
    echo "${seed_flag} must be false in production; seed migrations install public demo credentials" >&2
    exit 1
  fi
done

capture_mode="${CONTROLLER_CAPTURE_MODE:-disabled}"
case "${capture_mode}" in
  guarded|disabled) ;;
  *) echo "CONTROLLER_CAPTURE_MODE must be guarded or disabled" >&2; exit 1 ;;
esac
cap_add=",${CONTROLLER_SERVICE_CAP_ADD:-CHOWN,DAC_OVERRIDE,FOWNER,SETGID,SETUID,NET_BIND_SERVICE,SYS_CHROOT,AUDIT_WRITE},"
if [[ "${capture_mode}" == guarded ]]; then
  if [[ "${2:-}" == base ]]; then
    echo "guarded capture requires make up-prod-host; base-only deployment lacks kernel policy privileges" >&2
    exit 1
  fi
  [[ "${CONTROLLER_SERVICE_CAP_DROP:-ALL}" == ALL ]] || { echo "guarded capture requires CAP_DROP=ALL" >&2; exit 1; }
  [[ "${CONTROLLER_SERVICE_SECURITY_OPT:-no-new-privileges:true}" == no-new-privileges:true ]] || { echo "guarded capture requires no-new-privileges:true" >&2; exit 1; }
  IFS=, read -r -a caps <<<"${CONTROLLER_SERVICE_CAP_ADD:-CHOWN,DAC_OVERRIDE,FOWNER,SETGID,SETUID,NET_BIND_SERVICE,SYS_CHROOT,AUDIT_WRITE}"
  for cap in "${caps[@]}"; do
    case "${cap#CAP_}" in
      CHOWN|DAC_OVERRIDE|FOWNER|SETGID|SETUID|NET_BIND_SERVICE|SYS_CHROOT|AUDIT_WRITE|NET_RAW) ;;
      *) echo "unsafe challenge capability in guarded capture: ${cap}" >&2; exit 1 ;;
    esac
  done
else
  if [[ "${cap_add^^}" == *,NET_RAW,* || "${cap_add^^}" == *,CAP_NET_RAW,* || "${cap_add^^}" == *,ALL,* || "${cap_add^^}" == *,CAP_ALL,* ]]; then
    echo "NET_RAW requires guarded capture; do not enable it manually or through ALL" >&2; exit 1
  fi
  cap_drop=",${CONTROLLER_SERVICE_CAP_DROP:-ALL},"
  if [[ "${cap_drop^^}" != *,ALL,* && "${cap_drop^^}" != *,CAP_ALL,* && "${cap_drop^^}" != *,NET_RAW,* && "${cap_drop^^}" != *,CAP_NET_RAW,* ]]; then
    echo "disabled capture requires dropping NET_RAW or ALL; Docker defaults include raw packet access" >&2; exit 1
  fi
fi

echo "production HTTPS configuration validated: ${prod_env}"
