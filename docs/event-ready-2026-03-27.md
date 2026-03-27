# Event Ready 2026-03-27

Validation date:

- `2026-03-27`

Validated git revision:

- `131f66e` `131f66e25bafa7ae363f4e0955f6a764fdf21d26`

Important:

- the latest March 27 validation was rerun after the host-automation and observability commits were checkpointed
- the release candidate should point at commit `131f66e25bafa7ae363f4e0955f6a764fdf21d26`

Validated host commands:

```bash
make smoke-prod-db-restore
sudo make go-live-check
```

What passed:

- Postgres backup and restore drill against the host stack
- Organizer-managed short-match rehearsal
- Host restart recovery drill for `controller-service`, `wireguard-gateway`, and `game-core`
- Final host baseline capture after the successful go-live run

Event-day references:

- `docs/operator-cheatsheet.md`
- `docs/final-rehearsal-checklist.md`
- `docs/deployment-host.md`

Validated artifact directories:

- `.runtime/prod-db-restore-20260327T151905Z`
- `.runtime/go-live-check-20260327T162500Z`

Expected evidence from the restore drill:

- `.runtime/prod-db-restore-20260327T151905Z/README.txt`
- `.runtime/prod-db-restore-20260327T151905Z/postgres-backup.sql`
- `.runtime/prod-db-restore-20260327T151905Z/postgres-backup.sql.sha256`
- `.runtime/prod-db-restore-20260327T151905Z/pre-restore-game-status.json`
- `.runtime/prod-db-restore-20260327T151905Z/post-restore-game-status.json`
- `.runtime/prod-db-restore-20260327T151905Z/pre-restore-scoreboard.json`
- `.runtime/prod-db-restore-20260327T151905Z/post-restore-scoreboard.json`
- `.runtime/prod-db-restore-20260327T151905Z/pre-restore-attacks.json`
- `.runtime/prod-db-restore-20260327T151905Z/post-restore-attacks.json`

Expected evidence from the go-live run:

- `.runtime/go-live-check-20260327T162500Z/README.txt`
- `.runtime/go-live-check-20260327T162500Z/short-match.env`
- `.runtime/go-live-check-20260327T162500Z/git-revision.txt`
- `.runtime/go-live-check-20260327T162500Z/prod-env.sha256`
- `.runtime/go-live-check-20260327T162500Z/final-iptables-filter.txt`
- `.runtime/go-live-check-20260327T162500Z/final-iptables-raw.txt`
- `.runtime/go-live-check-20260327T162500Z/final-nft-ruleset.txt`
- `.runtime/go-live-check-20260327T162500Z/final-wg-show.txt`
- `.runtime/go-live-check-20260327T162500Z/final-compose-ps.txt`

Release note:

- create the release tag from commit `131f66e25bafa7ae363f4e0955f6a764fdf21d26` after committing this readiness note update
