FROM debian:trixie-slim

# Install Flask from the OS mirror (python3-flask) instead of pip/PyPI so the
# build depends only on the Debian mirror, not a container registry or PyPI CDN.
RUN apt-get update \
  && apt-get install -y --no-install-recommends python3 python3-flask openssh-server passwd tcpdump \
  && rm -rf /var/lib/apt/lists/*

RUN mkdir -p /run/sshd /opt/ad/state

WORKDIR /app

COPY app.py /app/app.py
COPY entrypoint.sh /app/entrypoint.sh

RUN chmod 755 /app/entrypoint.sh

ENTRYPOINT ["/app/entrypoint.sh"]
