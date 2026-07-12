# Implementation Plan

## Goal

Build the AD Platform in phases that reduce the highest operational and fairness risks first, rather than trying to ship all subsystems at once.

## Current Status

Completed:

- Phase 0 foundation
- Phase 1 authoritative game state (match, team, service models)
- Phase 2 participant API contract and dashboard wiring
- Phase 3 runtime plane (docker controller, isolation, wireguard)
- Phase 4 unlock, SSH, and reset semantics
- Phase 5 checker and scoring (Faust model, scoring worker, audit path)
- Phase 6 frontend and realtime (team dashboard, live attack map, scoreboard)
- Admin control plane security and resource management (DELETE support)
- Phase 7 hardening baseline: observability dashboards, smokes, backup/restore, trusted reconcile, release-candidate gates

## Hard Requirements

- one isolated runtime per `team x service`
- attack, defense, and SLA scoring
- checker, controller, and submission subsystems
- team-triggered factory reset that preserves unlock state for the same match
- exploit-gated SSH access to the owned service container
- WireGuard access for each team member
- stable participant API compatible with automation

## Risk Register

### 1. Score correctness

Failure mode:

- duplicate submissions
- wrong expiry handling
- non-deterministic score recomputation

Mitigation:

- Postgres as the source of truth
- append-only score events
- replayable tick finalization
- contract tests for submission verdicts and tick windows

### 2. Runtime isolation failure

Failure mode:

- compromise of one service reaches another service
- shared volume leakage
- SSH policy exposed on the attack path

Mitigation:

- dedicated network island per `team x service`
- default-deny east-west policy
- no shared writable volumes between services
- owner-only SSH firewalling on the service IP, with a future option for a separate maintenance subnet

### 3. Checker instability

Failure mode:

- timeouts skew SLA
- retries hide genuine failures
- slow checks drift across tick boundaries

Mitigation:

- bounded worker deadlines
- explicit attempt tracking
- per-phase latency metrics
- clear tick close semantics

### 4. Reset and unlock ambiguity

Failure mode:

- reset wipes patch state unexpectedly
- unlock behavior differs from the competition rules

Mitigation:

- document reset as factory restore
- preserve unlock state after reset for the same match
- add integration tests for unlock -> reset -> SSH session flow

### 5. Participant automation breakage

Failure mode:

- API responses change during the event
- target discovery format is unstable

Mitigation:

- versioned `/api/v2` API
- OpenAPI contract as the single source of truth
- no breaking changes without `/api/v3`

## Phases

## Phase 0: Foundation

Deliverables:

- monorepo scaffold
- Go service skeletons
- Next.js app scaffold
- shared config and HTTP utilities
- local Postgres and Redis compose stack
- implementation plan and participant API spec

Exit criteria:

- backend compiles with `go test ./...`
- compose file validates
- API contract is present in Markdown and OpenAPI form

## Phase 1: Authoritative Game State

Deliverables:

- database schema and migrations
- match, team, service, and tick models
- flag encoding and expiry rules
- tick scheduler skeleton in `game-core`

Exit criteria:

- ticks are generated deterministically
- flags can be created and validated offline
- schema covers replay and audit needs

## Phase 2: Participant API

Deliverables:

- JWT-based auth
- `/api/v2/challenges`
- `/api/v2/services`
- `/api/v2/submit`
- rate limiting and audit logging

Exit criteria:

- bulk flag submission matches the published verdict contract
- auth failures and rate-limit responses are stable
- automation can discover targets and submit flags end-to-end

## Phase 3: Runtime Plane

Deliverables:

- controller abstraction for Docker runtime
- service deployment model for `team x service`
- network isolation rules
- WireGuard peer provisioning per member and optional team bot

Exit criteria:

- each service is reachable only on the intended published port
- sibling services cannot talk to each other directly
- peer revocation is per member, not per team

## Phase 4: Unlock, SSH, and Reset

Deliverables:

- unlock proof validation
- stable per-team/per-service SSH credential issuance and owner-only SSH policy on the service IP
- factory reset preserving unlock state
- optional soft restart

Exit criteria:

- a team can unlock its own service, obtain SSH, patch it, and still SSH after factory reset
- SSH on port 22 is reachable only for the owning team after unlock

## Phase 5: Checker and Scoring

Deliverables:

- checker job queue and worker model
- PUT/GET/CHECK execution contract
- SLA scoring
- attack and defense scoring
- score snapshots per tick

Exit criteria:

- score recomputation from stored events matches online scoring
- checker failures produce predictable SLA impact

## Phase 6: Frontend and Realtime

Deliverables:

- Next.js team dashboard
- scoreboard
- live attack map
- service status and reset controls

Exit criteria:

- scoreboard renders from snapshots
- live updates are additive, not authoritative
- reset and unlock state are visible to participants

## Phase 7: Hardening

Deliverables:

- observability dashboards
- alerting for checker backlog and runtime drift
- backup and restore procedure
- load test for submissions and realtime fanout
- game-day runbook

Exit criteria:

- the platform tolerates service crashes and worker restarts without score loss
- contest operators can recover from partial outages with documented steps

## Build Order Recommendation

1. finish the scaffold and shared packages
2. implement database schema and migrations
3. implement the submission path and flag model
4. implement runtime deployment and WireGuard provisioning
5. implement unlock and reset semantics
6. implement checker execution and scoring finalization
7. implement frontend and realtime delivery
8. run failure drills before treating the platform as production-ready
