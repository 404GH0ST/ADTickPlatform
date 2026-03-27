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

load_default_env_files() {
  if [[ -f .env ]]; then
    load_env_file .env
    return 0
  fi

  if [[ -f .env.example ]]; then
    load_env_file .env.example
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
  local i

  if ! command -v curl >/dev/null 2>&1; then
    if [[ -n "${name}" ]]; then
      echo "curl not found, skipping readiness check for ${name}"
    fi
    return 0
  fi

  for ((i = 1; i <= attempts; i++)); do
    if curl -fsS "${url}" >/dev/null 2>&1; then
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
  local base_url="${AD_PLATFORM_PUBLIC_BASE_URL:-${EDGE_SITE_ADDRESS:-http://localhost}}"
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
