#!/usr/bin/env bash
# install-host-deps.sh
#
# Idempotently installs the host-level packages required to run the
# ADTickPlatform locally or in production.
#
# The Docker install follows Docker's official Ubuntu install procedure
# verbatim: https://docs.docker.com/engine/install/ubuntu/
#
# Supported targets: Ubuntu 22.04 / 24.04 / 26.04 LTS, Debian 12+.

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${ROOT_DIR}"

# --- 0. Sanity checks ---------------------------------------------------------

if [ "$(uname -s)" != "Linux" ]; then
  echo "ERROR: install-host-deps must run on Linux." >&2
  echo "       Target hosts: Ubuntu 22.04/24.04/26.04 LTS or Debian 12+." >&2
  exit 1
fi

if ! command -v apt-get >/dev/null 2>&1; then
  echo "ERROR: apt-get not found. This script targets Debian/Ubuntu hosts only." >&2
  exit 1
fi

if [ "$(id -u)" -eq 0 ]; then
  SUDO=""
else
  SUDO="sudo"
fi

# --- 1. Remove any conflicting unofficial Docker packages --------------------

echo ">>> Removing any conflicting unofficial Docker packages (best-effort)..."
CONFLICTING_PKGS="$(
  dpkg --get-selections \
    docker.io docker-compose docker-compose-v2 docker-doc \
    podman-docker containerd runc 2>/dev/null \
    | cut -f1
)"
if [ -n "${CONFLICTING_PKGS}" ]; then
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

echo ">>> Installing project host dependencies..."
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

# --- 5. Enable Docker service -----------------------------------------------

echo ">>> Enabling Docker service..."
if command -v systemctl >/dev/null 2>&1; then
  ${SUDO} systemctl enable --now docker || true
else
  echo "    (systemctl not available - start dockerd manually if needed)"
fi

# --- 6. Verify --------------------------------------------------------------

echo ">>> Verifying installation..."
docker --version
docker compose version

cat <<EOF

==> install-host-deps complete.

    Production next step:    sudo make preflight-prod-host
    Development next step:   docker compose -f deploy/compose/dev.yml up -d

    Manual Docker verify:    sudo docker run hello-world
EOF
