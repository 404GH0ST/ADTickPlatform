FROM debian:trixie-slim

# Debian's nodejs provides /usr/bin/node; apt pulls from the OS mirror (not a
# container registry), which keeps the build portable across hosts.
RUN apt-get update \
  && apt-get install -y --no-install-recommends nodejs openssh-server passwd tcpdump \
  && rm -rf /var/lib/apt/lists/*

RUN mkdir -p /run/sshd /opt/ad/state

WORKDIR /app

COPY server.js /app/server.js
COPY entrypoint.sh /app/entrypoint.sh

RUN chmod 755 /app/entrypoint.sh

ENTRYPOINT ["/app/entrypoint.sh"]
