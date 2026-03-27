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

Latest release-candidate validation artifacts:

```bash
ls -1 .runtime/release-candidate-*
```

Latest attack-map load validation artifacts:

```bash
ls -1 .runtime/attack-map-load-*
```

Diff the current host against a saved baseline:

```bash
diff -u .runtime/go-live-check-<timestamp>/final-iptables-filter.txt .runtime/final-iptables-filter.txt
```
