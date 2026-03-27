# Sample LFI Challenge

This package is a simple Local File Inclusion (LFI) challenge example for AD tick games.

It demonstrates three platform-critical behaviors:

- unlock proof exposure through an LFI path traversal
- flag storage and retrieval across checker `put/get`
- SLA checks using only valid, normal user inputs

## Service Behavior

The service image:

- starts `sshd`
- enables root password login
- listens on `PORT` (`AD_PLATFORM_SERVICE_PORT`)
- stores message state under `/opt/ad/state/messages`
- writes unlock proof to `/opt/ad/state/secret/unlock-proof.txt`
- provides a vulnerable template renderer (`/v1/render`) with path traversal/LFI

Endpoints:

- `GET /v1/health`
- `POST /v1/messages`
- `GET /v1/messages/<slot>?token=...`
- `GET /v1/render?template=...` (intentionally vulnerable)

There is no direct unlock endpoint. Teams are expected to recover unlock proof via LFI.

## Checker Behavior

The checker supports `validate`, `put`, `get`, and `check`.

- `put`: writes `AD_FLAG` into one of 3 slots based on `AD_TICK_ID % 3`
- `get`: reads back the flag using metadata (`slot`, `token`)
- `check`: performs only normal expected user requests:
  - health check
  - render `welcome.txt`
  - create and read a normal message

This `check` behavior is important for SLA: if a team patches the vuln but breaks valid user functionality, SLA should fail.

## Why This Mimics Real AD Tick

- The exploit path exists and can leak sensitive data.
- The checker still validates legitimate user flows.
- Slot-based storage (`tick % 3`) gives simple rotating state across ticks.

## Build

```bash
docker build -t adplatform/sample-lfi:baseline examples/sample-lfi-challenge/service
docker build -t adplatform/sample-lfi-checker:latest examples/sample-lfi-challenge/checker
```

## Use With The Platform

Create a challenge with:

- `baseline_image = adplatform/sample-lfi:baseline`
- `checker_image = adplatform/sample-lfi-checker:latest`
- `service_port = 6767` (or your preferred service port)
- `service_subnet_octet = <unique subnet octet>`

Then deploy from organizer dashboard or organizer API.

## Quick Manual Test

Example normal request:

```bash
curl -sS http://<target-host>:<target-port>/v1/health
```

Example vulnerable request pattern (for demonstration):

```bash
curl -sS "http://<target-host>:<target-port>/v1/render?template=../../state/secret/unlock-proof.txt"
```
