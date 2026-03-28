# Operator Cheat Sheet

Use this on event day after the host stack and config are already frozen.

## Preflight

Known-good validation:

```bash
make validate-prod-release-candidate
sudo make go-live-check
```

If you do not want to run the full workflow as `root`, configure `sudo NOPASSWD` for the host inspection commands used by the automation. `make preflight-prod-host` now checks this before deployment.

Runtime alert snapshot:

```bash
curl -s -H "Authorization: Bearer ${ADMIN_API_TOKEN}" \
  "${AD_PLATFORM_API_URL}/api/v2/admin/operations/status" | jq
```

Manual alternatives:

```bash
make smoke-prod-short-match
make smoke-prod-host-recovery
make validate-attack-map-load
make smoke-prod-db-restore
make capture-prod-host-baseline
```

Full release-candidate wrapper:

```bash
make validate-prod-release-candidate
```

Render an event-ready note from a validated artifact tree:

```bash
EVENT_READY_OUTPUT=docs/event-ready-YYYY-MM-DD.md \
EVENT_READY_DATE=YYYY-MM-DD \
make render-event-ready-note
```

`make render-event-ready-note` defaults to the latest `.runtime/release-candidate-*` artifact. Set `EVENT_READY_ARTIFACT_DIR=.runtime/release-candidate-<timestamp>` if you want to render from an older validation run.

`make validate-prod-release-candidate` now renders `event-ready-YYYY-MM-DD.md` into the artifact directory automatically. Set `EVENT_READY_OUTPUT=docs/event-ready-YYYY-MM-DD.md` if you want the wrapper to also write the checked-in note in one pass.

Verify that the checked-in event-ready note still matches the validated artifact tree:

```bash
EVENT_READY_DATE=YYYY-MM-DD \
make verify-event-ready-note
```

Verify the full release-candidate gate before tagging:

```bash
make verify-release-candidate
```

This checks the latest `.runtime/release-candidate-*` evidence set, requires the nested go-live runtime alert snapshot to be healthy, requires the attack-map load report to have passed, requires the captured `game-core`, `submission-service`, `controller-service`, `realtime-gateway`, and `wireguard-gateway` metrics snapshots to exist and contain the expected metric families, requires the generated `operator-report.html` to exist, confirms the rendered event-ready note exists in the artifact tree, and by default rejects the release if `git HEAD` differs from the validated commit. Set `RELEASE_CANDIDATE_REQUIRE_HEAD_MATCH=false` only when auditing an older artifact tree.

If you also want this command to verify the checked-in readiness note, add:

```bash
RELEASE_CANDIDATE_REQUIRE_EVENT_READY_NOTE=true \
make verify-release-candidate
```

Create the annotated release tag from the validated commit only:

```bash
RELEASE_TAG=vYYYY.MM.DD \
make tag-release
```

Dry-run the tagging flow without creating the git tag:

```bash
RELEASE_TAG=vYYYY.MM.DD \
RELEASE_TAG_DRY_RUN=true \
make tag-release
```

`make tag-release` stays strict: it requires the checked-in event-ready note to match the validated artifact tree before it will create the annotated tag.

## Core Commands

Bring the host stack up:

```bash
make up-prod-host
```

Watch logs:

```bash
make logs-prod-host
```

Capture a fresh baseline snapshot:

```bash
make capture-prod-host-baseline
```

## Match Operations

Quick organizer status:

```bash
curl -s -H "Authorization: Bearer $ADMIN_API_TOKEN" http://localhost/api/v2/admin/game/status | jq
```

Recent checker runs:

```bash
curl -s -H "Authorization: Bearer $ADMIN_API_TOKEN" http://localhost/api/v2/admin/game/checker-runs | jq '.data.items[:10]'
```

Authoritative scoreboard:

```bash
curl -s -H "Authorization: Bearer $ADMIN_API_TOKEN" http://localhost/api/v2/admin/game/scoreboard | jq
```

Metrics snapshots:

```bash
curl -s http://127.0.0.1:8081/metrics | rg 'adplatform_game_core_|adplatform_http_'
curl -s http://127.0.0.1:8082/metrics | rg 'adplatform_submission_service_|adplatform_http_'
curl -s http://127.0.0.1:18084/metrics | rg 'adplatform_controller_service_|adplatform_http_'
curl -s http://127.0.0.1:8086/metrics | rg 'adplatform_realtime_gateway_|adplatform_http_'
curl -s http://127.0.0.1:18087/metrics | rg 'adplatform_wireguard_gateway_|adplatform_http_'
```

Backfill metrics snapshots into an existing go-live artifact directory:

```bash
GO_LIVE_METRICS_OUTPUT_DIR=.runtime/go-live-check-<timestamp> \
make capture-go-live-metrics
```

Summarize the latest validated artifact tree:

```bash
make summarize-validation-artifacts
```

Render the lightweight HTML operator report for the latest validated artifact tree:

```bash
make render-validation-report
```

Fail fast if the latest validated artifact tree needs operator attention:

```bash
make check-validation-alerts
```

## Recovery

Restart-recovery drill:

```bash
make smoke-prod-host-recovery
```

Database restore drill:

```bash
make smoke-prod-db-restore
```

Attack-map load validation:

```bash
make validate-attack-map-load
```

Firewall teardown if rules drift badly:

```bash
make firewall-cleanup
```

## Network Triage

Filter chain counters:

```bash
sudo iptables -L ADPLATFORM-WG-SERVICES -n -v --line-numbers
```

Raw chain counters:

```bash
sudo iptables -t raw -L ADPLATFORM-WG-RAW -n -v --line-numbers || true
```

WireGuard state:

```bash
sudo wg show wg0
```

Live traffic:

```bash
sudo tcpdump -ni wg0 'host 10.70.0.1 or net 10.80.0.0/16'
```

## Artifacts

Latest go-live run artifacts:

```bash
ls -1 .runtime/go-live-check-*
```

Inspect the final organizer runtime alert snapshot:

```bash
jq . .runtime/go-live-check-<timestamp>/operations-status.json
```

Inspect the machine-readable go-live summary:

```bash
jq . .runtime/go-live-check-<timestamp>/summary.json
```

Inspect the operator-facing go-live summary:

```bash
jq . .runtime/go-live-check-<timestamp>/operator-summary.json
xdg-open .runtime/go-live-check-<timestamp>/operator-report.html
```

Inspect the captured metrics snapshots:

```bash
rg 'adplatform_game_core_|adplatform_http_' .runtime/go-live-check-<timestamp>/game-core-metrics.prom
rg 'adplatform_submission_service_|adplatform_http_' .runtime/go-live-check-<timestamp>/submission-service-metrics.prom
rg 'adplatform_controller_service_|adplatform_http_' .runtime/go-live-check-<timestamp>/controller-service-metrics.prom
rg 'adplatform_realtime_gateway_|adplatform_http_' .runtime/go-live-check-<timestamp>/realtime-gateway-metrics.prom
rg 'adplatform_wireguard_gateway_|adplatform_http_' .runtime/go-live-check-<timestamp>/wireguard-gateway-metrics.prom
```

Latest release-candidate validation artifacts:

```bash
ls -1 .runtime/release-candidate-*
```

Inspect the operator-facing release-candidate summary:

```bash
jq . .runtime/release-candidate-<timestamp>/operator-summary.json
xdg-open .runtime/release-candidate-<timestamp>/operator-report.html
```

Check the operator-facing release status and print any derived alerts:

```bash
make check-validation-alerts
```

Inspect the machine-readable release-candidate summary:

```bash
jq . .runtime/release-candidate-<timestamp>/summary.json
```

Latest attack-map load validation artifacts:

```bash
ls -1 .runtime/attack-map-load-*
```

Diff the current host against a saved baseline:

```bash
diff -u .runtime/go-live-check-<timestamp>/final-iptables-filter.txt .runtime/final-iptables-filter.txt
```
