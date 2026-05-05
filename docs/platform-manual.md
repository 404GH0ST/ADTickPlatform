# Participant Platform Manual

This is the repo copy of the participant manual. The browser-facing manual lives
at `/docs/participant`, and the machine-readable contract lives at
`/docs/platform-api-v2.openapi.yaml`.

Use this document for the high-level workflow. Use the OpenAPI YAML for exact
request and response shapes.

## Current API Contract

- Successful API responses are raw JSON bodies.
- Error responses use `application/problem+json`.
- Programmatic participant actions use `Authorization: Bearer <team JWT>`.
- Service endpoints are reachable over the WireGuard game network, not via
  host-published ports.

## Participant Flow

1. Authenticate with `POST /api/v2/authenticate`.
2. Discover targets with:
   - `GET /api/v2/challenges`
   - `GET /api/v2/services`
   - `GET /api/v2/scoreboard`
   - `GET /api/v2/attacks`
   - `GET /api/v2/game/status`
3. Download whitebox source when available with
   `GET /api/v2/challenges/{challenge_id}/source`.
4. Inspect your own service state with `GET /api/v2/team/services`.
5. Extract the unlock proof from your own service and submit it to
   `POST /api/v2/services/{challenge_id}/unlock`.
6. Retrieve the stable team SSH credential from
   `POST /api/v2/services/{challenge_id}/ssh-session`.
7. Use restart or factory reset when needed:
   - `POST /api/v2/services/{challenge_id}/reset/restart`
   - `POST /api/v2/services/{challenge_id}/reset/factory`
8. Submit captured flags with `POST /api/v2/submit`.

## Authentication

`POST /api/v2/authenticate`

Request:

```json
{
  "email": "alpha.captain@example.com",
  "password": "alpha-secret"
}
```

Success:

```json
{
  "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.example",
  "token_type": "Bearer"
}
```

Failure:

```json
{
  "title": "Authentication failed",
  "status": 403,
  "detail": "email or password is wrong."
}
```

## Target Discovery

`GET /api/v2/challenges` returns the published challenge catalog, including
`has_source_download`.

`GET /api/v2/services` returns a nested map:

```json
{
  "1": {
    "101": ["10.80.1.11:10001"],
    "102": ["10.80.1.12:10001"]
  }
}
```

`GET /api/v2/scoreboard` returns an array of scoreboard rows.

`GET /api/v2/attacks` returns a paged attack feed:

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

`GET /api/v2/game/status` returns the public match, scheduler, and current tick
state.

## Source Download

`GET /api/v2/challenges/{challenge_id}/source`

- Requires participant authentication.
- Streams the configured whitebox challenge source bundle.
- The payload may be a generated archive or a stored artifact file.

## Owned Service State

`GET /api/v2/team/services`

Returns readable state for the authenticated team's services, including:

- `status`
- `checker`
- `unlocked`
- `ssh_hint`
- `last_event`
- `reset_cooldown`
- `sla_status`
- `sla_phase`
- `sla_tick_id`
- `sla_message`

Example:

```json
[
  {
    "challenge_id": 1,
    "team_id": 101,
    "name": "banking",
    "endpoint": "10.80.1.11:10001",
    "status": "stable",
    "checker": "passing",
    "unlocked": true,
    "ssh_hint": "ssh root@10.80.1.11 -p 22",
    "last_event": "nginx.conf hotfix deployed 2m ago",
    "reset_cooldown": "ready",
    "sla_status": "passing",
    "sla_phase": "check",
    "sla_tick_id": 248,
    "sla_message": "latest SLA cycle passed"
  }
]
```

## Unlock and SSH

`POST /api/v2/services/{challenge_id}/unlock`

Request:

```json
{
  "proof": "<unlock proof>"
}
```

Success:

```json
{
  "challenge_id": 1,
  "team_id": 101,
  "unlocked": true
}
```

`POST /api/v2/services/{challenge_id}/ssh-session`

Success:

```json
{
  "challenge_id": 1,
  "host": "10.80.1.11",
  "port": 22,
  "username": "root",
  "password": "<stable team root password>",
  "password_mode": "stable",
  "connection_hint": "ssh root@10.80.1.11 -p 22"
}
```

Notes:

- The service must already be unlocked.
- The SSH host is the same service IP.
- The credential is stable for the team and service until organizer-side policy
  changes it.

## Recovery Actions

`POST /api/v2/services/{challenge_id}/reset/restart`

Success:

```json
{
  "challenge_id": 1,
  "team_id": 101,
  "action": "restart"
}
```

`POST /api/v2/services/{challenge_id}/reset/factory`

Success:

```json
{
  "challenge_id": 1,
  "team_id": 101,
  "action": "factory_reset",
  "unlock_preserved": true
}
```

## Flag Submission

`POST /api/v2/submit`

Request:

```json
{
  "flags": [
    "FLAGv1.foo.bar",
    "FLAGv1.baz.qux"
  ]
}
```

Success:

```json
{
  "results": [
    {
      "flag": "FLAGv1.foo.bar",
      "status": "invalid",
      "detail": "flag is wrong or expired."
    },
    {
      "flag": "FLAGv1.baz.qux",
      "status": "accepted",
      "detail": "flag is correct."
    },
    {
      "flag": "FLAGv1.baz.qux",
      "status": "duplicate",
      "detail": "flag already submitted."
    }
  ],
  "accepted_count": 1,
  "rejected_count": 2
}
```

The HTTP request succeeds when the batch is processed. Each submitted flag gets
its own result entry.

## Error Shape

Errors use Problem Details:

```json
{
  "title": "Submission rejected",
  "status": 400,
  "detail": "contest is over."
}
```

## Source Of Truth

- Browser manual: `/docs/participant`
- Swagger UI: `/docs/platform-api`
- Raw OpenAPI YAML: `/docs/platform-api-v2.openapi.yaml`
