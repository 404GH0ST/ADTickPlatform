# Game Rules And Runtime Flows

## 1. Purpose

This document defines the default game behavior that the architecture is designed to support. The exact numbers can be tuned later, but the platform should be built so these rules are configurable.

## 2. Match Model

- A match is divided into fixed-duration ticks.
- Every service is deployed once per team.
- Every `team x service` is isolated from the team's other services.
- Checkers run every tick against every `team x service`.
- Flags are planted by checkers and stolen by opposing teams.
- Teams patch their own service after unlocking direct access to that service container.
- Team members reach the game network through individual WireGuard peers.

Suggested defaults:

- tick duration: `120` seconds
- flag validity window: `3` ticks
- checker timeout per phase: `10` to `20` seconds
- submission grace period: `0` seconds unless explicitly configured

Recommended access defaults:

- one WireGuard config per team member
- one optional `team-bot` WireGuard config per team

## 3. Tick Phases

Each tick can be modeled as four logical phases:

1. `open`
   - the tick becomes current
   - jobs for checker runs are scheduled
2. `put`
   - checker places or refreshes service state and stores a flag
3. `get/check`
   - checker verifies retrieval, integrity, and availability
4. `finalize`
   - scores are computed once the submission window for the tick closes

The platform does not need to expose these as separate user-visible phases, but internal state should model them clearly.

## 4. Default Scoring Formula

The scoring engine should support per-service weights. Let `W(service)` be the configured weight.

### 4.1 Attack Score

For each accepted flag submission:

```text
attack_points = W(service)
```

Rules:

- only the first valid submission for a flag scores
- the attacker must not be the owning team
- the flag must still be within its validity window
- duplicate or replayed submissions score `0`

Per tick:

```text
attack_score(team, tick) = sum(attack_points for all accepted first-valid submissions by team in tick)
```

### 4.2 Defense Score

Defense should reward teams for retaining their issued flags.

For a specific `team x service x tick`:

```text
defense_ratio = 1 - (stolen_valid_flags / issued_valid_flags)
defense_score = W(service) * max(defense_ratio, 0)
```

Interpretation:

- if no issued flag is stolen, defense score is full weight
- if all issued flags are stolen, defense score is `0`
- partial theft reduces defense proportionally

### 4.3 SLA Score

SLA should reflect service availability and integrity as measured by the checker only.

Suggested phase weights:

- `put`: `0.3`
- `get`: `0.5`
- `check`: `0.2`

For each checker phase:

- success = `1`
- failure = `0`

Then:

```text
sla_ratio = 0.3 * put_ok + 0.5 * get_ok + 0.2 * check_ok
sla_score = W(service) * sla_ratio
```

If a service does not use a distinct `check` phase, redistribute the weight between `put` and `get`.

### 4.4 Total Score

```text
total_score(team, tick) = attack_score + sum(defense_score per service) + sum(sla_score per service)
```

## 5. Flag Lifecycle

## 5.1 Generation

Each flag should be derived from:

- match identifier
- owner team identifier
- service identifier
- issuing tick
- slot number
- server-side secret

Example logical format:

```text
FLAGv1.<payload>.<mac>
```

The payload should be enough to identify the owner and validity period after decoding.

## 5.2 Validity

Suggested rule:

- a flag issued in tick `N` is valid through the end of tick `N + 2`

This gives a `3`-tick validity window:

- issued at `N`
- valid in `N`
- valid in `N + 1`
- valid in `N + 2`
- expired starting at `N + 3`

## 5.3 Submission

Submission flow:

1. team submits a candidate flag
2. Submission Service parses and verifies signature/MAC
3. it checks ownership and expiry
4. it checks whether the submitting team is different from the owner
5. it checks whether the flag already has a winning submitter
6. it records acceptance or rejection

Accepted submissions create:

- a submission record
- an attack event
- a pending or immediate scoring event

## 6. Checker Behavior

Every service should provide a checker implementing at least:

- `put`: place flag and return metadata for retrieval
- `get`: retrieve the same flag using stored metadata
- `check`: optional availability or integrity checks beyond flag retrieval

Checker outputs should include:

- run id
- team id
- service id
- tick id
- phase
- success/failure
- latency
- error class
- debug artifact path if needed

The platform should treat checker timeouts as explicit failures.

## 7. Unlocking Patch Access

## 7.1 Intent

Teams must solve their own service before they can patch it.

## 7.2 Default Flow

1. organizer builds each service image with a team-specific unlock secret or exploit-gated unlock path
2. a team extracts its own unlock proof by solving the service
3. the team submits the proof to the platform
4. the platform validates the proof against expected team/service state
5. the platform issues short-lived SSH credentials for that exact `team x service` container

## 7.3 Patch Session

During an active patch session, a team may:

- SSH into its own service container
- modify code or patch files
- restart the service process
- verify that the service is healthy again

Recommended access controls:

1. credential is short-lived
2. access is scoped to one `team x service`
3. SSH uses the same service IP, but port 22 is opened only for the owning team after unlock over WireGuard
4. every unlock and session is audited

## 7.4 Unlock And Patch Policy

Recommended defaults:

- unlock persists for the whole match for that `team x service`
- patch credentials are short-lived and renewable
- every SSH session is audited
- teams can patch multiple times after unlock
- factory reset does not revoke unlock for that `team x service`

## 8. Reset Behavior

## 8.1 Team Reset

Player-facing reset should:

- stop and remove the current container
- remove the service's persistent state volume
- recreate the service from the organizer baseline image
- discard in-container patch changes
- preserve the same exposed endpoint identity if possible

This makes reset a factory restore action that teams can use when a patch breaks SLA.

Recommended default:

- keep unlock status for that `team x service` after reset during the same match

## 8.2 Optional Soft Restart

If you also want a non-destructive recovery action, add a separate restart control that:

- restarts the current service process or container
- preserves the attached service state volume
- preserves current filesystem modifications

## 8.3 Admin Factory Reset

Admin-only factory reset may still be useful for mass recovery or organizer intervention:

- recreate the service from the organizer baseline image
- optionally revoke unlock state or active SSH grants

Use this only for corrective operations.

## 8.4 Why Both Actions Can Be Useful

In the direct-container model, `factory reset` and `soft restart` solve different problems:

- `factory reset` recovers from a bad patch and restores SLA quickly
- `soft restart` recovers from a transient crash without discarding a working patch

If you want competition fidelity over patch preservation, keep factory reset as the primary team action and make soft restart optional.

## 9. Live Scoreboard

The scoreboard should present at least:

- total score
- attack score
- defense score
- SLA score
- per-service breakdown
- rank changes over time

Suggested refresh model:

- initial render from a score snapshot
- live updates over SSE or WebSocket

## 10. Live Attack Map

The live attack map should visualize accepted attack events only.

Each event should include:

- attacker team
- victim team
- service
- tick
- timestamp

Optional metadata:

- team location
- service category
- attack intensity aggregation windows

## 11. Operational Safeguards

- reject duplicate submissions quickly
- enforce per-team submission rate limits
- cap concurrent resets per team/service
- audit all patch unlock, SSH session, and reset operations
- audit WireGuard peer issuance and revocation
- keep scorer deterministic and replayable
- store enough checker context for dispute resolution

## 12. Minimum Viable Implementation

The smallest implementation that still satisfies the attack-defense tick model is:

1. fixed tick scheduler
2. service catalog with per-team Docker deployments
3. checker `put/get`
4. submission validator with duplicate protection
5. attack, defense, and SLA scoring
6. scoreboard
7. team-triggered factory reset
8. patch unlock and direct SSH access to the owned service container
9. per-member WireGuard access into the game network
10. live attack map

## 13. Open Policy Choices

These are game-design choices rather than architecture blockers:

- exact tick duration
- flag validity length
- whether patch unlock is permanent or per-session
- whether teams may reset during every tick or under cooldown
- whether each team also gets a dedicated automation WireGuard peer
- whether attack scoring is linear or normalized
- whether SLA penalties should be harsher for checker timeouts than ordinary failures
