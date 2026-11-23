FROM debian:trixie-slim

# Debian's php-cli provides /usr/bin/php (with the built-in -S web server); apt
# pulls from the OS mirror (not a container registry) for portable builds.
RUN apt-get update \
  && apt-get install -y --no-install-recommends php-cli openssh-server passwd tcpdump \
  && rm -rf /var/lib/apt/lists/*

RUN mkdir -p /run/sshd /opt/ad/state

WORKDIR /app

COPY router.php /app/router.php
COPY entrypoint.sh /app/entrypoint.sh

RUN chmod 755 /app/entrypoint.sh

ENTRYPOINT ["/app/entrypoint.sh"]
