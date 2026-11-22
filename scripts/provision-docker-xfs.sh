#!/usr/bin/env bash
# Provision Docker's data-root on overlay2 + XFS with project quotas, which
# `scripts/preflight-prod-host.sh` requires for per-challenge writable-layer and
# state-volume disk limits (CONTROLLER_SERVICE_DISK_LIMIT_MIB).
#
# It does NOT reformat any real partition: it creates a loop-backed XFS image
# file on the existing filesystem, mounts it with project quotas, and points
# Docker's data-root at it. Idempotent: if Docker already meets the requirement
# it exits 0 without changes.
#
# WARNING: changing Docker's data-root and storage driver makes the images and
# containers on the current data-root invisible to the daemon. They are not
# deleted (they stay on the old data-root), but you must recreate them (compose
# up / docker run) after this runs. Named-volume DATA can be carried over with
# MIGRATE_VOLUMES=true. Run on a host you are provisioning for the event, not a
# busy shared daemon you cannot interrupt.
#
# Usage:
#   sudo CONFIRM=yes ./scripts/provision-docker-xfs.sh
# Env:
#   CONFIRM=yes                 required to apply changes (otherwise dry-run)
#   DOCKER_XFS_IMAGE=/var/lib/docker-xfs.img
#   DOCKER_XFS_MOUNT=/var/lib/docker-xfs
#   DOCKER_XFS_SIZE=150G
#   MIGRATE_VOLUMES=false       copy existing named volumes to the new data-root
set -uo pipefail

IMAGE="${DOCKER_XFS_IMAGE:-/var/lib/docker-xfs.img}"
MOUNT="${DOCKER_XFS_MOUNT:-/var/lib/docker-xfs}"
SIZE="${DOCKER_XFS_SIZE:-150G}"
CONFIRM="${CONFIRM:-no}"
MIGRATE_VOLUMES="${MIGRATE_VOLUMES:-false}"
DAEMON_JSON=/etc/docker/daemon.json

die() { echo "error: $*" >&2; exit 1; }
need_root() { [ "$(id -u)" = 0 ] || die "run as root (sudo)"; }

docker_meets_requirement() {
  command -v docker >/dev/null 2>&1 || return 1
  local driver root
  read -r driver root < <(docker info --format '{{.Driver}} {{.DockerRootDir}}' 2>/dev/null) || return 1
  [ "$driver" = overlay2 ] || return 1
  [ "$(findmnt -n -o FSTYPE -T "${root}/overlay2" 2>/dev/null)" = xfs ] || return 1
  local opts
  opts=",$(findmnt -n -o OPTIONS -T "${root}/overlay2" 2>/dev/null),"
  [[ "$opts" == *,pquota,* || "$opts" == *,prjquota,* ]] || return 1
  return 0
}

need_root

if docker_meets_requirement; then
  echo "Docker already runs overlay2 on XFS with project quotas; nothing to do."
  docker info --format 'driver={{.Driver}} root={{.DockerRootDir}}'
  exit 0
fi

echo "Docker does not currently meet the prod-host storage requirement:"
docker info --format '  driver={{.Driver}} root={{.DockerRootDir}}' 2>/dev/null || echo "  (docker not reachable)"
echo "Plan:"
echo "  - create ${SIZE} XFS image at ${IMAGE}, mount (pquota) at ${MOUNT}"
echo "  - set Docker data-root=${MOUNT}, storage-driver=overlay2, containerd-snapshotter=false"
echo "  - restart docker (current images/containers on the old data-root become inactive)"
[ "${MIGRATE_VOLUMES}" = true ] && echo "  - copy existing named volumes to the new data-root"

if [ "${CONFIRM}" != yes ]; then
  echo
  echo "DRY RUN. Re-run with CONFIRM=yes to apply (e.g. 'sudo CONFIRM=yes $0')."
  exit 0
fi

if ! command -v mkfs.xfs >/dev/null 2>&1; then
  echo "installing xfsprogs..."
  if command -v pacman >/dev/null 2>&1; then pacman -S --needed --noconfirm xfsprogs
  elif command -v apt-get >/dev/null 2>&1; then apt-get update && apt-get install -y xfsprogs
  elif command -v dnf >/dev/null 2>&1; then dnf install -y xfsprogs
  else die "install xfsprogs manually (mkfs.xfs not found)"; fi
fi
modprobe xfs 2>/dev/null || true

echo "== create XFS image =="
[ -f "${IMAGE}" ] || truncate -s "${SIZE}" "${IMAGE}"
blkid "${IMAGE}" 2>/dev/null | grep -q xfs || mkfs.xfs -q "${IMAGE}"
mkdir -p "${MOUNT}"

echo "== persist mount (fstab, before docker) =="
if ! grep -q " ${MOUNT} " /etc/fstab; then
  echo "${IMAGE} ${MOUNT} xfs loop,pquota,x-systemd.before=docker.service 0 0" >> /etc/fstab
fi
systemctl daemon-reload 2>/dev/null || true
mountpoint -q "${MOUNT}" || mount "${MOUNT}"
findmnt -no SOURCE,FSTYPE,OPTIONS "${MOUNT}" || die "mount failed"

echo "== stop docker =="
systemctl stop docker docker.socket 2>/dev/null || true
sleep 2

if [ "${MIGRATE_VOLUMES}" = true ] && [ -d /var/lib/docker/volumes ]; then
  echo "== copy named volumes =="
  mkdir -p "${MOUNT}/volumes"
  cp -a /var/lib/docker/volumes/. "${MOUNT}/volumes/" || true
fi

echo "== write ${DAEMON_JSON} =="
mkdir -p /etc/docker
if [ -s "${DAEMON_JSON}" ] && command -v python3 >/dev/null 2>&1; then
  python3 - "$DAEMON_JSON" "$MOUNT" <<'PY'
import json, sys
path, mount = sys.argv[1], sys.argv[2]
try:
    cfg = json.load(open(path))
except Exception:
    cfg = {}
cfg["data-root"] = mount
cfg["storage-driver"] = "overlay2"
cfg.setdefault("features", {})
cfg["features"]["containerd-snapshotter"] = False
json.dump(cfg, open(path, "w"), indent=2)
PY
else
  cat > "${DAEMON_JSON}" <<J
{
  "data-root": "${MOUNT}",
  "storage-driver": "overlay2",
  "features": { "containerd-snapshotter": false }
}
J
fi

echo "== start docker =="
systemctl start docker
sleep 3

echo "== verify =="
docker info --format 'driver={{.Driver}} root={{.DockerRootDir}}'
if docker volume create --driver local --opt size=100m adplatform-quota-probe >/dev/null 2>&1; then
  docker volume rm adplatform-quota-probe >/dev/null
  echo "project quota OK"
else
  die "project-quota volume probe failed"
fi
docker_meets_requirement && echo "prod-host storage requirement satisfied." || die "requirement still not met"
