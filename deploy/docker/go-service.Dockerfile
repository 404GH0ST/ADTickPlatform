ARG GO_VERSION=1.26

FROM golang:${GO_VERSION}-alpine AS builder

ARG SERVICE_PATH
ARG TARGETOS=linux
ARG TARGETARCH=amd64

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY internal ./internal
COPY services ./services

RUN test -n "${SERVICE_PATH}"
RUN CGO_ENABLED=0 GOOS="${TARGETOS}" GOARCH="${TARGETARCH}" go build -o /out/service "./services/${SERVICE_PATH}"

FROM alpine:3.22

ARG EXTRA_APK=""
ARG APK_RETRIES=5

RUN set -eu; \
    attempt=1; \
    while true; do \
        if apk add --no-cache ca-certificates tzdata bash ${EXTRA_APK}; then \
            break; \
        fi; \
        if [ "${attempt}" -ge "${APK_RETRIES}" ]; then \
            exit 1; \
        fi; \
        attempt=$((attempt + 1)); \
        sleep 3; \
    done

WORKDIR /app

COPY --from=builder /out/service /usr/local/bin/service

ENTRYPOINT ["/usr/local/bin/service"]
