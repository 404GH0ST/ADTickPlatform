# Challenge Runtime Contract

This document defines the container contract that the platform expects from:

- the `service` image deployed once per `team x challenge`
- the `checker` image executed by `checker-runner`

These rules are what make deployment, checker execution, SSH patch access, and deterministic IP assignment work.

## Service Container

Each challenge service is one Docker container per team.

Current runtime assumptions:

- runtime kind: `docker`
- one container per `team x challenge`
- one persistent Docker volume per `team x challenge`
- same IP is used for both the vulnerable service and SSH access
- the container is attached to the game Docker network with a static IP

### Required Capabilities

The baseline service image must provide:

- `/bin/sh`
- an SSH daemon such as `sshd` or `dropbear`
- a password setter such as `chpasswd` or `passwd`

Those are already enforced by controller-side runtime validation and SSH-contract probes.

### Expected Runtime Behavior

The service should listen on the configured `service_port`.

The platform injects:

- `AD_PLATFORM_TEAM_ID`
- `AD_PLATFORM_CHALLENGE_ID`
- `AD_PLATFORM_SERVICE_IP`
- `AD_PLATFORM_SERVICE_PORT`
- `AD_PLATFORM_ENDPOINT`
- `AD_PLATFORM_UNLOCK_PROOF`
- `PORT`

Recommended service-image behavior:

- bind the vulnerable service to `AD_PLATFORM_SERVICE_PORT` or `PORT`
- do not rely on Docker host port publishing; the platform reaches the service through its static `10.80.x.y` address on the game network
- expose the unlock proof through the intended exploit path
- keep mutable state in the mounted state directory if the challenge needs persistence between restarts

### Persistent State

The controller mounts one Docker volume into:

- `CONTROLLER_STATE_MOUNT_PATH`

Current default:

- `/opt/ad/state`

Semantics:

- `restart`: preserves that volume
- `factory reset`: removes that volume and recreates the container from the organizer baseline image

## Checker Container

The checker image is executed by `checker-runner` as an ephemeral Docker container.

When `CHECKER_RUNNER_MODE=docker`, the checker container must join the same Docker network as the target service container.

Current default:

- `AD_PLATFORM_NETWORK_LAYOUT=per-service`
- `CHECKER_RUNNER_DOCKER_NETWORK=adplatform_game`

### Required Entrypoint

One of these must exist:

- `checker` in `PATH`
- `/checker`
- `/app/checker`
- `/checker.sh`

### Required Phases

The checker must support:

- `put`
- `get`
- `check`

Validation also accepts:

- `validate`
- or `--help`

### Checker Environment

`checker-runner` injects:

- `AD_PHASE`
- `AD_TEAM_ID`
- `AD_TEAM_NAME`
- `AD_CHALLENGE_ID`
- `AD_CHALLENGE_NAME`
- `AD_TARGET`
- `AD_TARGET_HOST`
- `AD_TARGET_IP`
- `AD_TARGET_PORT`
- `AD_TICK_ID`
- `AD_FLAG`
- `AD_METADATA`

Use:

- `AD_TARGET` when your checker already expects `host:port`
- `AD_TARGET_HOST` and `AD_TARGET_PORT` when you want explicit socket parameters
- `AD_FLAG` as the authoritative flag value for `put` and `get`
- `AD_METADATA` to carry checker output from one phase to the next inside the same tick

## IP Assignment

Each challenge stores:

- `service_port`
- `service_subnet_octet`

The deployed service address is:

```text
10.80.<service_subnet_octet>.<team_service_octet>:<service_port>
```

Current team octet rule:

```text
team_service_octet = team_id - 90
```

Examples:

- `team_id=101`, `service_subnet_octet=2`, `service_port=10002`
  - service IP: `10.80.2.11`
  - endpoint: `10.80.2.11:10002`
- `team_id=104`, `service_subnet_octet=7`, `service_port=31337`
  - service IP: `10.80.7.14`
  - endpoint: `10.80.7.14:31337`

Defaults when the organizer does not specify them:

- `service_port = 10000 + challenge_id`
- `service_subnet_octet = challenge_id`

## Organizer Challenge Fields

The admin challenge model now supports:

- `name`
- `baseline_image`
- `checker_image`
- `weight`
- `service_port`
- `service_subnet_octet`

This means the organizer can control:

- which baseline service image is deployed
- which checker image is executed
- which in-container service port is targeted
- which `10.80.x.y` subnet octet is assigned for that challenge

A working reference package is included in:

- `examples/sample-http-challenge`
- `examples/sample-lfi-challenge`

## Docker Network Layout

When `CONTROLLER_RUNTIME_MODE=docker`, the platform supports two layouts:

- `per-service`
  - current default
  - one Docker network per `service_subnet_octet`
  - all team replicas of the same challenge share that network
  - different services are isolated from each other at the Docker network layer
- `shared`
  - compatibility mode
  - all services share one Docker network

Current defaults:

- `AD_PLATFORM_NETWORK_LAYOUT=per-service`
- `CONTROLLER_DOCKER_NETWORK=adplatform_game`
- `CHECKER_RUNNER_DOCKER_NETWORK=adplatform_game`

In `per-service` layout, those values are base names. The controller derives the real network from the assigned service IP.

Example for service IP `10.80.50.11`:

- controller network name: `adplatform_game_svc_050`
- checker network name: `adplatform_game_svc_050`
- Docker subnet: `10.80.50.0/24`

Do not pre-create a shared `10.80.0.0/16` Docker network when running `per-service` layout. It overlaps with the per-service `/24` networks and will break deployment reconcile.

In `shared` layout, the single configured network should cover the service range used by the platform, typically:

- `10.80.0.0/16`

Example bootstrap command for `shared` layout only:

```bash
docker network create --subnet 10.80.0.0/16 adplatform_game
```

## Practical Build Guidance

For challenge authors:

1. build the service image so it can listen on an injected port
2. keep the exploitable secret path tied to `AD_PLATFORM_UNLOCK_PROOF`
3. include `sshd` plus a password setter
4. keep mutable state under the mounted state directory if persistence matters
5. build the checker to consume `AD_TARGET_HOST`, `AD_TARGET_PORT`, `AD_FLAG`, and `AD_METADATA`

For organizers:

1. validate the challenge before deploy
2. choose a unique `service_subnet_octet` per challenge
3. choose the actual service port the checker should attack
4. keep the service-address range aligned with the `10.80.x.y` addressing scheme

For an end-to-end Docker-mode reference run:

```bash
make smoke-sample-challenge-docker
```

That smoke resets the Postgres-backed stack to a clean match baseline, deploys the sample challenge to every team, starts a match, advances one authoritative tick, and verifies persisted `put/get/check` runs for the deployed sample service replicas.

## Minimal Example Images

### Service Image

This is the minimum shape that works with the platform contract:

```dockerfile
FROM debian:bookworm-slim

RUN apt-get update \
  && apt-get install -y --no-install-recommends openssh-server python3 \
  && rm -rf /var/lib/apt/lists/*

RUN mkdir -p /run/sshd /opt/ad/state

WORKDIR /app
COPY service.py /app/service.py
COPY entrypoint.sh /app/entrypoint.sh
RUN chmod +x /app/entrypoint.sh

ENTRYPOINT ["/app/entrypoint.sh"]
```

```sh
#!/bin/sh
set -eu

mkdir -p /run/sshd /opt/ad/state
echo "root:bootstrap" | chpasswd

/usr/sbin/sshd
exec python3 /app/service.py
```

Service expectations:

- listen on `PORT` or `AD_PLATFORM_SERVICE_PORT`
- expose the exploit path that reveals `AD_PLATFORM_UNLOCK_PROOF`
- keep mutable state under `/opt/ad/state` if restart persistence matters

### Checker Image

This is the minimum checker shape:

```dockerfile
FROM python:3.12-slim

WORKDIR /app
COPY checker.py /app/checker.py
RUN chmod +x /app/checker.py

ENTRYPOINT ["python3", "/app/checker.py"]
```

```python
#!/usr/bin/env python3
import os
import socket
import sys

phase = sys.argv[1]
host = os.environ["AD_TARGET_HOST"]
port = int(os.environ["AD_TARGET_PORT"])
flag = os.environ.get("AD_FLAG", "")

if phase == "validate":
    print("ok")
    raise SystemExit(0)

with socket.create_connection((host, port), timeout=5) as sock:
    if phase == "put":
        sock.sendall(f"PUT {flag}\n".encode())
    elif phase == "get":
        sock.sendall(f"GET {flag}\n".encode())
    elif phase == "check":
        sock.sendall(b"PING\n")
    else:
        raise SystemExit(2)
    print(sock.recv(4096).decode(errors="replace"))
```

Checker expectations:

- support `put`, `get`, `check`
- optionally support `validate`
- reach the target through `AD_TARGET_HOST` and `AD_TARGET_PORT`
- treat `AD_FLAG` as the authoritative flag value for the current tick
