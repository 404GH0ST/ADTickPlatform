# Organizer Admin API

This document defines the current organizer-facing API used by the Next.js `/admin` dashboard.

## Auth

Use a static organizer bearer token for now:

```text
Authorization: Bearer <admin token>
```

Set `ADMIN_API_TOKEN` to a non-empty secret before starting the admin API. The
production services reject example and development token values.

```text
ADMIN_API_TOKEN=<strong random admin token>
```

## Product ops endpoints

| Method | Path | Purpose |
| --- | --- | --- |
| `GET` | `/api/v2/announcements` | Public match announcements |
| `GET` / `POST` | `/api/v2/admin/announcements` | List / create organizer announcements |
| `DELETE` | `/api/v2/admin/announcements/{id}` | Delete an announcement |
| `POST` | `/api/v2/admin/import/teams` | Bulk create teams and nested players |
| `PUT` | `/api/v2/me/password` | Participant password change |

Bulk import creates each team **atomically** with its players (a player failure rolls that team back). Response status:

- `200` when at least one team was created (partial success may include `errors`)
- `422` when the body is valid but nothing was created
- `400` when the request body is invalid

Bulk import body:

```json
{
  "teams": [
    {
      "name": "Team Nova",
      "contact_email": "nova@example.com",
      "players": [
        {
          "display_name": "Nova Cap",
          "email": "nova.cap@example.com",
          "password": "secret-password",
          "role": "captain"
        }
      ]
    }
  ]
}
```

## Endpoints

### GET `/api/v2/admin/teams`

List teams with player counts and deployed challenge counts.

### POST `/api/v2/admin/teams`

Create a new team.

Request body:

```json
{
  "name": "Team Nova",
  "contact_email": "team-nova@example.com"
}
```

Behavior:

- creates the team row
- creates a zeroed scoreboard row
- replicates every already-published challenge into that team's service set

### DELETE `/api/v2/admin/teams/{team_id}`

Delete a team.

Behavior:

- removes the team record
- cascades deletion to all associated players, scoreboard entries, and flags
- notifies the controller to stop and remove all challenge service containers for this team

### GET `/api/v2/admin/players`

List players with their team binding and WireGuard peer name.

### POST `/api/v2/admin/players`

Create a player for a team.

Request body:

```json
{
  "team_id": 101,
  "display_name": "Alpha Captain",
  "email": "alpha.captain@example.com",
  "password": "bootstrap-secret",
  "role": "captain"
}
```

Valid roles are `member`, `captain`, or `organizer`. `organizer` accounts have access to the global admin dashboard. Team ID `0` can be used for team-less organizer accounts.

### DELETE `/api/v2/admin/players/{player_id}`

Delete a player.

Behavior:

- removes the player record
- removes the associated WireGuard peer configuration

### GET `/api/v2/admin/players/{player_id}/wireguard`

Fetch the current WireGuard config and peer metadata for one player.

### POST `/api/v2/admin/players/{player_id}/wireguard/rotate`

Rotate one player's WireGuard config and issue a fresh artifact.

### POST `/api/v2/admin/players/{player_id}/wireguard/revoke`

Revoke one player's current WireGuard peer without deleting the player account.

### GET `/api/v2/admin/wireguard/status`

Read the last known gateway sync state, current mode, peer counts, and most recent applied revision.

### POST `/api/v2/admin/wireguard/reconcile`

Render a fresh WireGuard gateway config snapshot from the authoritative player peer rows and apply it through the gateway service.

### GET `/api/v2/admin/access/status`

Read the controller-side service access policy sync state, including how many services currently have SSH open versus locked.

### POST `/api/v2/admin/access/reconcile`

Render and apply controller-side service access rules from current service unlock state and active team peers.

### GET `/api/v2/admin/challenges`

List all challenge definitions, including drafts and published challenges.

### POST `/api/v2/admin/challenges`

Create a draft challenge.

Request body:

```json
{
  "name": "proxy",
  "baseline_image": "registry.local/proxy:baseline",
  "checker_image": "registry.local/proxy-checker:latest",
  "service_port": 10007,
  "service_subnet_octet": 7,
  "egress_enabled": true
}
```

Behavior:

- `service_port` controls the target port that both participants and checkers use
- `service_subnet_octet` controls the `10.80.x.y` subnet assigned to that challenge
- `egress_enabled` (default `true`) controls whether the per-team service container
  is allowed to reach the public internet. When `false`, the controller emits an
  nftables/iptables DROP rule that blocks outbound traffic from that challenge's
  service containers to the `CONTROLLER_INTERNET_INTERFACE` (default `eth0`).
  Use this for air-gapped challenges that must not phone home or coordinate over
  external channels.
- if omitted:
  - `service_port = 10000 + challenge_id`
  - `service_subnet_octet = challenge_id`
  - `egress_enabled = true`

### DELETE `/api/v2/admin/challenges/{challenge_id}`

Delete a challenge.

Behavior:

- removes the challenge record
- cascades deletion to all team service instances and submitted flags for this challenge
- notifies the controller to stop and remove all containers for this challenge across all teams

### POST `/api/v2/admin/challenges/{challenge_id}/validate`

Run controller-side challenge package validation before any rollout state is created.

Behavior:

- loads the challenge metadata from authoritative platform state
- asks the controller to validate the baseline service image against the SSH patch contract
- asks the controller to validate the checker image against the standard checker entrypoint contract
- requires the checker validation path to advertise canonical service-state support
- returns a validation result even when the package is invalid, so organizers can inspect the failure reason

### POST `/api/v2/admin/challenges/{challenge_id}/deploy`

Publish the challenge and queue one service instance rollout for every team that does not already have a ready instance.

Behavior:

- re-runs controller-side challenge package validation and refuses the deploy when validation does not return `valid`
- inserts or updates a `team_service_states` row for each team that needs a rollout
- inserts or updates a `service_instances` row for each team with `runtime_status = queued`
- creates a `deployment_jobs` row
- marks older unfinished deployment jobs for the same challenge as `superseded`
- marks the challenge as published
- makes the challenge visible to the participant API

### GET `/api/v2/admin/deployments`

List deployment jobs with queued versus ready team counts.

### DELETE `/api/v2/admin/deployments/{deployment_id}`

Delete an inactive deployment job row from the organizer history list.

Behavior:

- refuses deletion while queued service instances still point at the job
- removes only the `deployment_jobs` record
- keeps the already deployed `service_instances` intact

### POST `/api/v2/admin/deployments/reconcile`

Run the trusted deployment converge path.

Behavior:

- asks the controller to converge runtime state, deployment rows, controller access truth, and WireGuard truth
- marks queued `service_instances` as `ready`
- updates `team_service_states` from `provisioning` to `stable`
- completes the matching `deployment_jobs`
- does not fall back to store-only success when the controller path is unavailable

Failure semantics:

- `503` when the controller-owned trusted reconcile path is unavailable
- `502` when runtime convergence finishes but access or WireGuard truth is still not established

### GET `/api/v2/admin/audit-logs`

List persisted control-plane audit entries.

Supported query parameters:

- `limit`
- `offset`
- `actor_type`
- `action`
- `target_type`
- `status`

Behavior:

- returns privileged organizer and participant actions recorded by the `api-gateway`
- current write coverage includes:
  - participant service unlock
  - participant SSH credential issuance
  - participant factory reset and restart
  - organizer team, player, and challenge creation
  - WireGuard inspect, rotate, revoke, and reconcile
  - controller access reconcile
  - challenge validation and deploy
  - deployment reconcile
  - match start and stop
  - tick advance
  - scheduler start and stop
  - scoring recompute

### GET `/api/v2/admin/game/scoring/audit`

Dry-run replay of persisted submissions and checker service state against the stored scoreboard snapshot.

Response shape:

- `status` as `ok` or `mismatch`
- `stored_rows` and `replayed_rows`
- `mismatch_count`
- `mismatches[]` with `team`, `field`, `stored`, `replayed`, `delta`, and optional `detail`

Behavior:

- does not update the stored scoreboard
- compares rank, attack, defense, SLA, and total points
- reports missing stored or replayed rows as row mismatches

### GET `/api/v2/admin/game/status`

Read the current match lifecycle state plus the latest persisted game tick and aggregate checker-run counts from `game-core`.

### GET `/api/v2/admin/game/match`

Read the persisted match lifecycle state.

Response shape:

- `state` as `not_started`, `running`, `paused`, or `finished`
- `started_at` optional RFC3339 timestamp
- `ended_at` optional RFC3339 timestamp
- `scheduled_start_at` optional RFC3339 timestamp from `GAME_CORE_MATCH_START_AT`
- `scheduled_end_at` optional RFC3339 timestamp from `GAME_CORE_MATCH_END_AT`
- `accepting_submissions` boolean
- `warmup` optional latest in-memory pre-match warmup result from `game-core`

The `warmup` field is diagnostic state only. It is not persisted across
`game-core` restarts.

### POST `/api/v2/admin/game/match/start`

Move the match into `running` state.

Behavior:

- runs pre-match warmup first when `GAME_CORE_WARMUP_REQUIRED=true` (default)
- returns `409 Conflict` with the full warmup result when required warmup fails
- opens authoritative participant flag submission
- sets `started_at` on first start
- runs one real tick immediately after the match enters `running` when
  `GAME_CORE_AUTO_TICK_ON_MATCH_START=true` (default)
- refuses restart after the match has already been finished
- when a configured schedule is already active, returns the effective running match state without mutating storage

The immediate startup tick is best-effort. If it fails after the match has
already entered `running`, the failure is logged by `game-core`; the scheduler
or a manual tick advance can still produce the next tick.

### PUT `/api/v2/admin/game/match/schedule`

Persist or clear the configured match window.

Behavior:

- accepts `scheduled_start_at` and `scheduled_end_at` as optional RFC3339 timestamps
- allows either field to be omitted or cleared independently
- rejects `scheduled_end_at` earlier than `scheduled_start_at`
- a background monitor persists `started_at` and `ended_at` automatically when the configured window opens or closes
- returns the effective match status after applying the updated window

### POST `/api/v2/admin/game/match/pause`

Move the match into `paused` state.

Behavior:

- pauses authoritative participant flag submission (submissions return 400 Bad Request)
- stops the tick scheduler if it is currently running
- blocks manual tick advancements
- triggers an automatic WireGuard gateway reconcile to restrict VPN network access to organizer-only peers

### POST `/api/v2/admin/game/match/resume`

Restore the match to `running` state from `paused`.

Behavior:

- re-opens authoritative participant flag submission
- restarts the tick scheduler automatically
- unblocks manual tick advancements
- triggers an automatic WireGuard gateway reconcile to restore VPN network access for all active participant peers

### POST `/api/v2/admin/game/match/stop`

Move the match into `finished` state.

Behavior:

- closes authoritative participant flag submission
- preserves `started_at`
- sets `ended_at`
- also stops the scheduler if it is currently running

### POST `/api/v2/admin/game/ticks/advance`

Advance one manual tick.

Behavior:

- requires the match to be in `running` state
- creates the next `game_ticks` row with `status = running`
- resolves current published `team x challenge` checker targets
- executes checker phases in order through `checker-runner`
- persists one `checker_runs` row per phase
- finalizes the tick with success, failed, and skipped phase counters

### GET `/api/v2/admin/game/checker-runs`

List recent persisted checker-run rows.

Query parameters:

- `limit` optional integer, default `25`
- `offset` optional integer, default `0`
- `tick_id` optional integer
- `team_id` optional integer
- `challenge_id` optional integer
- `phase` optional string
- `status` optional string

Response shape:

- `items`
- `limit`
- `offset`
- `total_count`
- `has_prev`
- `has_next`

Each checker run item also exposes the synthesized per-tick service state for the same
`team x challenge x tick` group:

- `service_state`
- `state_phase`
- `state_message`

### GET `/api/v2/admin/game/scheduler`

Read the automatic tick scheduler state, interval, and last run metadata from `game-core`.

### GET `/api/v2/admin/game/scheduler/events`

List persisted scheduler audit events.

Query parameters:

- `limit` optional integer, default `25`
- `offset` optional integer, default `0`
- `event_type` optional string
- `source` optional string
- `state` optional string

Response shape:

- `items`
- `limit`
- `offset`
- `total_count`
- `has_prev`
- `has_next`

Behavior:

- returns newest-first event rows from `game-core`
- includes organizer starts, restore starts after process boot, manual stops, and scheduled tick outcomes
- remains available across `game-core` restarts when using Postgres-backed state

### POST `/api/v2/admin/game/scheduler/start`

Start interval-driven automatic tick advancement in `game-core`.

Behavior:

- requires the match to be in `running` state
- refuses to start before the match begins or after it has been finished
- a background monitor also starts the scheduler automatically once a configured match window opens

### POST `/api/v2/admin/game/scheduler/stop`

Stop interval-driven automatic tick advancement in `game-core`.

## Controller Service

The repository now also exposes controller-facing endpoints:

- `GET /internal/v1/deployments`
- `POST /internal/v1/deployments/reconcile`
- `POST /internal/v1/challenges/validate`
- `GET /internal/v1/access/status`
- `POST /internal/v1/access/reconcile`
- `POST /internal/v1/teams/{team_id}/services/{challenge_id}/access/reconcile`
- `POST /internal/v1/teams/{team_id}/services/{challenge_id}/ssh-credential`
- `POST /internal/v1/teams/{team_id}/services/{challenge_id}/reset/factory`
- `POST /internal/v1/teams/{team_id}/services/{challenge_id}/restart`

These use the same bearer token for now and operate on the same Postgres-backed state.

The organizer Next.js route now preserves controller and gateway reconcile statuses directly; it does not flatten trusted reconcile failures into a generic success path.

Unlock operations in the participant API now also trigger controller-side access reconcile so SSH access can be opened for the owning team's active peers immediately after unlock.

SSH session issuance in the participant API now also calls the controller so the stable per-team/per-service `root` password is applied inside the target Docker container. In Docker mode the default implementation uses `docker exec` and a shell script that prefers `chpasswd` and falls back to `passwd root`.

Before deploy/restart/password-apply actions are accepted in Docker mode, the controller now probes the container for the baseline SSH contract:

- `/bin/sh`
- an SSH daemon binary such as `sshd` or `dropbear`
- a password setter such as `chpasswd` or `passwd`

This behavior is controlled through:

- `CONTROLLER_SSH_CONTRACT_MODE=verify|disabled`
- `CONTROLLER_SSH_PASSWORD_APPLY_MODE=docker-exec|disabled`

Organizer challenge validation now uses the same contract earlier through the packaging-time validation endpoint. In Docker mode the controller runs ephemeral containers from both images before any rollout job is queued.

Current package policy checks:

- service image:
  - `/bin/sh`
  - an SSH daemon binary such as `sshd` or `dropbear`
  - a password setter such as `chpasswd` or `passwd`
- checker image:
  - `/bin/sh`
  - a standard checker entrypoint available as `checker`, `/checker`, `/app/checker`, or `/checker.sh`

In `dry-run` mode the controller returns a `valid` result with a message that validation was assumed, so local control-plane flows remain usable without Docker.

When `CHECKER_RUNNER_INTERNAL_URL` is configured, the controller delegates checker-image validation to the checker-runner service instead of using its local fallback probe.

## Checker Runner Service

The repository now also exposes checker-runner endpoints:

- `POST /internal/v1/checkers/validate`
- `POST /internal/v1/checkers/execute`

Current checker-runner contract:

- images must be runnable with `/bin/sh`
- a checker entrypoint must exist as `checker`, `/checker`, `/app/checker`, or `/checker.sh`
- the entrypoint must support `put`, `get`, and `check`
- package validation accepts `validate` or `--help` as the checker self-check path

`POST /internal/v1/checkers/execute` currently forwards environment variables such as:

- `AD_PHASE`
- `AD_TEAM_ID`
- `AD_CHALLENGE_ID`
- `AD_TARGET`
- `AD_TICK_ID`
- `AD_FLAG`
- `AD_METADATA`

This is implemented and now wired into the manual `game-core` tick-advance path.

## Game-Core Service

The repository now also exposes `game-core` endpoints:

- `GET /internal/v1/game/status`
- `GET /internal/v1/game/match`
- `POST /internal/v1/game/match/start`
- `POST /internal/v1/game/match/pause`
- `POST /internal/v1/game/match/resume`
- `POST /internal/v1/game/match/stop`
- `PUT /internal/v1/game/match/schedule`
- `POST /internal/v1/game/ticks/advance`
- `POST /internal/v1/game/warmup`
- `GET /internal/v1/game/warmup`
- `GET /internal/v1/game/checker-runs`
- `GET /internal/v1/game/scheduler`
- `GET /internal/v1/game/scheduler/events`
- `POST /internal/v1/game/scheduler/start`
- `POST /internal/v1/game/scheduler/stop`
- `GET /internal/v1/game/scoreboard`
- `GET /internal/v1/game/attacks`
- `POST /internal/v1/game/scoring/recompute`
- `GET /internal/v1/game/scoring/audit`
- `POST /internal/v1/flags/submit`

Current behavior:

- `game-core` owns persisted `game_ticks` and `checker_runs`
- `game-core` also persists scheduler state and scheduler audit events
- required pre-match warmup runs checker `put` for every target before match start
- warmup uses real tick-0 flags but does not persist flags, ticks, checker runs,
  or scoreboard rows
- successful `put` phases issue persisted HMAC-backed flags in `issued_flags`
- participant submissions can be validated against `issued_flags` through `game-core`
- scoreboard rows are recomputed from persisted checker and submission state
- attack feed rows are read from authoritative `attack_events`
- organizer traffic reaches it indirectly through `api-gateway`
- tick advance can be manual or interval-scheduled through organizer controls
- the initial `not_started` to `running` transition can fire tick 1 immediately
  before the regular scheduler interval
- scheduler state can resume automatically after process restart when the last persisted state was `running`
- checker failures produce persisted `failed` rows
- checker executions may also report a canonical service state directly
- persisted `checker_service_states` prefer checker-reported states and fall back
  to the current phase-derived summary when a checker does not emit one
- later phases on the same target become `skipped`

This is now authoritative for tick history, checker history, flag validation, scoreboard recomputation, and attack-feed reads.

## Realtime Gateway Service

The repository now also exposes realtime gateway endpoints:

- `GET /public/v1/scoreboard/stream`
- `GET /public/v1/attacks/stream`
- `GET /admin/v1/game/scoreboard/stream`
- `GET /admin/v1/game/status/stream`
- `GET /admin/v1/game/checker-runs/stream`
- `GET /admin/v1/game/scheduler/events/stream`

Current behavior:

- the gateway polls public participant snapshots from `REALTIME_SOURCE_URL`
- the gateway also polls admin game snapshots using `REALTIME_SOURCE_ADMIN_TOKEN`
- scoreboard and attack-feed changes are published as SSE frames
- organizer game status, scoreboard, checker-run, and scheduler-event changes are also published as SSE frames
- each frame contains the full latest snapshot, not a partial delta
- the participant dashboard subscribes directly to public streams
- the organizer dashboard reaches admin streams through same-origin Next proxy routes so `ADMIN_API_TOKEN` stays server-side

## WireGuard Gateway Service

The repository now also exposes WireGuard gateway endpoints:

- `GET /internal/v1/wireguard/status`
- `POST /internal/v1/wireguard/reconcile`

In `dry-run` mode the service records sync status only.

In `files` mode it renders:

- the server config at `WIREGUARD_GATEWAY_CONFIG_PATH`
- a peer manifest at `WIREGUARD_GATEWAY_PEERS_PATH`
- optional firewall rules at `WIREGUARD_GATEWAY_RULES_PATH`
- a sync status artifact at `WIREGUARD_GATEWAY_STATUS_PATH`

Revoked peers are omitted from the rendered gateway config.
The rendered gateway config is a raw `wg syncconf` config, not a `wg-quick` config, so it intentionally omits directives such as `Address` and `SaveConfig`.

In `host` mode it uses the same rendered artifacts and then runs:

- `wg syncconf <interface> <config>`
- `nft -f <rules>` when `WIREGUARD_GATEWAY_FIREWALL_BACKEND=nftables`

The interface, binaries, timeout, and nftables table name are controlled through:

- `WIREGUARD_GATEWAY_INTERFACE`
- `WIREGUARD_GATEWAY_WG_BIN`
- `WIREGUARD_GATEWAY_APPLY_TIMEOUT`
- `WIREGUARD_GATEWAY_FIREWALL_BACKEND`
- `WIREGUARD_GATEWAY_NFT_BIN`
- `WIREGUARD_GATEWAY_FIREWALL_TABLE`

## Current Scope

Implemented now:

- organizer team creation
- organizer player creation
- generated WireGuard config issuance, inspection, rotate, revoke, and gateway reconcile/status flows
- host-mode gateway apply hooks for `wg syncconf` and `nftables`
- challenge draft creation
- packaging-time service and checker image validation
- deploy-to-all-teams queueing
- deployment job listing and reconcile
- controller-side service access policy rendering and reconcile
- controller-side stable SSH password application for Docker runtimes
- controller-side SSH contract verification for Docker service containers
- checker-runner internal validate/execute endpoints plus controller delegation for checker-image validation
- Next.js organizer dashboard integration
- controller service deployment endpoints

Not implemented yet:

- container reset/rebuild orchestration beyond initial `docker run`
- automated target-host validation still depends on running the provided host smoke on the real machine
- checker execution wired into authoritative tick scheduling and SLA persistence
- challenge undeploy, edit, or delete flows
