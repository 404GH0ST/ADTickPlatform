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
- A required pre-match warmup can run checker `put` once for every
  `team x service` before the match is allowed to start.

Suggested defaults:

- tick duration: `120` seconds
- flag validity window: `1` tick (fixed; not configurable)
- checker timeout per phase: `10` to `20` seconds
- submission grace period: none. A flag stops being accepted the moment the
  next tick starts, before that tick's new flags are planted. There is no
  setting to extend this.

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

Before the first scoring tick, `game-core` can run a pre-match warmup. Warmup
executes only the checker `put` phase for every target, using a real flag whose
`issued_tick` and `expires_tick` are both `0`. That flag is planted into the
service so the checker path matches a real tick, but the warmup does not persist
`issued_flags`, checker runs, ticks, or scoreboard rows. With
`GAME_CORE_WARMUP_REQUIRED=true` (default), match start is blocked unless the
warmup reaches `GAME_CORE_WARMUP_MIN_SUCCESS_RATE`. This also applies to a
scheduled start: when `scheduled_start_at` is reached, `game-core` runs the
warmup and starts the match only after it passes. Until then the match stays
`not_started` (no ticks, no submissions), the failed result is shown in the
organizer warmup status, and the warmup is retried every 30 seconds.

When the match first transitions from `not_started` to `running`, `game-core`
can immediately advance tick 1 (`GAME_CORE_AUTO_TICK_ON_MATCH_START=true` by
default). This makes scoreable flags available at match start instead of waiting
for the scheduler's first interval. Scheduled ticks continue from the next
interval, so the startup tick is not repeated by pause/resume.

## 4. Default Scoring Formula

The current platform now uses a Faust-style scoring model. Score is computed
per `team x service`, and team totals are derived by summing those service rows.

### 4.1 Attack Score

For each accepted flag submission on a flag `F`:

```text
attack_points(F) = 1 + 1 / capture_count(F)
```

Rules:

- accepted submissions are always by a non-owning team
- the flag must still be within its validity window
- `capture_count(F)` is the total number of accepted captures of the same flag
- the first accepted capture is worth `2.0`
- later accepted captures are worth less, but always more than `1.0`

Per tick:

```text
attack_score(team, tick) = sum(attack_points for all accepted submissions by team in tick)
```

### 4.2 Defense Score

Defense is a loss function over issued flags.

For a specific `team x service x tick`:

```text
defense_penalty(flag) = -(capture_count(flag) ^ 0.75)
```

Interpretation:

- an un-stolen flag contributes `0`
- a stolen flag contributes a negative value
- repeated steals of the same flag increase the penalty sublinearly

Per service:

```text
defense_score(team, service) = sum(defense_penalty for all flags issued to that team and service)
```

### 4.3 SLA Score

SLA is derived from canonical service states and multiplied by
`sqrt(active_team_count)`.

The platform now persists a per-tick service state for each team and challenge.
Checker integrations may report a canonical Faust-style state directly:

- `ok`
- `recovering`
- `flag_not_found`
- `faulty`
- `down`

When a checker does not report a canonical state explicitly, the platform falls
back to the current phase-derived mapping from persisted `put/get/check` runs:

- `OK = 1.0` when `put && get && check`
- `RECOVERING = 0.5` when `!put && get && check`
- all other phase combinations score `0.0`

Then:

```text
sla_score(team, service) = sum(tick_value) * sqrt(active_team_count)
```

SLA is then scored directly from the persisted service-state table. Explicit
checker-reported states take precedence over inferred states for the same tick.

### 4.4 Total Score

```text
total_score(team, tick) = attack_score + sum(defense_score per service) + sum(sla_score per service)
```

## 5. Flag Lifecycle

## 5.1 Generation

Each flag is derived from:

- owner team identifier
- service identifier
- issuing tick
- expiry tick
- server-side secret (`GAME_CORE_FLAG_SECRET`)

There is no match identifier or random component. With the same secret, the
same team, service and tick always produce the same flag string. A rehearsal
and the real event that both start from a fresh database (team IDs from 101,
challenge IDs from 1) would therefore issue identical flags. Anyone who saw
rehearsal flags could submit them again in the real event. Generate a new
`GAME_CORE_FLAG_SECRET` before every event.

Example logical format:

```text
PLAYIT{<payload>.<mac>}
```

The `<payload>` is the base64url encoding of
`<owner_team_id>:<challenge_id>:<issued_tick>:<expires_tick>`, signed with HMAC-SHA256
using the platform flag secret. The `PLAYIT` prefix and the curly braces
are the configurable flag format — the organizer can change the prefix
from the admin dashboard (see [Platform Settings](admin-api.md#platform-settings)),
and the codec re-wraps the signed payload accordingly. The default prefix
is `PLAYIT`.

The payload should be enough to identify the owner and validity period after decoding.

## 5.2 Validity

Default rule:

- a flag issued in tick `N` is valid only through the end of tick `N`

This gives a `1`-tick validity window:

- issued at `N`
- valid in `N`
- expired starting at `N + 1`

Warmup flags use the same signed flag format, but tick `0` flags are never
persisted to `issued_flags`. They are therefore not scoreable submissions.

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

Teams earn patch access by submitting at least one correct flag from another team on the same challenge.

## 7.2 Default Flow

1. A team captures another team's flag and submits it through the flag submission API or TCP submission service.
2. Game-core validates the flag and accepts the capture. Own-team, invalid, expired, and duplicate submissions do not create a grant.
3. Acceptance automatically unlocks SSH on the submitting team's own container for that challenge and reconciles network access.
4. The team opens **SSH Access** to retrieve its stable root credential.
5. Factory reset preserves the grant during the same match. Organizer maintenance or access reset clears it; a new accepted capture grants access again.

## 7.3 Patch Session

During an active patch session, a team may:

- SSH into its own service container
- modify code or patch files
- restart the service process
- verify that the service is healthy again

Recommended access controls:

1. credential is stable for the team and service until organizer-side policy changes it
2. access is scoped to one `team x service`
3. SSH uses the same service IP, but port 22 is opened only for the owning team after unlock over WireGuard
4. every unlock and session is audited

## 7.4 Unlock And Patch Policy

Recommended defaults:

- unlock persists for the whole match for that `team x service`
- patch credentials are stable for the team and service until organizer-side policy changes them
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

## 8.1.1 Cooldowns

Each owned service has independent participant cooldowns, enforced by the API
gateway and starting when the request is accepted:

- factory reset: one per 90 seconds
- restart: one per 30 seconds

A request rejected before anything runs (the service is not in an actionable
state, or the controller refuses the reset before touching the runtime) does not
start the cooldown. Once the runtime has been touched, the cooldown stands even
if the action fails.

A repeated request inside the window returns `429 Too Many Requests` with a
`Retry-After` header and a detail such as `Factory reset is on cooldown for this
service. Wait 42 seconds before trying again.` The services page shows the
remaining time on the button and in the service card.

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

## Flag retention and replacement penalties

A successful `put` creates an obligation to retain that exact flag until its next
platform-controlled replacement. Game-core saves the retrieval metadata and checks
it again at randomized times during the tick, before the next `put`, and before
match completion. The monitor polls every five seconds; checks are scheduled with
secret randomness between one eighth and one third of the configured tick interval
(or 30–90 seconds when an interval is unavailable). Pre-rotation verification runs
regardless of the next scheduled time. Late failures are attributed to the tick
that issued the flag, rather than the next healthy tick.

The check before match completion has a 3-minute budget, because its cost grows
with the number of team services. Checks that finish within the budget are scored
normally. If the budget runs out, the match still stops, and a
`final_retention_incomplete` scheduler event records how many targets were
covered. The order is shuffled, so the same services are not always the ones
left unchecked.

Any failed retention check permanently forfeits that service's SLA for the issuing
tick. A checker-confirmed missing or replaced value also incurs a defense deduction:

```text
n = number of other active teams eligible to play at confirmation time
penalty = ceil(max(10, 2*n + n^0.75 + 1))
```

This exceeds an upper bound on the defense loss avoided plus the attack points
denied by hiding one flag. Examples: 4 total teams → 10 points; 8 → 20 points;
50 → 118 points. SLA forfeiture is additional. The first confirmed amount is
persisted once per team/challenge/tick; repeated checks, later recovery, process
restarts, and later team-count changes do not multiply or erase that deduction.
Failures and the actual deduction appear in checker history and score replay.

Checker `get` exit code **42** means a reachable service explicitly reported a
missing flag or returned a value different from the originally issued value.
Transport failures and ordinary checker errors still lose the tick's SLA but do
not receive this additional confirmed-loss deduction. All bundled checkers support
this distinction; custom checker images must implement the exit-code contract.
Participant factory resets delete the state volume. A reset that reaches state
removal with an outstanding issued flag forfeits that flag's tick SLA and records
the same severe defense deduction. The deduction is reserved atomically with
checker-token rotation, then reversed if the controller confirms failure before
volume removal. It persists if recovery fails after removal or the removal outcome
is uncertain. An HTTP disconnect does not cancel a reset already in progress.
A checker put already in flight finishes recording
its flag before reset preparation. The deduction is recorded once per flag.
Successful reset responses include `penalty_points`, and the service's last event
explains the loss. Organizer resets rotate the per-instance checker token, invalidating
outstanding retention snapshots without this participant-reset deduction.
Maintenance, inactive/deferred instances, and stale checker snapshots are excluded
from checker-based penalty recording.

Submission truth remains the signed, issued flag in the platform database. A
replacement value never becomes authoritative, and a previously stolen genuine
flag stays valid for its original submission window. Invalid decoys cannot prove
which team successfully exploited a service, so they do not receive compensation
attack points. Retention checking measures storage availability; it cannot prove
malicious intent or stop container root from selectively serving honest responses
to the checker while patching the attack path.
