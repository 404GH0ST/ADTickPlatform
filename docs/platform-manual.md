# Participant Platform Manual

This document defines the participant-facing platform specification. It is intentionally similar in spirit to common Attack-Defense competition manuals such as the COMPFEST public API style: simple versioned endpoints, JWT authentication, bulk flag submission, and a stable target-discovery format for team automation.

Machine-readable versions:

- [Interactive Swagger UI](/docs/platform-api)
- [OpenAPI Spec (YAML)](/docs/platform-api-v2.openapi.yaml)

## 1. Access Model

Participants use two access paths:

1. `HTTPS API`
   - used for authentication, service discovery, score data, flag submission, unlock, and reset actions
2. `WireGuard`
   - used to reach own and enemy services on the private game network
   - also used to SSH to that same service IP on port 22 after unlock

Recommended host layout:

- API host: `https://api.<platform-domain>`
- Web host: `https://app.<platform-domain>`
- WireGuard endpoint: `vpn.<platform-domain>:51820`

## 2. Identity And Credentials

Each team receives:

- one web/API account per member
- one WireGuard configuration per member
- one team JWT after API authentication
- optionally one dedicated `team-bot` WireGuard peer for automation

Authentication model:

- JWTs are issued per player account
- organizer-created player records are the source of truth for login
- the JWT carries team context, so one authenticated player can only act for that player's own team

Important rule:

- factory reset preserves the unlocked state of the service for that team during the same match

## 3. API Conventions

- API base path: `/api/v2`
- content type: `application/json`
- team-authenticated endpoints use `Authorization: Bearer <team JWT>`
- timestamps use RFC 3339 UTC format
- numeric ids are stable for the match

Common response envelope:

```json
{
  "status": "success",
  "data": {}
}
```

Common error envelope:

```json
{
  "status": "failed",
  "message": "human readable error"
}
```

## 4. Public API

### POST `/api/v2/authenticate`

Get a JWT for team-authenticated actions such as flag submission, service unlock, and reset.

Request body:

```json
{
  "email": "alpha.captain@example.com",
  "password": "alpha-secret"
}
```

Response `200`:

```json
{
  "status": "success",
  "data": "<team jwt>"
}
```

Response `403`:

```json
{
  "status": "forbidden",
  "message": "email or password is wrong."
}
```

Response `429`:

```json
{
  "status": "too many request",
  "message": "no bruteforce needed, calm down a little bit."
}
```

### GET `/api/v2/challenges`

Get all challenge definitions.

Response `200`:

```json
{
  "status": "success",
  "data": [
    {
      "id": 1,
      "name": "banking"
    },
    {
      "id": 2,
      "name": "chat"
    }
  ]
}
```

Response `429`:

```json
{
  "status": "too many request",
  "message": "no bruteforce needed, calm down a little bit."
}
```

### GET `/api/v2/services`

Get all challenge service addresses for all teams. These are the attack targets reachable through WireGuard.

Request header:

```text
Authorization: Bearer <team JWT>
```

Response `200`:

```json
{
  "status": "success",
  "data": {
    "1": {
      "101": ["10.80.1.11:10001"],
      "102": ["10.80.1.12:10001"]
    },
    "2": {
      "101": ["10.80.2.11:10002"],
      "102": ["10.80.2.12:10002"]
    }
  }
}
```

Response `429`:

```json
{
  "status": "too many request",
  "message": "no bruteforce needed, calm down a little bit."
}
```

Notes:

- first-level key is `challenge_id`
- second-level key is `team_id`
- each value is a list of reachable service endpoints
- endpoints are reachable only through the WireGuard game network
- this endpoint is lightly rate limited to protect the control plane from polling storms

### GET `/api/v2/scoreboard`

Get the current scoreboard snapshot.

Response `200`:

```json
{
  "status": "success",
  "data": [
    {
      "rank": 1,
      "team": "Team Alpha",
      "attack": 1840,
      "defense": 910,
      "sla": 780,
      "total": 3530,
      "delta": "+2"
    }
  ]
}
```

Response `429`:

```json
{
  "status": "too many request",
  "message": "no bruteforce needed, calm down a little bit."
}
```

Notes:

- this is a snapshot endpoint, not a websocket stream
- attack, defense, and SLA are split explicitly
- when `game-core` is configured, the snapshot is derived from authoritative persisted checker and submission state
- this endpoint is lightly rate limited to protect the platform from polling floods

### GET `/api/v2/game/status`

Get the current public match lifecycle state, scheduler state, and current authoritative tick.

Response `200`:

```json
{
  "status": "success",
  "data": {
    "match": {
      "state": "running",
      "started_at": "2026-03-10T10:00:00Z",
      "accepting_submissions": true
    },
    "current_tick": {
      "id": 248,
      "status": "completed"
    },
    "scheduler": {
      "state": "running",
      "interval_seconds": 60,
      "last_tick_id": 248
    },
    "total_ticks": 248
  }
}
```

Response `429`:

```json
{
  "status": "too many request",
  "message": "no bruteforce needed, calm down a little bit."
}
```

Notes:

- this endpoint is public and read only
- use it to decide whether participant submissions should still be offered in your tooling or dashboard
- `match.accepting_submissions` is the direct signal for whether new flag submissions are currently accepted
- `current_tick` and `total_ticks` let dashboards explain whether a stopped or finished match is paused mid-round or complete
- this endpoint is lightly rate limited to protect the control plane from polling floods

### GET `/api/v2/attacks`

Get the latest accepted attack events for dashboards or team tooling.

Query parameters:

- `limit` optional integer, default `12`
- `offset` optional integer, default `0`
- `attacker` optional case-insensitive substring filter for attacker team name
- `victim` optional case-insensitive substring filter for victim team name
- `service` optional case-insensitive substring filter for service name
- `tick_from` optional inclusive lower bound for tick number
- `tick_to` optional inclusive upper bound for tick number

Response `200`:

```json
{
  "status": "success",
  "data": {
    "items": [
      {
        "id": "atk-1",
        "attacker": "Team Alpha",
        "victim": "Team Delta",
        "service": "banking",
        "tick": 248,
        "verdict": "first valid submission accepted"
      }
    ],
    "limit": 12,
    "offset": 0,
    "total_count": 1,
    "has_prev": false,
    "has_next": false
  }
}
```

Response `429`:

```json
{
  "status": "too many request",
  "message": "no bruteforce needed, calm down a little bit."
}
```

Response `429`:

```json
{
  "status": "too many request",
  "message": "no bruteforce needed, calm down a little bit."
}
```

Notes:

- when `game-core` is configured, this feed is sourced from authoritative accepted-submission events
- this endpoint is newest-first and can be paged by `offset` for historical browsing
- text filters are applied before paging, so `total_count` reflects the filtered result set
- tick filters are inclusive, so `tick_from=240&tick_to=248` returns events from ticks 240 through 248

## Realtime Streams

The platform also exposes public SSE streams through the realtime gateway.

### GET `http://<realtime-host>/public/v1/scoreboard/stream`

Stream the latest scoreboard snapshot as server-sent events.

Stream payload:

```json
[
  {
    "rank": 1,
    "team": "Team Alpha",
    "attack": 1840,
    "defense": 910,
    "sla": 780,
    "total": 3530,
    "delta": "+2"
  }
]
```

### GET `http://<realtime-host>/public/v1/attacks/stream`

Stream the latest accepted attack-feed snapshot as server-sent events.

Stream payload:

```json
{
  "items": [
    {
      "id": "atk-1",
      "attacker": "Team Alpha",
      "victim": "Team Delta",
      "service": "banking",
      "tick": 248,
      "verdict": "first valid submission accepted"
    }
  ],
  "limit": 12,
  "offset": 0,
  "total_count": 1,
  "has_prev": false,
  "has_next": false
}
```

Notes:

- these are presentation streams only; scoring authority remains in `game-core`
- each event frame contains the full latest snapshot, not a delta patch

### POST `/api/v2/submit`

Submit one or more flags.

Request header:

```text
Authorization: Bearer <team JWT>
```

Request body:

```json
{
  "flags": [
    "<flag value 1>",
    "<flag value 2>",
    "<flag value 3>"
  ]
}
```

Response `200`:

```json
{
  "status": "success",
  "data": [
    {
      "flag": "<flag value 1>",
      "verdict": "flag is wrong or expired."
    },
    {
      "flag": "<flag value 2>",
      "verdict": "flag is correct."
    },
    {
      "flag": "<flag value 3>",
      "verdict": "flag already submitted."
    }
  ]
}
```

Response `400` when the contest has not started:

```json
{
  "status": "failed",
  "message": "contest has not started yet."
}
```

Response `400` when the contest is over:

```json
{
  "status": "failed",
  "message": "contest is over."
}
```

Response `403`:

```json
{
  "status": "forbidden",
  "message": "please authenticate before submit."
}
```

Response `429`:

```json
{
  "status": "too many request",
  "message": "no bruteforce needed, calm down a little bit."
}
```

Notes:

- the platform validates ownership, expiry, and first-valid semantics before accepting a flag
- when `game-core` is configured, submission validation is authoritative against persisted issued-flag rows
- flags should be batched to reduce per-request overhead

## 5. Team Service Management API

These endpoints extend the public submission API with service access and recovery actions.

### GET `/api/v2/team/services`

Get the current readable state of your own services.

Request header:

```text
Authorization: Bearer <team JWT>
```

Response `200`:

```json
{
  "status": "success",
  "data": [
    {
      "challenge_id": 1,
      "team_id": 101,
      "name": "banking",
      "endpoint": "10.80.1.11:10001",
      "status": "stable",
      "checker": "passing",
      "unlocked": true,
      "ssh_hint": "ssh root@10.80.1.11",
      "last_event": "nginx.conf hotfix deployed 2m ago",
      "reset_cooldown": "ready"
    }
  ]
}
```

Use this endpoint for:

- dashboard service cards
- unlock-state reads
- current SSH hint display
- reset cooldown and last event summaries
- this endpoint is lightly rate limited per authenticated team

### POST `/api/v2/services/{challenge_id}/unlock`

Submit proof that your team solved its own service and wants SSH access.
In the current platform contract, this proof is a team-specific value injected into that exact service container as `AD_PLATFORM_UNLOCK_PROOF`; teams are expected to extract it by solving the service itself.

Request header:

```text
Authorization: Bearer <team JWT>
```

Request body:

```json
{
  "proof": "<unlock proof>"
}
```

Response `200`:

```json
{
  "status": "success",
  "data": {
    "challenge_id": 1,
    "team_id": 101,
    "unlocked": true,
    "ssh_credential_ttl_seconds": 1800
  }
}
```

Response `400`:

```json
{
  "status": "failed",
  "message": "unlock proof is invalid."
}
```

Response `429`:

```json
{
  "status": "too many request",
  "message": "no bruteforce needed, calm down a little bit."
}
```

### POST `/api/v2/services/{challenge_id}/ssh-session`

Issue a short-lived root credential for the unlocked service.

Request header:

```text
Authorization: Bearer <team JWT>
```

Response `200`:

```json
{
  "status": "success",
  "data": {
    "host": "10.80.1.11",
    "port": 22,
    "username": "root",
    "password": "<one-time root password>",
    "expires_at": "2026-03-10T12:30:00Z",
    "connection_hint": "ssh root@10.80.1.11"
  }
}
```

Rules:

- only the owning team can request access
- the service must already be unlocked
- the SSH host is the same IP as the service endpoint and is reachable only through WireGuard
- the password is one-time and should be treated as shown-once secret material
- session issuance is audited
- session issuance is rate limited

### POST `/api/v2/services/{challenge_id}/reset/factory`

Factory-reset the service back to the organizer baseline image.

Request header:

```text
Authorization: Bearer <team JWT>
```

Response `200`:

```json
{
  "status": "success",
  "data": {
    "challenge_id": 1,
    "team_id": 101,
    "action": "factory_reset",
    "unlock_preserved": true
  }
}
```

Effect:

- destroys the current container
- removes the per-service persistent state volume
- recreates the service from the pristine organizer image
- preserves unlocked status for that team during the same match
- discards any filesystem changes made by the team inside that service
- when controller integration is enabled, runtime recreation happens before the platform state is updated
- this action is rate limited

### POST `/api/v2/services/{challenge_id}/reset/restart`

Optional non-destructive restart for teams that want to recover a process without discarding filesystem changes.

Request header:

```text
Authorization: Bearer <team JWT>
```

Response `200`:

```json
{
  "status": "success",
  "data": {
    "challenge_id": 1,
    "team_id": 101,
    "action": "restart"
  }
}
```

Effect:

- restarts the current service container when it exists
- preserves the service's attached persistent state volume and filesystem changes
- recreates it from baseline when the runtime is missing and controller integration is enabled
- this action is rate limited

## 6. WireGuard Rules

- every human member gets a unique WireGuard peer
- every peer can reach own and enemy published service endpoints
- WireGuard does not bypass service isolation
- SSH on port 22 is allowed only for the member's own team and only after unlock
- peer revocation does not affect other team members

Recommended addition:

- provide one optional `team-bot` WireGuard peer for automation hosts

## 7. Service Isolation Rules

The manual should make the network model explicit so teams know what assumptions are safe.

Rules:

- services are isolated from sibling services
- compromise of one service does not grant routable access to another service
- only published challenge ports are reachable from the game network
- service maintenance SSH is not part of the public attack surface

## 8. Automation Guidance

Teams should be able to automate at least:

1. authenticate and cache the JWT
2. fetch challenge ids and target endpoints
3. attack enemy services through WireGuard
4. batch-submit flags
5. unlock owned services
6. request SSH session data
7. trigger factory reset if a patch breaks SLA

Recommended client behavior:

- handle `429` with exponential backoff
- batch flags in one submission request
- treat service lists as dynamic and refresh periodically
- renew SSH credentials only when needed
- separate human and bot WireGuard peers

## 9. Stability Promise

To support team automation, the platform should keep these stable during a live event:

- `/api/v2` endpoint paths
- response envelope structure
- challenge ids
- team ids
- verdict strings or documented machine-readable verdict codes

If a breaking change is unavoidable, introduce `/api/v3` instead of mutating `/api/v2` during the competition.
