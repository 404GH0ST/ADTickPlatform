#!/usr/bin/env bash
# Read-only checks; never changes kernel boot options or running containers.
set -euo pipefail
ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
source "${ROOT_DIR}/scripts/lib/common.sh"
load_env_file "${1:-${PROD_ENV:-${ROOT_DIR}/deploy/compose/prod.env}}"
[[ "${CONTROLLER_CAPTURE_MODE:-disabled}" == guarded ]] || exit 0
fail() { echo "guarded capture preflight: $*" >&2; exit 1; }
[[ "$(uname -s)" == Linux ]] || fail "requires a Linux Docker host"
[[ -f /sys/fs/cgroup/cgroup.controllers ]] || fail "requires cgroup v2"
[[ -r /sys/kernel/btf/vmlinux ]] || fail "requires kernel BTF"
[[ -r /sys/kernel/security/lsm ]] || fail "cannot read active Linux security modules"
lsm="$(cat /sys/kernel/security/lsm)"
[[ ",${lsm}," == *,bpf,* ]] || fail "BPF LSM is inactive; enable bpf in the host lsm= boot configuration and reboot before deployment"
context_endpoint="${DOCKER_HOST:-$(docker context inspect --format '{{.Endpoints.docker.Host}}')}"
[[ "${context_endpoint}" == unix://* ]] || fail "requires the local rootful Docker daemon (host namespace/cgroup mounts must match)"
[[ "$(docker info --format '{{.CgroupVersion}}')" == 2 ]] || fail "Docker must use cgroup v2"
security="$(docker info --format '{{json .SecurityOptions}}')"
[[ "${security}" != *rootless* && "${security}" != *userns* ]] || fail "rootless Docker and user namespace remapping are unsupported"
# Docker normally makes its runtime directory root-only. Do not mistake an
# operator traversal denial for a missing directory; bind mounting it below
# still fails closed if the daemon cannot find the configured source.
if [[ "${EUID}" -eq 0 ]]; then
  [[ -d "${CAPTURE_NETNS_HOST_PATH:-/var/run/docker/netns}" ]] || fail "Docker network namespace directory is missing; configure CAPTURE_NETNS_HOST_PATH"
elif sudo_noninteractive_available; then
  sudo -n test -d "${CAPTURE_NETNS_HOST_PATH:-/var/run/docker/netns}" || fail "Docker network namespace directory is missing"
fi
echo "guarded capture host prerequisites passed (kernel attachment is verified before each challenge starts)"
