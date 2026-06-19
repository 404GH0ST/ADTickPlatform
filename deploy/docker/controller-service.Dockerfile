ARG GO_VERSION=1.26

FROM golang:${GO_VERSION}-alpine AS builder

ARG TARGETOS=linux
ARG TARGETARCH=amd64

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY internal ./internal
COPY services ./services

RUN CGO_ENABLED=0 GOOS="${TARGETOS}" GOARCH="${TARGETARCH}" go build -o /out/service ./services/controller-service

FROM docker:28-cli AS dockercli

FROM debian:bookworm-slim

RUN apt-get update \
    && apt-get install -y --no-install-recommends \
        bash \
        ca-certificates \
        iptables \
        nftables \
        tzdata \
        wget \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app

COPY --from=builder /out/service /usr/local/bin/service
COPY --from=dockercli /usr/local/bin/docker /usr/local/bin/docker

ENTRYPOINT ["/usr/local/bin/service"]
