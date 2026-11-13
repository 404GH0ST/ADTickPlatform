ARG GO_VERSION=1.27

FROM golang:${GO_VERSION}-alpine AS builder

ARG TARGETOS=linux
ARG TARGETARCH=amd64

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY internal ./internal
COPY services ./services

RUN CGO_ENABLED=0 GOOS="${TARGETOS}" GOARCH="${TARGETARCH}" go build -o /out/service ./services/controller-service

FROM debian:trixie-slim AS capture-builder
RUN apt-get update && apt-get install -y --no-install-recommends clang gcc libc6-dev libbpf-dev linux-libc-dev
WORKDIR /src
COPY deploy/security/direct-capture/capture.bpf.c deploy/security/direct-capture/policy.c ./
RUN clang -O2 -g -target bpf -I/usr/include/$(gcc -dumpmachine) -c capture.bpf.c -o capture.bpf.o \
    && gcc -O2 -Wall -Wextra -Werror policy.c -o policy -lbpf

FROM docker:29-cli AS dockercli

FROM debian:trixie-slim

RUN apt-get update \
    && apt-get install -y --no-install-recommends \
        bash \
        ca-certificates \
        libbpf1 \
        iptables \
        nftables \
        tzdata \
        wget \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app

COPY --from=builder /out/service /usr/local/bin/service
COPY --from=dockercli /usr/local/bin/docker /usr/local/bin/docker

COPY --from=capture-builder /src/policy /usr/local/bin/adplatform-capture-policy
COPY --from=capture-builder /src/capture.bpf.o /usr/local/share/adplatform/capture.bpf.o

ENTRYPOINT ["/usr/local/bin/service"]
