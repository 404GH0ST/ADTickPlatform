# Operations Runbook

## Scoring Mismatch

1. Open `Admin -> Game -> Quick Actions` and run `Audit Scores`.
2. If the result is `mismatch`, capture the mismatch rows from the Scoreboard audit card before recomputing.
3. Run `Recompute Scores`, then run `Audit Scores` again.
4. If mismatches remain, export the runtime incident bundle and preserve the scoring-worker metrics snapshot.

Useful signals:

- `adplatform_scoring_worker_last_audit_mismatches`
- `adplatform_scoring_worker_last_audit_ok`
- `adplatform_scoring_worker_degraded`

## Stale Realtime

1. Check the organizer operations alerts and metrics cards for realtime sync failures.
2. Confirm `/healthz` and `/metrics` for `realtime-gateway`.
3. Refresh the admin page once to force a REST refetch.
4. Restart only the realtime gateway if REST data is current but stream updates remain stale.

## Backup And Restore Smoke

Before event day and after production data migrations:

1. Run `make smoke-prod-db-restore`.
2. Run `make validate-prod-release-candidate`.
3. Run `make premerge` before merging operational UI or scoring changes.

## Live monitoring (Prometheus + Grafana)

**Stack:** Prometheus scrapes `/metrics` from all eight backend services
plus the dedicated `adplatform_rate_limit_exceeded_total` counter from
`/internal/v1/rate-limit/metrics` on api-gateway. Grafana auto-loads
the **ADTickPlatform — Match Overview** dashboard and five alert rules
(service down, sustained 429 spike, submit failure rate, scheduler
stuck, scoring audit mismatch).

**Bring it up (local dev):**

```bash
docker compose -f deploy/compose/dev.yml up -d prometheus grafana
# or: make monitoring-up
```

Default endpoints:

- Grafana: <http://localhost:13000> (default user `admin` / password
  `adplatform`; override with `GRAFANA_ADMIN_USER` / `GRAFANA_ADMIN_PASSWORD`)
- Prometheus: <http://localhost:19090>

Backend services run on the host (`make run-backend-stack-postgres`).
Prometheus reaches them via `host.docker.internal:PORT`; if you move
the backend into the same compose network, switch the targets in
`deploy/prometheus/prometheus.yml` to service DNS names.

**Pre-match sanity check:**

1. `make monitoring-up` and wait 30s for the first scrape.
2. Open Grafana, confirm all eight service status panels are green.
3. In the *Rate Limiting* row, `429s/min by endpoint` should be flat
   (no traffic yet) and *Top endpoints* should be empty.
4. Trigger one submit manually from the participant shell to confirm
   the pipeline end-to-end; the *Submits/min* and *Verdict distribution*
   panels should respond.

**During the match (every 15-30 minutes):**

- **Top right of the dashboard → Alerts panel.** Any firing rule
  appears with a link to the alert definition. The five rules are
  conservative (1-3 minute `for:` window) so they only fire on real
  problems, not transient blips.
- **Rate Limiting row.** A steady rise on `submit` or `services`
  means a team is hammering past their budget. The 429 itself is
  fine (it's protecting the platform); sustained 30+/min for 3 minutes
  triggers the `AdplatformRateLimitSustainedSpike` alert.
- **Match State row.** `Scheduler` should flip between stopped /
  running around match start; if it stays `running` but `Current tick`
  doesn't advance, the scheduler is stuck (alert: `AdplatformSchedulerStuck`).
- **Operations row.** Spike in `controller-service` reconcile rate =
  services are flapping (probably being patched or attacked).
  `Scoring audit mismatches` must stay at 0; > 0 means the replay and
  the stored scoreboard disagree, which is a hard bug.

**Common failure modes:**

- `api-gateway` panel red but everything else green → JWT secret
  misconfigured. Check `TEAM_JWT_SECRET` is set and matches across
  `run-backend-stack-postgres`.
- `submission-service` panel red and submit Verdict distribution
  empty → submission-service is the bottleneck. Check
  `scoring_worker_degraded`; if true, game-core can't reach the store.
- All panels "No data" → Prometheus can't reach the host. On Linux
  without Docker 20.10+, replace `host.docker.internal:PORT` in
  `deploy/prometheus/prometheus.yml` with the host LAN IP (e.g.
  `192.168.1.42:PORT`).

**Clean shutdown / reset:**

```bash
make monitoring-down      # stop containers, keep data
make monitoring-clean      # also delete prometheus-data + grafana-data
```

Persistent data lives in Docker named volumes
(`ad-platform-local_prometheus-data`, `ad-platform-local_grafana-data`).
15 days of TSDB retention is configured; reduce in
`deploy/compose/dev.yml` if disk is tight.

### Production monitoring

Production runs the same Prometheus + Grafana stack but mounted on the
`ad-platform-prod` compose network. Key differences from dev:

- **Prometheus config** uses Docker service DNS targets
  (`api-gateway:8080`, `game-core:8081`, etc.) instead of
  `host.docker.internal:PORT`, because in prod every backend service
  runs in the same `control` network. Config is in
  `deploy/prometheus/prometheus.prod.yml`.
- **TSDB retention** defaults to 30d (configurable via
  `PROMETHEUS_RETENTION` in `prod.env`).
- **Ports** are bound to the host only (`127.0.0.1` semantics via the
  default `0.0.0.0` bind, but the edge Caddy does NOT route to them).
  Participants never see Grafana or Prometheus.
- **Credentials** come from `prod.env`:
  `GRAFANA_ADMIN_USER` / `GRAFANA_ADMIN_PASSWORD` /
  `PROMETHEUS_HOST_PORT` / `GRAFANA_HOST_PORT`.

Bring it up after `make up-prod`:

```bash
test -f deploy/compose/prod.env || cp deploy/compose/prod.env.example deploy/compose/prod.env
$EDITOR deploy/compose/prod.env   # set GRAFANA_ADMIN_PASSWORD, ports, retention
make monitoring-up-prod
```

This validates that `prod.env` exists, brings up Prometheus + Grafana,
and prints the local URLs. From a remote host, use an SSH tunnel:

```bash
ssh -L 13000:127.0.0.1:13000 -L 19090:127.0.0.1:19090 user@prod-server
# Then open http://localhost:13000 in your local browser
```

**Why two prometheus.yml files?** Prometheus' static_configs can't
read environment variables, so the target list is baked into the file
at compose-mount time. Dev uses `host.docker.internal` (services on
the host), prod uses service DNS (services in Docker). If you need
to add a service, edit both files.

Clean shutdown:

```bash
make monitoring-down-prod    # stop, keep data
make monitoring-clean-prod   # also delete volumes
```
