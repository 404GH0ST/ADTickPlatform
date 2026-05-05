#!/bin/sh
set -eu

mkdir -p /run/sshd /opt/ad/state /opt/ad/public /opt/ad/exposed

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

echo "root:bootstrap" | chpasswd

/usr/sbin/sshd
exec python3 /app/service.py
