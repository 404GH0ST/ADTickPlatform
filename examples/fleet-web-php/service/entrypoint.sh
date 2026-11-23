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

# SSH stays closed until the platform provisions a per-team credential after an
# accepted enemy flag on this challenge. Lock root so password auth cannot
# succeed before then.
passwd -l root >/dev/null 2>&1 || true

/usr/sbin/sshd

PORT_VALUE="${PORT:-${AD_PLATFORM_SERVICE_PORT:-8080}}"
exec php -d variables_order=EGPCS -S 0.0.0.0:"${PORT_VALUE}" /app/router.php
