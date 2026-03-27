FROM debian:bookworm-slim

RUN apt-get update \
  && apt-get install -y --no-install-recommends openssh-server python3 \
  && rm -rf /var/lib/apt/lists/*

RUN mkdir -p /run/sshd /opt/ad/state /opt/ad/content/templates

WORKDIR /app

COPY service.py /app/service.py
COPY entrypoint.sh /app/entrypoint.sh

RUN chmod 755 /app/entrypoint.sh

ENTRYPOINT ["/app/entrypoint.sh"]
