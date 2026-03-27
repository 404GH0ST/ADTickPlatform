# Sample HTTP Challenge

This is a minimal challenge package that satisfies the platform runtime contract.

It exists for two purposes:

- to show challenge authors exactly how the service and checker containers should look
- to give the platform a known-good package for Docker-mode smoke testing

## Service Behavior

The service image:

- starts `sshd`
- enables root password login
- listens on `PORT`
- stores its current flag under `/opt/ad/state`
- exposes a deliberately insecure unlock endpoint at `GET /unlock`
- exposes a deliberately insecure flag disclosure endpoint at `GET /leak`

Endpoints:

- `GET /health`
- `POST /flag` with `{"flag":"..."}`
- `GET /flag?value=...`
- `GET /unlock`
- `GET /leak`

`/leak` exists so the platform can run a real end-to-end attack smoke against the sample package. It is intentionally insecure and should not be treated as a normal checker path.

## Checker Behavior

The checker image:

- supports `validate`
- supports `put`, `get`, and `check`
- uses `AD_TARGET_HOST`, `AD_TARGET_PORT`, and `AD_FLAG`

## Build

```bash
docker build -t adplatform/sample-http:baseline examples/sample-http-challenge/service
docker build -t adplatform/sample-http-checker:latest examples/sample-http-challenge/checker
```

## Use With The Platform

Create a challenge with:

- `baseline_image = adplatform/sample-http:baseline`
- `checker_image = adplatform/sample-http-checker:latest`

Then deploy and reconcile it through the organizer API or dashboard.

For a full Docker-mode end-to-end smoke:

```bash
make smoke-sample-challenge-docker
```
