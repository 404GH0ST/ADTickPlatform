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
- exposes a deliberately insecure flag disclosure endpoint at `GET /leak`

Endpoints:

- `GET /health`
- `POST /flag` with `X-Checker-Token` and `{"flag":"..."}`
- `GET /flag` with `X-Checker-Token`
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

## Capture and SSH Access

Attack **another team's** service for this challenge and submit its current flag to
`POST /api/v2/submit` with your participant bearer token and `{"flags":["<captured flag>"]}`.
One accepted enemy flag automatically unlocks SSH on **your own** service for the same
challenge. Open **SSH Access** in the dashboard to retrieve the stable team credential.
Own-team, expired, invalid, and duplicate flags do not create an access grant. Factory
reset preserves access; organizer maintenance clears it. No unlock proof is required.

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
