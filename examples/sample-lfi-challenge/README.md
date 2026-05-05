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
- stores the current flag at `/opt/ad/state/flag.txt`
- stores message state under `/opt/ad/state/messages`
- writes unlock proof to `/opt/ad/state/secret/unlock-proof.txt`
- provides a vulnerable template renderer (`/v1/render`) with path traversal/LFI

Endpoints:

- `GET /v1/health`
- `POST /v1/flag` with `X-Checker-Token`
- `GET /v1/flag?value=...`
- `POST /v1/messages`
- `GET /v1/messages/<slot>?token=...`
- `GET /v1/render?template=...` (intentionally vulnerable)

There is no direct unlock endpoint. Teams are expected to recover unlock proof via LFI.

## Checker Behavior

The checker supports `validate`, `put`, `get`, and `check`.

- `put`: replaces `/opt/ad/state/flag.txt` with `AD_FLAG`
- `get`: reads back the current flag using metadata (`path=flag.txt`)
- `check`: performs only normal expected user requests:
  - health check
  - render `welcome.txt`
  - create and read a normal message

This `check` behavior is important for SLA: if a team patches the vuln but breaks valid user functionality, SLA should fail.

## Why This Mimics Real AD Tick

- The exploit path exists and can leak sensitive data.
- The checker still validates legitimate user flows.
- The checker replaces `flag.txt` each tick, so the service exposes one current flag instead of accumulating tick files.

## Security Boundary

`flag.txt` is written with owner-only permissions and replaced atomically on each checker `put`.
This limits accidental reads by non-root users in the same container, but it is not a defense
against root access, `docker exec`, Docker socket access, or host-level access. The platform
must enforce that teams can reach other teams' containers only through the intended service
port, and that SSH access is scoped to the owning team after unlock.

The intended attack path still works: the vulnerable `/v1/render` endpoint reads files as the
service process, so another team can recover the current flag through LFI even though direct
filesystem reads by unrelated non-root users are denied.

The checker write endpoint requires `X-Checker-Token` so unauthenticated network attackers
cannot overwrite the current flag through the normal `put` API. For a real event, set a
non-default `AD_CHECKER_TOKEN` consistently for the service and checker images. The bundled
default is only for local examples.

## Build

```bash
docker build -t adplatform/sample-lfi:baseline examples/sample-lfi-challenge/service
docker build -t adplatform/sample-lfi-checker:latest examples/sample-lfi-challenge/checker
```

## Use With The Platform

Create a challenge with:

- `baseline_image = adplatform/sample-lfi:baseline`
- `checker_image = adplatform/sample-lfi-checker:latest`
- `source_bundle_path = sample-lfi-challenge` when `AD_CHALLENGE_SOURCE_ROOT`
  points at a sanitized public source directory that contains dummy values
  instead of real secrets
- `service_port = 6767` (or your preferred service port)
- `service_subnet_octet = <unique subnet octet>`

Then deploy from organizer dashboard or organizer API.

For whitebox events, keep the downloadable source separate from any private
build context. The source bundle is served verbatim to participants, so real
secrets should be injected only at runtime.

## Quick Manual Test

Example normal request:

```bash
curl -sS http://<target-host>:<target-port>/v1/health
```

Example vulnerable request pattern (for demonstration):

```bash
curl -sS "http://<target-host>:<target-port>/v1/render?template=../../state/secret/unlock-proof.txt"
curl -sS "http://<target-host>:<target-port>/v1/render?template=../../state/flag.txt"
```
