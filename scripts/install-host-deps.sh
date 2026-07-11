#!/usr/bin/env bash
# install-host-deps.sh
#
# Idempotently installs the host-level packages required to run the
# ADTickPlatform locally or in production.
#
# Supported targets:
#   - Ubuntu 22.04 / 24.04 / 26.04 LTS, Debian 12+ (apt + Docker Inc repo)
#   - Arch Linux / Arch-based (pacman; Docker from official repos)
#
# Ubuntu Docker install follows Docker's official procedure:
#   https://docs.docker.com/engine/install/ubuntu/

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${ROOT_DIR}"

# --- 0. Sanity checks ---------------------------------------------------------

if [ "$(uname -s)" != "Linux" ]; then
  echo "ERROR: install-host-deps must run on Linux." >&2
  echo "       Target hosts: Ubuntu 22.04/24.04/26.04 LTS, Debian 12+, or Arch Linux." >&2
  exit 1
fi

if [ "$(id -u)" -eq 0 ]; then
  SUDO=""
else
  SUDO="sudo"
fi

detect_pkg_manager() {
  # shellcheck source=/dev/null
  if [ -f /etc/os-release ]; then
    . /etc/os-release
  fi

  if command -v pacman >/dev/null 2>&1; then
    case "${ID:-}:${ID_LIKE:-}" in
      arch:*|*:*arch*|*:*archlinux*)
        echo "pacman"
        return 0
        ;;
    esac
    # Manjaro, EndeavourOS, CachyOS, etc. often set ID differently but ship pacman.
    if [ -f /etc/arch-release ] || [ "${ID:-}" = "arch" ] || [ "${ID:-}" = "manjaro" ] || [ "${ID:-}" = "endeavouros" ] || [ "${ID:-}" = "cachyos" ]; then
      echo "pacman"
      return 0
    fi
  fi

  if command -v apt-get >/dev/null 2>&1; then
    echo "apt"
    return 0
  fi

  echo "unsupported"
}

enable_docker_service() {
  echo ">>> Enabling Docker service..."
  if command -v systemctl >/dev/null 2>&1; then
    ${SUDO} systemctl enable --now docker || true
  else
    echo "    (systemctl not available - start dockerd manually if needed)"
  fi
}

verify_install() {
  echo ">>> Verifying installation..."
  docker --version
  if docker compose version >/dev/null 2>&1; then
    docker compose version
  elif command -v docker-compose >/dev/null 2>&1; then
    docker-compose version
  else
    echo "ERROR: neither 'docker compose' nor 'docker-compose' is available." >&2
    exit 1
  fi

  cat <<EOF

==> install-host-deps complete.

    Production next step:    sudo make preflight-prod-host
    Development next step:   docker compose -f deploy/compose/dev.yml up -d

    Manual Docker verify:    sudo docker run hello-world
EOF
}

install_debian_ubuntu() {
  # --- 1. Remove any conflicting unofficial Docker packages --------------------

  echo ">>> Removing any conflicting unofficial Docker packages (best-effort)..."
  CONFLICTING_PKGS="$(
    dpkg --get-selections \
      docker.io docker-compose docker-compose-v2 docker-doc \
      podman-docker containerd runc 2>/dev/null \
      | cut -f1
  )"
  if [ -n "${CONFLICTING_PKGS}" ]; then
    # shellcheck disable=SC2086
    ${SUDO} apt remove -y ${CONFLICTING_PKGS} || true
  else
    echo "    (none installed)"
  fi

  # --- 2. Install Docker apt repo prerequisites --------------------------------

  echo ">>> Installing ca-certificates and curl (Docker apt repo prerequisites)..."
  ${SUDO} apt update
  ${SUDO} apt install -y ca-certificates curl

  # --- 3. Add Docker's official GPG key and apt repository ---------------------

  echo ">>> Adding Docker's official GPG key and apt repository..."
  ${SUDO} install -m 0755 -d /etc/apt/keyrings
  ${SUDO} curl -fsSL https://download.docker.com/linux/ubuntu/gpg -o /etc/apt/keyrings/docker.asc
  ${SUDO} chmod a+r /etc/apt/keyrings/docker.asc

  # shellcheck source=/dev/null
  . /etc/os-release
  DISTRO_CODENAME="${UBUNTU_CODENAME:-${VERSION_CODENAME:-}}"
  if [ -z "${DISTRO_CODENAME}" ]; then
    echo "ERROR: cannot determine distro codename from /etc/os-release." >&2
    exit 1
  fi
  ARCH="$(dpkg --print-architecture)"

  # Format follows the Docker official Ubuntu install docs verbatim.
  ${SUDO} tee /etc/apt/sources.list.d/docker.sources >/dev/null <<EOF
Types: deb
URIs: https://download.docker.com/linux/ubuntu
Suites: ${DISTRO_CODENAME}
Components: stable
Architectures: ${ARCH}
Signed-By: /etc/apt/keyrings/docker.asc
EOF

  ${SUDO} apt update

  # --- 4. Install the project host dependencies --------------------------------

  echo ">>> Installing project host dependencies (Debian/Ubuntu)..."
  ${SUDO} apt install -y \
    curl \
    git \
    make \
    ca-certificates \
    docker-ce \
    docker-ce-cli \
    containerd.io \
    docker-buildx-plugin \
    docker-compose-plugin \
    wireguard-tools \
    nftables \
    iptables \
    iproute2 \
    jq
}

install_arch() {
  # Arch ships Docker, compose, and buildx from the official repos.
  # See: https://wiki.archlinux.org/title/Docker

  echo ">>> Syncing pacman databases..."
  ${SUDO} pacman -Sy --noconfirm

  echo ">>> Installing project host dependencies (Arch Linux)..."
  # --needed keeps the install idempotent when packages are already present.
  ${SUDO} pacman -S --needed --noconfirm \
    curl \
    git \
    make \
    ca-certificates \
    docker \
    docker-compose \
    docker-buildx \
    wireguard-tools \
    nftables \
    iptables \
    iproute2 \
    jq

  # docker-compose on Arch provides the Compose V2 plugin used as `docker compose`.
  if ! docker compose version >/dev/null 2>&1 && command -v docker-compose >/dev/null 2>&1; then
    echo "    note: use 'docker compose' (plugin) or 'docker-compose' if the plugin is not on PATH yet"
  fi
}

PKG_MANAGER="$(detect_pkg_manager)"
case "${PKG_MANAGER}" in
  apt)
    install_debian_ubuntu
    ;;
  pacman)
    install_arch
    ;;
  *)
    echo "ERROR: unsupported package manager." >&2
    echo "       Supported hosts: Ubuntu 22.04/24.04/26.04 LTS, Debian 12+, or Arch Linux." >&2
    exit 1
    ;;
esac

enable_docker_service
verify_install
