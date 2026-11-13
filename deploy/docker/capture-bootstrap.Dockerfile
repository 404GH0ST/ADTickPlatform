ARG GO_VERSION=1.27
FROM golang:${GO_VERSION}-alpine AS builder
ARG TARGETOS=linux
ARG TARGETARCH=amd64
WORKDIR /src
COPY go.mod go.sum ./
COPY services/capture-bootstrap ./services/capture-bootstrap
RUN CGO_ENABLED=0 GOOS="${TARGETOS}" GOARCH="${TARGETARCH}" go build -o /start ./services/capture-bootstrap
FROM scratch
COPY --from=builder /start /start
ENTRYPOINT ["/start"]
CMD ["install", "/capture"]
