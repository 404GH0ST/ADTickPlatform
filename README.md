# AD Platform

A comprehensive Attack-Defense Tick CTF (Capture The Flag) platform. 

This platform manages periodic ticks, per-team isolated service instances, checker-driven validations (PUT/GET/SLA), stolen-flag submissions, live scoring, and WireGuard-based team network access.

## Features

- **Go Backend Services**: Highly concurrent control-plane services.
- **Next.js Frontend**: Modern dashboard for participants and organizers built with TypeScript, Bun, and shadcn/ui.
- **PostgreSQL**: Authoritative and auditable game state storage.
- **Redis**: Fast caching, locks, and live event queue management.
- **Docker Isolation**: Secure, per-team containerized service instances.
- **WireGuard Access**: Secure VPN access to the game network for participants.
- **Live Match Mechanics**: Real-time scoreboard, attack map, tick scheduling, and flag validation.
- **Dynamic Access Control**: Exploit-gated SSH access into owned service containers for patching.

## Documentation

Comprehensive documentation is available in the `docs/` directory:

- [System Architecture](docs/architecture.md)
- [Deployment: Debian/Ubuntu Host](docs/deployment-host.md)
- [Game Rules And Runtime Flows](docs/game-rules.md)
- [Participant Platform Manual](docs/platform-manual.md)
- [Participant OpenAPI Spec](docs/platform-api-v2.openapi.yaml)
- [Challenge Runtime Contract](docs/challenge-runtime.md)
- [Implementation Plan](docs/implementation-plan.md)
- [Organizer Admin API](docs/admin-api.md)
- [Final Rehearsal Checklist](docs/final-rehearsal-checklist.md)
- [Sample HTTP Challenge](examples/sample-http-challenge/README.md)
- [Sample LFI Challenge](examples/sample-lfi-challenge/README.md)

Participant web docs routes are available live at `/docs/participant`, `/docs/platform-api`, and `/docs/platform-api-v2.openapi.yaml`.

## Repository Layout

```text
.
├── apps/web                 # Next.js frontend application
├── deploy/                  # Docker Compose and deployment configurations
│   ├── caddy/               # Edge proxy configuration
│   ├── compose/             # Local and production compose files
│   └── docker/              # Dockerfiles for services
├── docs/                    # Platform documentation
├── examples/                # Example challenges
├── internal/                # Shared Go packages (platform, database, game network)
├── scripts/                 # Utility scripts for bootstrapping and smoke testing
└── services/                # Go backend microservices
```

## Quick Start (Local Development)

### Prerequisites

- [Docker](https://docs.docker.com/get-docker/) & Docker Compose
- [Go](https://go.dev/doc/install) 1.21+
- [Bun](https://bun.sh/)
- `make`

### Setup

1. Clone the repository and prepare your environment variables:
   ```bash
   cp .env.example .env
   ```

2. Start the local database stack (PostgreSQL and Redis):
   ```bash
   docker compose -f deploy/compose/dev.yml up -d
   ```

3. Run the backend services:
   ```bash
   make run-backend-stack-postgres
   ```

4. In a separate terminal, install frontend dependencies and start the web app:
   ```bash
   cd apps/web
   bun install
   bun run dev
   ```

## Production Deployment

Production uses a unified Docker Compose setup behind a Caddy edge proxy.

1. Configure production environments:
   ```bash
   cp deploy/compose/prod.env.example deploy/compose/prod.env
   # Edit prod.env with your specific secrets and domain (EDGE_SITE_ADDRESS)
   ```

2. Bring up the production stack:
   ```bash
   make up-prod
   ```
   *(Note: This automatically builds the Next.js standalone output prior to container creation).*

For host-level WireGuard and firewall enforcement, use the host-enforcement profile:
```bash
make preflight-prod-host
make up-prod-host
```

`make preflight-prod-host` now also verifies that host automation can obtain root non-interactively. Event-day commands such as `make go-live-check`, `make smoke-prod-host-enforcement`, and `make capture-prod-host-baseline` should be run either as `root` or by an operator with `sudo NOPASSWD` for the required host inspection commands.

## Testing & Validation

The repository includes extensive scripts for validating platform functionality.

- **Full CI Gate**: `make ci`
- **Backend Unit Tests**: `make test`
- **Frontend Typecheck**: `bun run web:typecheck`
- **Frontend Production Build**: `bun run web:build`
- **Browser Regression Tests**: `bunx playwright install chromium && make e2e`
- **Participant Flow Simulation**: `make smoke-participant`
- **Full Docker Challenge Smoke Test**: `make smoke-sample-challenge-docker`
- **Production Database Restore Drill**: `make smoke-prod-db-restore`

GitHub Actions runs `make ci` plus the Playwright browser regression suite on pushes and pull requests so the main branch stays aligned with the documented local validation path.
When the browser job fails, CI uploads `apps/web/playwright-report` and `apps/web/test-results` so traces, screenshots, videos, and snapshot diffs are available from the failed run.

If you need to reset the environment to a clean state during testing:
```bash
make bootstrap-clean-match
```

## Service Contracts

For challenges deployed to the platform, the service image must provide:
- A working `/bin/sh`
- An SSH daemon (`sshd`, `dropbear`)
- A password setter (`chpasswd`, `passwd`)
- Logic to expose or protect the injected `AD_PLATFORM_UNLOCK_PROOF` environment variable.

For more details, see the [Challenge Runtime Contract](docs/challenge-runtime.md).
