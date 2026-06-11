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
4. Download your own WireGuard peer config with `GET /api/v2/me/wireguard`.
   The response is the rendered client `.conf` text and the response headers
   carry the suggested filename via `Content-Disposition`.
5. Inspect your own service state with `GET /api/v2/team/services`.
6. Extract the unlock proof from your own service and submit it to
   `POST /api/v2/services/{challenge_id}/unlock`.
7. Retrieve the stable team SSH credential from
   `POST /api/v2/services/{challenge_id}/ssh-session`.
8. Use restart or factory reset when needed:
   - `POST /api/v2/services/{challenge_id}/reset/restart`
   - `POST /api/v2/services/{challenge_id}/reset/factory`
9. Submit captured flags with `POST /api/v2/submit`.

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

The participant service state surface now uses Faust-style labels:

- `ok`
- `recovering`
- `flag_not_found`
- `faulty`
- `down`

Those states come from the persisted per-tick service-state table. When a
checker reports a canonical state directly, that value is used; otherwise the
platform falls back to the current `put/get/check` phase mapping.

`GET /api/v2/game/status` returns the public match, scheduler, and current tick
state.

## Source Download

`GET /api/v2/challenges/{challenge_id}/source`

- Requires participant authentication.
- Streams the configured whitebox challenge source bundle.
- The payload may be a generated archive or a stored artifact file.

## WireGuard Config Download

`GET /api/v2/me/wireguard`

- Requires participant authentication; the team JWT must resolve to a player
  with a generated peer.
- Returns the rendered client `.conf` text for the authenticated player. The
  body is the full `[Interface]`/`[Peer]` block the participant imports into
  their WireGuard client.
- The `Content-Type` is `text/plain; charset=utf-8` and `Content-Disposition`
  is `attachment; filename="<peer-name>.conf"` so a browser saves the file
  with the correct name and never tries to render it inline.
- Organizer accounts have no peer config and receive `403`.
- A revoked peer still resolves to a config (so the participant can replace a
  previously downloaded file); the body reflects the current `status`.
- Rate limited per team alongside other credential download endpoints.

Notes:

- The config contains the per-player private key, preshared key, and assigned
  `Address` inside the WireGuard game network. Treat the file as a secret.
- Import the file directly into WireGuard, `wg-quick`, or any compatible
  client. The `Endpoint` is the configured gateway; the `AllowedIPs` cover
  the game-network subnets.

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

`sla_status` uses Faust-style state names:

- `ok`
- `recovering`
- `flag_not_found`
- `faulty`
- `down`
- `unknown`

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
    "sla_status": "ok",
    "sla_phase": "check",
    "sla_tick_id": 248,
    "sla_message": "service passed storage, retrieval, and functionality checks"
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
    "PLAYIT{Zm9vOmJhcjoxMjM0NTY3ODkw.YWJjZGVm}",
    "PLAYIT{eHl6OmJhcjoxMjM0NTY3ODkw.bXlrZXlz"]
  ]
}
```

> **Note:** The default flag format is `PLAYIT{<payload>.<mac>}`. Organizers can change
> the prefix from the admin dashboard (Platform Settings → Flag Format). The payload is
> base64url-encoded and the MAC is an HMAC-SHA256 signature computed by the platform.

Success:

```json
{
  "results": [
    {
      "flag": "PLAYIT{Zm9vOmJhcjoxMjM0NTY3ODkw.YWJjZGVm}",
      "status": "invalid",
      "detail": "flag is wrong or expired."
    },
    {
      "flag": "PLAYIT{eHl6OmJhcjoxMjM0NTY3ODkw.bXlrZXlz]",
      "status": "accepted",
      "detail": "flag is correct."
    },
    {
      "flag": "PLAYIT{eHl6OmJhcjoxMjM0NTY3ODkw.bXlrZXlz]",
      "status": "duplicate",
      "detail": "flag already submitted."
    }
  ],
  "accepted_count": 1,
  "rejected_count": 2
}
```

The HTTP request succeeds when the batch is processed. Each submitted flag gets
its own result entry. If the match is temporarily paused by the organizers, the entire submission request will fail with a `400 Bad Request` and the detail `"contest is temporarily paused."`.

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
