# Deployment: Debian/Ubuntu Host Enforcement

This guide explains how to prepare and run the AD Platform on a Debian or Ubuntu server using the "Host Enforcement" model (`prod-host`). 

In this model, the **Controller** and **WireGuard Gateway** run with `network_mode: host` and `NET_ADMIN` privileges to manage real Linux firewall rules (`nftables` and `iptables`) and the host's WireGuard interface.

## 1. System Requirements

- **OS**: Debian 12 (Bookworm) or Ubuntu 22.04/24.04 LTS.
- **Hardware**: At least 4GB RAM and 2 CPUs (scaling depends on team count).
- **Network**: A public IPv4 address and port `51820/udp` open for WireGuard.

## 2. Server Preparation

Install the required system packages:

```bash
sudo apt update
sudo apt install -y \
    curl \
    git \
    make \
    docker.io \
    docker-compose-v2 \
    wireguard-tools \
    nftables \
    iptables \
    iproute2 \
    jq
```

### Enable IP Forwarding
The server must act as a router to pass traffic between the WireGuard interface (`wg0`) and the Docker containers.

```bash
# Enable for the current session
sudo sysctl -w net.ipv4.ip_forward=1

# Make it persistent
echo "net.ipv4.ip_forward=1" | sudo tee -a /etc/sysctl.d/99-adplatform.conf
```

## 3. Platform Setup

1. **Clone the repository**:
   ```bash
   git clone <repository-url> ad-platform
   cd ad-platform
   ```

2. **Configure Environment**:
   ```bash
   cp deploy/compose/prod.env.example deploy/compose/prod.env
   nano deploy/compose/prod.env
   ```
   **Critical values to set**:
   - `EDGE_SITE_ADDRESS`: Your public domain or IP (e.g., `http://1.2.3.4`).
   - `ADMIN_API_TOKEN`: A long, random string.
   - `WIREGUARD_SERVER_ENDPOINT`: Your public IP (e.g., `1.2.3.4`).
   - `WIREGUARD_SERVER_PRIVATE_KEY`: Generate this in the next step.
   - `AD_PLATFORM_EMAIL` / `AD_PLATFORM_PASSWORD`: Set these to a real participant account if you want to use the host smoke scripts. The example alpha credentials only work on seeded/demo data.
   - `AD_PLATFORM_TEAM_ID`: Leave this empty unless you need an explicit consistency check. The smoke scripts derive the team from the authenticated participant account.

3. **Generate Keys**:
   ```bash
   make wg-host-keygen
   ```
   Copy the output keys into your `prod.env`.

## 4. Host Interface Setup

The platform expects a WireGuard interface named `wg0` to exist on the host.

```bash
# This installs /etc/wireguard/wg0.conf and brings the interface up
sudo make wg-host-setup
```

Verify the interface is active:
```bash
sudo wg show
```

## 5. Deployment

1. **Run Preflight**:
   Ensures all required binaries, environment variables, and ports are ready.
   ```bash
   make preflight-prod-host
   ```
   It also verifies that the operator can execute host-inspection commands non-interactively.
   For unattended rehearsals and event-day automation, run the stack as `root` or configure `sudo NOPASSWD` for the required `wg`, `iptables`, and `nft` reads.

2. **Start the Stack**:
   ```bash
   make up-prod-host
   ```

## 6. Bootstrap Administrator

Once the platform is running, create your first organizer account:

```bash
make create-admin PASSWORD=your-secure-password
```

This now also fetches and writes the organizer WireGuard client config by default to `.runtime/admin-wireguard/`.
Set a custom output directory with:

```bash
make create-admin PASSWORD=your-secure-password ADMIN_WIREGUARD_OUTPUT_DIR=/path/to/output
```

You can now log in at your configured `EDGE_SITE_ADDRESS` / `login` to access the dashboard.

## 7. Troubleshooting

### Firewall Conflicts
If you have existing rules that conflict with the platform, you can perform a total reset:
```bash
make firewall-cleanup
```

### Logs
Monitor the control plane and runtime enforcement:
```bash
make logs-prod-host
```
For the condensed event-day command list, see `docs/operator-cheatsheet.md`.

### Metrics
Every service now exposes a Prometheus-style `/metrics` endpoint. The highest-value runtime snapshots are:
```bash
curl -s http://127.0.0.1:8081/metrics | rg 'adplatform_game_core_|adplatform_http_'
curl -s http://127.0.0.1:8082/metrics | rg 'adplatform_submission_service_|adplatform_http_'
curl -s http://127.0.0.1:18084/metrics | rg 'adplatform_controller_service_|adplatform_http_'
curl -s http://127.0.0.1:8086/metrics | rg 'adplatform_realtime_gateway_|adplatform_http_'
curl -s http://127.0.0.1:18087/metrics | rg 'adplatform_wireguard_gateway_|adplatform_http_'
```
`game-core` exposes match, tick, checker-run, and scheduler gauges. `submission-service` exposes flag-submit counts, verdict classes, and attack-feed request metrics. `controller-service` exposes reconcile, SSH credential, and runtime action counters plus live access-policy status gauges. `realtime-gateway` exposes subscriber counts, cached snapshot sizes, and sync-health counters. `wireguard-gateway` exposes reconcile activity and current peer counts in addition to the shared HTTP request metrics.

### Recovery Drill
Before the event, validate restart recovery with the same host-mode stack:
```bash
make smoke-prod-host-recovery
```
This restarts `controller-service`, `wireguard-gateway`, and `game-core`, waits for the edge/admin endpoints to recover, and then re-runs the host enforcement smoke checks.

### Database Restore Drill
Validate that the authoritative Postgres state can be dumped, recreated, and restored without losing match state:
```bash
make smoke-prod-db-restore
```
This captures a SQL backup from the production Postgres container, stops the application services, drops and recreates the database, restores the backup, brings the stack back up, and compares pre/post game status, scoreboard, and accepted-attack snapshots. Artifacts are written into `.runtime/prod-db-restore-*`.

### Short Match Rehearsal
Run an organizer-managed match rehearsal against the host stack:
```bash
make smoke-prod-short-match
```
This resets the stack to a clean organizer-managed state, creates two teams plus a sample challenge, deploys it, starts the match and scheduler, waits for ticks to run, and checks checker/scoring behavior. For a longer scheduler run, set `ORGANIZER_SMOKE_TARGET_TICKS=10`.
The wrapper also sets a short scheduler interval for the rehearsal by default; override it with `ORGANIZER_SMOKE_SCHEDULER_INTERVAL_SECONDS`.

### One-Shot Go-Live Check
Run the main host validation flow end to end:
```bash
sudo make go-live-check
```
This runs the short match rehearsal, reuses the created participant account for the restart recovery drill, and then captures the final baseline snapshot into a timestamped `.runtime/go-live-check-*` directory.
The artifact directory also records the git revision, a `prod.env` SHA256 fingerprint, an `operator-summary.json` snapshot, and an `operator-report.html` status page for the validated run.

### Release-Candidate Validation
Run the full release-candidate host workflow:
```bash
make validate-prod-release-candidate
```
This wraps host preflight, compose validation, `up-prod-host`, the database restore drill, and `go-live-check` into a single `.runtime/release-candidate-*` directory so one command yields the full evidence set for a candidate build.
It also writes `operator-summary.json` and `operator-report.html` at the release-candidate root so operators can inspect the consolidated health summary without parsing the nested evidence manually.

### Capture Baseline
After the stack is in a known-good state, capture an incident-response baseline:
```bash
make capture-prod-host-baseline
```
This writes the current filter/raw iptables state, nft ruleset, WireGuard state, and compose service status into `.runtime/`.

### Common Issues
- **Docker Network Mismatch**: Ensure `CONTROLLER_DOCKER_NETWORK` in `prod.env` matches the actual name created by Docker Compose (usually `ad-platform-prod_control`).
- **IPv4 Forwarding**: If participants can connect to VPN but cannot reach services, double-check `net.ipv4.ip_forward=1`.
