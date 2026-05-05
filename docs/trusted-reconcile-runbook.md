# Trusted Reconcile Runbook

This runbook is the operator path for the deployment/reconcile reliability lane.

## Goal

Prove that:

- public admin reconcile is healthy
- controller access truth is applied
- WireGuard truth is applied
- unlocked services grant SSH correctly
- locked services stay healthy while SSH remains denied
- injected SSH drift is repaired without manual firewall edits

## Prerequisites

Set these in `deploy/compose/prod.env` or export them before running the checks:

- `ADMIN_API_TOKEN`
- `AD_PLATFORM_EMAIL`
- `AD_PLATFORM_PASSWORD`
- `UNLOCK_PROOF_SECRET`

If you run host-mode networking, use:

- `deploy/compose/prod.host-enforcement.yml`

## Quick Truth Checks

From the host:

```bash
curl -fsS -X POST http://10.70.0.1/api/admin/deployments/reconcile
curl -fsS http://10.70.0.1/api/admin/access/status
curl -fsS http://10.70.0.1/api/admin/wireguard/status
```

Healthy output should show:

- reconcile returns `200`
- access status `state=applied`
- wireguard status `state=applied`

## Canonical Drift-Recovery Smoke

Run:

```bash
AD_PLATFORM_EMAIL='team@example.com' \
AD_PLATFORM_PASSWORD='password' \
./scripts/smoke-prod-host-enforcement.sh
```

This proves:

- participant unlock works
- trusted reconcile repairs a deleted SSH allow rule
- unauthorized peers do not gain SSH access

## Runtime Deploy/Reconcile Smoke

Use a known-good baseline/checker pair:

```bash
set -a; source deploy/compose/prod.env; set +a

AD_PLATFORM_API_URL='http://10.70.0.1' \
AD_PLATFORM_EMAIL='team@example.com' \
AD_PLATFORM_PASSWORD='password' \
RUNTIME_SMOKE_BASELINE_IMAGE='adplatform/sample-lfi:baseline' \
RUNTIME_SMOKE_CHECKER_IMAGE='adplatform/sample-lfi-checker:latest' \
./scripts/smoke-admin-runtime-flow.sh
```

Default behavior:

- creates a temporary `runtime-smoke-*` challenge
- validates it
- deploys it
- runs trusted reconcile
- confirms participant endpoint publication
- deletes the temporary challenge automatically on exit

To keep the temporary challenge for inspection:

```bash
RUNTIME_SMOKE_KEEP_CHALLENGE=1 ./scripts/smoke-admin-runtime-flow.sh
```

To clean up older `runtime-smoke-*` challenges that were created before auto-cleanup existed:

```bash
./scripts/cleanup-runtime-smoke-challenges.sh
./scripts/cleanup-runtime-smoke-challenges.sh --apply
```

## When Reconcile Fails

Check in this order:

1. live admin proxy:
   - `/api/admin/deployments/reconcile`
   - `/api/admin/access/status`
   - `/api/admin/wireguard/status`
2. direct admin API:
   - `/api/v2/admin/deployments/reconcile`
3. controller logs:
   - `docker logs ad-platform-prod-controller-service-1`
4. persisted status artifacts:
   - `/runtime/controller/access-status.json`
   - `/runtime/wireguard/status.json`
5. rendered rules:
   - `/runtime/controller/access.nft`
   - `/runtime/wireguard/peers.json`

## Expected Failure Meaning

- `503` on deployment reconcile:
  controller-owned trusted reconcile path is unavailable
- `502` on deployment reconcile:
  runtime converged but access or WireGuard truth was not established
- locked service returns health but denies SSH:
  expected
- unlocked service denies SSH:
  investigate controller access truth first
