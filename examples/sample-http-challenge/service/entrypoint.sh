#!/bin/sh
set -eu

mkdir -p /run/sshd /opt/ad/state

if command -v ssh-keygen >/dev/null 2>&1; then
  ssh-keygen -A >/dev/null 2>&1 || true
fi

cat >/etc/ssh/sshd_config.d/adplatform.conf <<'EOF'
PermitRootLogin yes
PasswordAuthentication yes
UsePAM no
ChallengeResponseAuthentication no
KbdInteractiveAuthentication no
PidFile /run/sshd.pid
EOF

# Do NOT ship a known root password. SSH stays closed until the platform
# provisions a per-team credential on unlock (controller runs chpasswd).
# Lock the account explicitly so password auth cannot succeed before then.
passwd -l root >/dev/null 2>&1 || true

/usr/sbin/sshd
exec python3 /app/service.py
