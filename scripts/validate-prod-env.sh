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

echo "production HTTPS configuration validated: ${prod_env}"
