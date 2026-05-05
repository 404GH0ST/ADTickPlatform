# Sample RCE Challenge

This package is a simple Remote Code Execution challenge example for AD tick games.

It demonstrates three platform-critical behaviors:

- unlock proof recovery through command injection
- single-current-flag storage and retrieval across checker `put/get`
- SLA checks using only valid, normal user inputs

## Service Behavior

The service image:

- starts `sshd`
- enables root password login
- listens on `PORT` (`AD_PLATFORM_SERVICE_PORT`)
- stores the current flag at `/opt/ad/state/flag.txt`
- exposes a read-only copy of the current flag at `/opt/ad/exposed/flag.txt`
- writes unlock proof to `/opt/ad/state/secret/unlock-proof.txt`
- exposes a read-only copy of the unlock proof at `/opt/ad/exposed/unlock-proof.txt`
- exposes a deliberately vulnerable diagnostics endpoint that interpolates user input into a shell command

Endpoints:

- `GET /v1/health`
- `GET /v1/status`
- `POST /v1/flag` with `X-Checker-Token`
- `GET /v1/flag?value=...`
- `GET /v1/diagnostics?host=...` (intentionally vulnerable)

There is no direct unlock endpoint. Teams are expected to recover unlock proof through RCE.

## Checker Behavior

The checker supports `validate`, `put`, `get`, and `check`.

- `put`: replaces `/opt/ad/state/flag.txt` with `AD_FLAG`
- `get`: reads back the current flag using metadata (`path=flag.txt`)
- `check`: performs only normal expected user requests:
  - health check
  - status read
  - diagnostics call with `host=localhost`

This `check` behavior is important for SLA: if a team patches the RCE but breaks valid diagnostics, SLA should fail.

## Why This Mimics Real AD Tick

- The exploit path can execute commands as the service process.
- The vulnerable command runs as the low-privilege `nobody` user in the container.
- The checker still validates legitimate user flows.
- The checker replaces `flag.txt` each tick, so the service exposes one current flag instead of accumulating tick files.

## Security Boundary

The authoritative state files under `/opt/ad/state` are written with owner-only permissions
and replaced atomically on each checker `put`. The RCE path executes as `nobody`, so it can
read `/opt/ad/exposed/flag.txt` and `/opt/ad/exposed/unlock-proof.txt` but cannot write the
authoritative state directory in the normal container setup. The checker write endpoint also
requires `X-Checker-Token` so unauthenticated network attackers cannot overwrite the current
flag through the normal `put` API.

The intended attack path still works: the vulnerable `/v1/diagnostics` endpoint executes shell
commands as `nobody`, so another team can recover the current flag through RCE even though
direct filesystem reads of `/opt/ad/state` by unrelated non-root users are denied.

This is not a defense against root access, `docker exec`, Docker socket access, or host-level
access. The platform must enforce that teams can reach other teams' containers only through the
intended service port, and that SSH access is scoped to the owning team after unlock.

For a real event, set a non-default `AD_CHECKER_TOKEN` consistently for the service and checker
images. The bundled default is only for local examples.

## Build

```bash
docker build -t adplatform/sample-rce:baseline examples/sample-rce-challenge/service
docker build -t adplatform/sample-rce-checker:latest examples/sample-rce-challenge/checker
```

## Use With The Platform

Create a challenge with:

- `baseline_image = adplatform/sample-rce:baseline`
- `checker_image = adplatform/sample-rce-checker:latest`
- `source_bundle_path = sample-rce-challenge` when `AD_CHALLENGE_SOURCE_ROOT`
  points at a sanitized public source directory that contains dummy values
  instead of real secrets
- `service_port = 6768` (or your preferred service port)
- `service_subnet_octet = <unique subnet octet>`

Then deploy from organizer dashboard or organizer API.

For whitebox events, keep the downloadable source separate from any private
build context. The source bundle is served verbatim to participants, so real
secrets should be injected only at runtime.

## Quick Manual Test

Example normal request:

```bash
curl -sS http://<target-host>:<target-port>/v1/diagnostics?host=localhost
```

Example vulnerable request pattern for recovering the current flag:

```bash
curl -sS "http://<target-host>:<target-port>/v1/diagnostics?host=localhost%3B%20cat%20/opt/ad/exposed/flag.txt"
```
