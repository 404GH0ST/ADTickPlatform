# Sample LFI Challenge

This package is a simple Local File Inclusion (LFI) challenge example for AD tick games.

It demonstrates three platform-critical behaviors:

- enemy flag capture through an LFI path traversal
- flag storage and retrieval across checker `put/get`
- SLA checks using only valid, normal user inputs

## Service Behavior

The service image:

- starts `sshd`
- enables root password login
- listens on `PORT` (`AD_PLATFORM_SERVICE_PORT`)
- stores the current flag at `/opt/ad/state/flag.txt`
- stores message state under `/opt/ad/state/messages`
- provides a vulnerable template renderer (`/v1/render`) with path traversal/LFI

Endpoints:

- `GET /v1/health`
- `POST /v1/flag` with `X-Checker-Token`
- `GET /v1/flag` with `X-Checker-Token`
- `POST /v1/messages`
- `GET /v1/messages/<slot>?token=...`
- `GET /v1/render?template=...` (intentionally vulnerable)

Teams recover another team’s current flag via LFI and submit it to the platform.

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
curl -sS "http://<target-host>:<target-port>/v1/render?template=../../state/flag.txt"
```

## Capture and SSH Access

Attack **another team's** service for this challenge and submit its current flag to
`POST /api/v2/submit` with your participant bearer token and `{"flags":["<captured flag>"]}`.
One accepted enemy flag automatically unlocks SSH on **your own** service for the same
challenge. Open **SSH Access** in the dashboard to retrieve the stable team credential.
Own-team, expired, invalid, and duplicate flags do not create an access grant. Factory
reset preserves access; organizer maintenance clears it. No unlock proof is required.

Run the bundled exploit against an enemy endpoint after the checker has stored a flag:

```bash
export PLATFORM_URL=http://localhost:8080
export EMAIL=alpha.captain@example.com
export PASSWORD=alpha-secret
python3 exploit.py <enemy-host> <enemy-port> <challenge-id>
```

The script supports the configured flag prefix, reads the API's `token` and
`results[].status` fields, and exits with an error if no capture is accepted.

See [the example verification guide](../README.md#verify-all-packages-in-isolation) to check all packages in disposable containers.

## Packet analysis

`tcpdump` is installed. After SSH access opens, guarded production instances can
capture incoming traffic directly:

```bash
tcpdump -p -Q in -i eth0 -nn
tcpdump -p -Q in -i eth0 -w /tmp/incoming.pcap
tcpdump -nn -r /tmp/incoming.pcap
```

Enable `CONTROLLER_CAPTURE_MODE=guarded` through `make setup-prod-env` and deploy
with `make up-prod-host`. The controller verifies kernel send restrictions and
the per-instance receive boundary before starting challenge code. Raw packet
injection is denied even for container root. The profile supports static IPv4
and one external interface. In `disabled` mode, only saved PCAP analysis works.
See [deployment requirements and first-rollout migration](../../deploy/security/direct-capture/README.md).
Rebuild the image and recreate legacy containers to install the package.

## Flag retention

The checker retrieves the actual stored flag without sending the expected value
in its request. A missing or different stored value returns checker exit code
**42**. Game-core verifies retention during the tick and before replacement;
confirmed loss forfeits that tick's SLA and applies the
[scaled defense penalty](../../docs/game-rules.md#flag-retention-and-replacement-penalties).
This remains true if the instance's checker token was used to overwrite the flag.
Rebuild service and checker images together for this token-authenticated read API.
