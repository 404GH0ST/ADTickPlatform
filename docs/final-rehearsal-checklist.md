# Final Rehearsal Checklist (Target 95%)

Use this runbook to validate the platform before the event.

Scope:
- Host-mode deployment (`up-prod-host`)
- WireGuard participant access
- Challenge deploy/checker/tick/submission/scoring flow
- Service restart/recovery behavior

Assumptions:
- `deploy/compose/prod.env` is configured
- Host WireGuard interface (`wg0`) is up
- Host has Docker, iptables, nftables, and required privileges
- Run `make preflight-prod-host` first so non-interactive root access is verified before the rehearsal
- `AD_PLATFORM_EMAIL` and `AD_PLATFORM_PASSWORD` in `deploy/compose/prod.env` point to a real participant account on this stack if you plan to use the smoke scripts
- `AD_PLATFORM_TEAM_ID` is blank or matches that participant's real team id

---

## 1. Host Enforcement Re-verify

Run:

```bash
make down-prod-host || true
make up-prod-host
make smoke-prod-host-enforcement
```

Pass criteria:
- `smoke-prod-host-enforcement` exits 0
- `controller-service` and `wireguard-gateway` do not restart-loop
- No `nft`/`iptables` backend errors in logs

Evidence:

```bash
docker logs ad-platform-prod-controller-service-1 | tail -n 50
docker logs ad-platform-prod-wireguard-gateway-1 | tail -n 50
sudo iptables -S FORWARD
sudo iptables -S DOCKER-USER
sudo iptables -S ADPLATFORM-WG-SERVICES
sudo iptables -t raw -S ADPLATFORM-WG-RAW || true
sudo wg show wg0
```

---

## 2. Client Reality Test (At Least 2 Devices)

For each participant device:
- Connect with generated WireGuard `.conf`
- Validate control-plane access:

```bash
curl -v http://10.70.0.1/healthz
curl -v http://10.70.0.1/api/v2/challenges
```

Validate service target access:

```bash
# Use live endpoint from /api/v2/services or /api/v2/team/services
curl -v http://<service-ip>:<service-port>/
```

Team workflow:
1. Team A unlocks own service
2. Team A requests SSH credential
3. Team B attacks Team A service
4. Team B submits stolen flag
5. Team B confirms Team A SSH access remains blocked on port `22`

Pass criteria:
- Health and challenge endpoints reachable over WG
- Service endpoint reachable for active services from both owner and non-owner teams
- SSH is locked before unlock and available after unlock for the owning team
- SSH to the same service IP on port `22` stays blocked for non-owner teams even when the service port is reachable
- Submit verdicts are correct (`correct`, `already submitted`, `wrong/expired`)

---

## 3. Full Match Rehearsal (Short Window)

Suggested scale:
- 2-4 teams
- 2 services
- 10-20 ticks

Run:

```bash
make smoke-prod-short-match
```

Optional:
- set `ORGANIZER_SMOKE_TARGET_TICKS=10` for a longer scheduler-driven rehearsal
- set `ORGANIZER_SMOKE_SCHEDULER_INTERVAL_SECONDS=2` or similar if you want the rehearsal to complete faster

What it does:
- resets the host stack to an organizer-managed clean state
- creates two teams and a sample challenge
- deploys the sample challenge
- sets a short scheduler interval for the rehearsal, then starts the match and scheduler
- waits for the requested number of ticks
- validates checker-run success and accepted attack scoring
- recomputes the authoritative scoreboard and checks the expected totals

Organizer/API checks:

```bash
curl -s -H "Authorization: Bearer $ADMIN_API_TOKEN" http://localhost/api/v2/admin/game/status | jq
curl -s -H "Authorization: Bearer $ADMIN_API_TOKEN" http://localhost/api/v2/admin/game/checker-runs | jq '.data.items[:10]'
curl -s -H "Authorization: Bearer $ADMIN_API_TOKEN" http://localhost/api/v2/admin/game/scoreboard | jq
```

Pass criteria:
- Ticks advance without scheduler stalls
- Checker phases (`put/get/check`) execute consistently
- Resets do not permanently break checker health
- Scoreboard changes match attack/defense/SLA expectations

---

## 4. Failure Drill (Recovery)

Simulate service restarts:

```bash
make smoke-prod-host-recovery
```

What it does:
- Restarts `controller-service`, `wireguard-gateway`, and `game-core`
- Waits for organizer/public endpoints to recover
- Re-runs `smoke-prod-host-enforcement`
- Verifies organizer access, wireguard, and game status endpoints still respond

Pass criteria:
- Startup restore reapplies access and wireguard policy
- No manual firewall patching required
- Participant and service traffic remain functional after restart

---

## 5. Operational Baseline Snapshot

Validate that the high-fanout attack feed and scoreboard path still meet release-candidate budgets:

```bash
make validate-attack-map-load
```

Pass criteria:
- the validation exits `0`
- `attack-map-load-report.json` shows `validation_status: "passed"`
- submission throughput stays above the configured floor
- submission p95, recompute, and attack-feed visibility stay within the configured thresholds

Artifacts:
- `.runtime/attack-map-load-*/attack-map-load-report.json`
- `.runtime/attack-map-load-*/attack-map-load.env`

Capture final known-good state:

```bash
make capture-prod-host-baseline
```

Pass criteria:
- Snapshots exist and match expected policy shape
- Operator can diff against these files during incident response

---

## 6. Database Restore Drill

Validate that the authoritative Postgres state can be backed up and restored on the live host profile:

```bash
make smoke-prod-db-restore
```

What it does:
- captures a SQL backup from the production Postgres container
- records pre-restore game status, scoreboard, attack feed, and compose state
- stops application services while keeping Postgres alive
- drops and recreates the database, then restores the captured backup
- brings the stack back up and compares post-restore snapshots to the pre-restore state

Pass criteria:
- restore exits 0
- match state, scheduler state, current tick, scoreboard, and accepted attack slice match before/after
- edge and organizer endpoints recover without manual intervention

Artifacts:
- `.runtime/prod-db-restore-*/postgres-backup.sql`
- `.runtime/prod-db-restore-*/postgres-backup.sql.sha256`
- `.runtime/prod-db-restore-*/pre-restore-*.json`
- `.runtime/prod-db-restore-*/post-restore-*.json`

---

## Stop/Go Decision

Go live only if all sections pass on:
1. Test host
2. Event host

Convenience:

```bash
make go-live-check
```

This runs sections 3, 4, 5, and the final baseline capture in sequence and writes the captured artifacts into a timestamped `.runtime/go-live-check-*` directory.
That directory also includes the generated short-match participant metadata used to chain the recovery drill against the same organizer-created state.
By default it also nests the attack-map load validation artifacts under `attack-map-load/`.
It also captures `operations-status.json` and expects the organizer runtime alert surface to report `healthy: true` with no active alerts at the end of the run.
It also writes `summary.json` so the final evidence set is machine-readable.
It also records the git revision and a `prod.env` SHA256 fingerprint for the validated run.
It also captures `game-core-metrics.prom`, `submission-service-metrics.prom`, `controller-service-metrics.prom`, and `realtime-gateway-metrics.prom` from the live host stack so the validation artifacts include scheduler/checker, submission-path, controller enforcement, and realtime health snapshots.

If any section fails:
- Fix
- Re-run from section 1
- Do not partially approve

Release-candidate convenience:

```bash
make validate-prod-release-candidate
```

This wraps preflight, compose validation, `up-prod-host`, the database restore drill, and `go-live-check` into one timestamped `.runtime/release-candidate-*` directory.
It also renders `event-ready-YYYY-MM-DD.md` into that artifact directory automatically.
Set `EVENT_READY_OUTPUT=docs/event-ready-YYYY-MM-DD.md` if you want the wrapper to mirror the generated note into the checked-in docs path during the same run.

Before tagging the release, verify the checked-in readiness note still matches the validated artifact tree:

```bash
EVENT_READY_DATE=YYYY-MM-DD make verify-event-ready-note
```

Final release gate:

```bash
make verify-release-candidate
```

This uses the latest `.runtime/release-candidate-*` artifact tree by default, requires the nested go-live runtime alert snapshot to still be healthy, requires the nested attack-map load validation report to still show `passed`, requires the nested `game-core`, `submission-service`, `controller-service`, and `realtime-gateway` metrics snapshots to still exist with the expected metric families, confirms the rendered event-ready note exists inside the artifact tree, and rejects the release if `git HEAD` does not match the validated commit.

Create the actual annotated release tag only after that passes:

```bash
RELEASE_TAG=vYYYY.MM.DD make tag-release
```

The tag step is stricter than the general verification step: it also requires the checked-in readiness note to match the validated artifact tree before it creates the annotated tag.

---

## Fast Triage Commands

```bash
make logs-prod-host
sudo iptables -L ADPLATFORM-WG-SERVICES -n -v --line-numbers
sudo iptables -t raw -L ADPLATFORM-WG-RAW -n -v --line-numbers || true
sudo tcpdump -ni wg0 'host 10.70.0.1 or net 10.80.0.0/16'
```
