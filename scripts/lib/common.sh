#!/usr/bin/env bash

load_env_file() {
  local env_file="$1"
  local line key value

  [[ -f "${env_file}" ]] || return 0

  while IFS= read -r line || [[ -n "${line}" ]]; do
    [[ -z "${line}" ]] && continue
    [[ "${line}" =~ ^[[:space:]]*# ]] && continue
    [[ "${line}" != *=* ]] && continue

    key="${line%%=*}"
    value="${line#*=}"
    key="${key#"${key%%[![:space:]]*}"}"
    key="${key%"${key##*[![:space:]]}"}"

    if [[ -n "${!key+x}" ]]; then
      continue
    fi

    export "${key}=${value}"
  done < "${env_file}"
}

load_env_file_override() {
  local env_file="$1"
  local line key value

  [[ -f "${env_file}" ]] || return 0

  while IFS= read -r line || [[ -n "${line}" ]]; do
    [[ -z "${line}" ]] && continue
    [[ "${line}" =~ ^[[:space:]]*# ]] && continue
    [[ "${line}" != *=* ]] && continue

    key="${line%%=*}"
    value="${line#*=}"
    key="${key#"${key%%[![:space:]]*}"}"
    key="${key%"${key##*[![:space:]]}"}"

    export "${key}=${value}"
  done < "${env_file}"
}

load_env_key_override() {
  local env_file="$1"
  local wanted_key="$2"
  local line key value

  [[ -f "${env_file}" ]] || return 0

  while IFS= read -r line || [[ -n "${line}" ]]; do
    [[ -z "${line}" ]] && continue
    [[ "${line}" =~ ^[[:space:]]*# ]] && continue
    [[ "${line}" != *=* ]] && continue

    key="${line%%=*}"
    value="${line#*=}"
    key="${key#"${key%%[![:space:]]*}"}"
    key="${key%"${key##*[![:space:]]}"}"

    if [[ "${key}" == "${wanted_key}" ]]; then
      export "${key}=${value}"
      return 0
    fi
  done < "${env_file}"
}

load_env_file_over_placeholders() {
  local env_file="$1"
  local line key value current

  [[ -f "${env_file}" ]] || return 0

  while IFS= read -r line || [[ -n "${line}" ]]; do
    [[ -z "${line}" ]] && continue
    [[ "${line}" =~ ^[[:space:]]*# ]] && continue
    [[ "${line}" != *=* ]] && continue

    key="${line%%=*}"
    value="${line#*=}"
    key="${key#"${key%%[![:space:]]*}"}"
    key="${key%"${key##*[![:space:]]}"}"
    current="${!key:-}"

    if is_placeholder_secret "${current}"; then
      export "${key}=${value}"
    fi
  done < "${env_file}"
}

load_default_env_files() {
  if [[ -f .env ]]; then
    load_env_file .env
    return 0
  fi

  if [[ -f .env.example ]]; then
    load_env_file .env.example
  fi
}

is_placeholder_secret() {
  local value="${1:-}"

  [[ -z "${value}" ]] && return 0
  [[ "${value}" == dev-* ]] && return 0
  [[ "${value}" == "dev-admin-token" ]] && return 0
  [[ "${value}" == "dev-team-token" ]] && return 0
  [[ "${value}" == *change-this-* ]] && return 0
  [[ "${value}" == *replace-with-* ]] && return 0

  return 1
}

random_secret() {
  if command -v openssl >/dev/null 2>&1; then
    openssl rand -hex 32
    return
  fi
  od -An -tx1 -N32 /dev/urandom | tr -d ' \n'
}

random_wireguard_private_key() {
  if command -v wg >/dev/null 2>&1; then
    wg genkey
    return
  fi
  if command -v openssl >/dev/null 2>&1; then
    openssl rand -base64 32
    return
  fi
  od -An -tx1 -N32 /dev/urandom | tr -d ' \n' | base64
}

ensure_runtime_secret() {
  local key="$1"
  local current="${!key:-}"
  local key_slug

  if is_placeholder_secret "${current}"; then
    key_slug="$(printf '%s' "${key}" | tr '[:upper:]' '[:lower:]')"
    export "${key}=local-${key_slug}-$(random_secret)"
  fi
}

ensure_runtime_wireguard_private_key() {
  if is_placeholder_secret "${WIREGUARD_SERVER_PRIVATE_KEY:-}"; then
    export WIREGUARD_SERVER_PRIVATE_KEY="$(random_wireguard_private_key)"
  fi
}

ensure_local_runtime_secrets() {
  ensure_runtime_secret ADMIN_API_TOKEN
  ensure_runtime_secret TEAM_JWT_SECRET
  ensure_runtime_secret UNLOCK_PROOF_SECRET
  ensure_runtime_secret SSH_CREDENTIAL_SECRET
  ensure_runtime_secret GAME_CORE_FLAG_SECRET
  ensure_runtime_secret CONTROLLER_INTERNAL_TOKEN
  ensure_runtime_secret GAME_CORE_INTERNAL_TOKEN
  ensure_runtime_secret SUBMISSION_SERVICE_INTERNAL_TOKEN
  ensure_runtime_secret SCORING_WORKER_INTERNAL_TOKEN
  ensure_runtime_secret CHECKER_RUNNER_INTERNAL_TOKEN
  ensure_runtime_secret WIREGUARD_GATEWAY_INTERNAL_TOKEN
  ensure_runtime_wireguard_private_key
  ensure_runtime_secret REALTIME_ADMIN_TOKEN
  if is_placeholder_secret "${REALTIME_SOURCE_ADMIN_TOKEN:-}"; then
    export REALTIME_SOURCE_ADMIN_TOKEN="${ADMIN_API_TOKEN}"
  fi
}

write_local_runtime_env_file() {
  local runtime_env="${1:-.runtime/dev-secrets.env}"

  mkdir -p "$(dirname "${runtime_env}")"
  cat > "${runtime_env}" <<EOF
ADMIN_API_TOKEN=${ADMIN_API_TOKEN}
TEAM_JWT_SECRET=${TEAM_JWT_SECRET}
UNLOCK_PROOF_SECRET=${UNLOCK_PROOF_SECRET}
SSH_CREDENTIAL_SECRET=${SSH_CREDENTIAL_SECRET}
GAME_CORE_FLAG_SECRET=${GAME_CORE_FLAG_SECRET}
CONTROLLER_INTERNAL_TOKEN=${CONTROLLER_INTERNAL_TOKEN}
GAME_CORE_INTERNAL_TOKEN=${GAME_CORE_INTERNAL_TOKEN}
SUBMISSION_SERVICE_INTERNAL_TOKEN=${SUBMISSION_SERVICE_INTERNAL_TOKEN}
SCORING_WORKER_INTERNAL_TOKEN=${SCORING_WORKER_INTERNAL_TOKEN}
CHECKER_RUNNER_INTERNAL_TOKEN=${CHECKER_RUNNER_INTERNAL_TOKEN}
WIREGUARD_GATEWAY_INTERNAL_TOKEN=${WIREGUARD_GATEWAY_INTERNAL_TOKEN}
WIREGUARD_SERVER_PRIVATE_KEY=${WIREGUARD_SERVER_PRIVATE_KEY}
REALTIME_SOURCE_ADMIN_TOKEN=${REALTIME_SOURCE_ADMIN_TOKEN}
REALTIME_ADMIN_TOKEN=${REALTIME_ADMIN_TOKEN}
EOF
  chmod 600 "${runtime_env}"
}

ensure_local_runtime_env_file() {
  local runtime_env="${1:-.runtime/dev-secrets.env}"

  if [[ -f "${runtime_env}" ]]; then
    load_env_file_over_placeholders "${runtime_env}"
  fi
  ensure_local_runtime_secrets
  write_local_runtime_env_file "${runtime_env}"
}

resolve_admin_api_token() {
  local runtime_env="${1:-.runtime/backend-stack.env}"

  if is_placeholder_secret "${ADMIN_API_TOKEN:-}" && [[ -f "${runtime_env}" ]]; then
    load_env_key_override "${runtime_env}" ADMIN_API_TOKEN
  fi

  if is_placeholder_secret "${ADMIN_API_TOKEN:-}"; then
    cat >&2 <<'EOF'
ADMIN_API_TOKEN is required and must not be a placeholder.
Run `make generate-prod-env` for production, or start the local stack with
`scripts/run-backend-stack.sh` so .runtime/backend-stack.env is generated.
EOF
    return 1
  fi

  printf '%s\n' "${ADMIN_API_TOKEN}"
}

load_runtime_env_if_present() {
  local runtime_env="${1:-.runtime/backend-stack.env}"

  if [[ -f "${runtime_env}" ]]; then
    load_env_file_override "${runtime_env}"
  fi
}

require_bin() {
  local binary="$1"
  if ! command -v "${binary}" >/dev/null 2>&1; then
    echo "missing required binary: ${binary}" >&2
    exit 1
  fi
}

sudo_noninteractive_available() {
  local probe=""

  for probe in /usr/bin/true /bin/true; do
    if [[ -x "${probe}" ]] && sudo -n "${probe}" >/dev/null 2>&1; then
      return 0
    fi
  done

  return 1
}

run_as_root_noninteractive() {
  if [[ "${EUID}" -eq 0 ]]; then
    "$@"
    return
  fi

  if sudo_noninteractive_available; then
    sudo -n "$@"
    return
  fi

  echo "passwordless sudo is required for host-level automation: $*" >&2
  echo "configure sudo NOPASSWD for this operator or run the command as root" >&2
  return 1
}

require_noninteractive_root() {
  if [[ "${EUID}" -eq 0 ]]; then
    return 0
  fi

  require_bin sudo
  if sudo_noninteractive_available; then
    return 0
  fi

  cat >&2 <<'EOF'
host automation requires non-interactive root access.
Configure sudo NOPASSWD for this operator or run the command with sudo.
EOF
  return 1
}

remove_unused_docker_network() {
  local network_name="$1"
  local attached=""

  [[ -n "${network_name}" ]] || return 0

  if ! docker network inspect "${network_name}" >/dev/null 2>&1; then
    return 0
  fi

  attached="$(docker network inspect -f '{{len .Containers}}' "${network_name}" 2>/dev/null || true)"
  if [[ "${attached}" != "0" ]]; then
    return 0
  fi

  docker network rm "${network_name}" >/dev/null 2>&1 || true
}

wait_for_http() {
  local url="$1"
  local attempts="${2:-40}"
  local name="${3:-}"
  shift $(( $# < 3 ? $# : 3 ))
  # Any remaining arguments are extra curl args (e.g. --cacert for a self-signed
  # edge); without them an https readiness probe fails TLS verification even when
  # the rest of the caller can reach the endpoint.
  local extra_curl_args=("$@")
  local i

  if ! command -v curl >/dev/null 2>&1; then
    if [[ -n "${name}" ]]; then
      echo "curl not found, skipping readiness check for ${name}"
    fi
    return 0
  fi

  local curl_tls_args=()
  if [[ "${url}" == https://* && "${ADMIN_CURL_INSECURE:-false}" == "true" ]]; then
    curl_tls_args=(-k)
  fi

  for ((i = 1; i <= attempts; i++)); do
    if curl "${curl_tls_args[@]}" ${extra_curl_args[@]+"${extra_curl_args[@]}"} -fsS "${url}" >/dev/null 2>&1; then
      if [[ -n "${name}" ]]; then
        echo "${name} ready at ${url}"
      fi
      return 0
    fi
    sleep 1
  done

  if [[ -n "${name}" ]]; then
    echo "${name} did not become ready at ${url}" >&2
  fi
  return 1
}

http_ready() {
  local url="$1"

  if ! command -v curl >/dev/null 2>&1; then
    return 1
  fi

  curl -fsS "${url}" >/dev/null 2>&1
}

derive_edge_base_url() {
  local base_url="${AD_PLATFORM_PUBLIC_BASE_URL:-${EDGE_SITE_ADDRESS:-https://localhost}}"
  local scheme authority default_port configured_port

  base_url="${base_url%/}"
  scheme="${base_url%%://*}"
  authority="${base_url#*://}"
  authority="${authority%%/*}"

  if [[ "${authority}" == *:* ]]; then
    printf '%s\n' "${base_url}"
    return 0
  fi

  if [[ "${scheme}" == "https" ]]; then
    default_port=443
    configured_port="${EDGE_HTTPS_PUBLIC_PORT:-443}"
  else
    default_port=80
    configured_port="${EDGE_HTTP_PUBLIC_PORT:-80}"
  fi

  if [[ "${configured_port}" == "${default_port}" ]]; then
    printf '%s\n' "${base_url}"
    return 0
  fi

  printf '%s:%s\n' "${base_url}" "${configured_port}"
}

slug_name() {
  printf '%s' "$1" | tr '[:upper:]' '[:lower:]' | sed 's/[^a-z0-9]/-/g; s/-\{2,\}/-/g; s/^-//; s/-$//'
}
