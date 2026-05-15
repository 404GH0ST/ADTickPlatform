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
