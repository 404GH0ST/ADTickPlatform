# Event Ready 2026-03-27

Validation date:

- `2026-03-27`

Validated git revision:

- `166b080` `166b0804c2dbb28ed48bac01153e22d849fbd188`

Important:

- the latest 2026-03-27 validation was rerun from the release-candidate wrapper with machine-readable summaries
- the release candidate should point at commit `166b0804c2dbb28ed48bac01153e22d849fbd188`

Validated host commands:

```bash
make validate-prod-release-candidate
```

What passed:

- Host preflight and compose validation for the host-enforcement profile
- Postgres backup and restore drill against the host stack
- Organizer-managed short-match rehearsal
- Host restart recovery drill for `controller-service`, `wireguard-gateway`, and `game-core`
- Attack-map load validation with checker-health validation and zero active runtime alerts after cleanup
- Final host baseline capture after the successful go-live run
- Machine-readable summary emission for the release-candidate and go-live artifact trees

Event-day references:

- `docs/operator-cheatsheet.md`
- `docs/final-rehearsal-checklist.md`
- `docs/deployment-host.md`

Validated artifact directories:

- `.runtime/release-candidate-20260327T193107Z`
- `.runtime/release-candidate-20260327T193107Z/prod-db-restore`
- `.runtime/release-candidate-20260327T193107Z/go-live-check`
- `.runtime/release-candidate-20260327T193107Z/go-live-check/attack-map-load`

Expected evidence from the release-candidate run:

- `.runtime/release-candidate-20260327T193107Z/README.txt`
- `.runtime/release-candidate-20260327T193107Z/summary.json`
- `.runtime/release-candidate-20260327T193107Z/git-revision.txt`
- `.runtime/release-candidate-20260327T193107Z/prod-env.sha256`

Expected evidence from the nested restore drill:

- `.runtime/release-candidate-20260327T193107Z/prod-db-restore/README.txt`
- `.runtime/release-candidate-20260327T193107Z/prod-db-restore/postgres-backup.sql`
- `.runtime/release-candidate-20260327T193107Z/prod-db-restore/postgres-backup.sql.sha256`
- `.runtime/release-candidate-20260327T193107Z/prod-db-restore/pre-restore-game-status.json`
- `.runtime/release-candidate-20260327T193107Z/prod-db-restore/post-restore-game-status.json`
- `.runtime/release-candidate-20260327T193107Z/prod-db-restore/pre-restore-scoreboard.json`
- `.runtime/release-candidate-20260327T193107Z/prod-db-restore/post-restore-scoreboard.json`
- `.runtime/release-candidate-20260327T193107Z/prod-db-restore/pre-restore-attacks.json`
- `.runtime/release-candidate-20260327T193107Z/prod-db-restore/post-restore-attacks.json`

Expected evidence from the nested go-live run:

- `.runtime/release-candidate-20260327T193107Z/go-live-check/README.txt`
- `.runtime/release-candidate-20260327T193107Z/go-live-check/summary.json`
- `.runtime/release-candidate-20260327T193107Z/go-live-check/short-match.env`
- `.runtime/release-candidate-20260327T193107Z/go-live-check/operations-status.json`
- `.runtime/release-candidate-20260327T193107Z/go-live-check/git-revision.txt`
- `.runtime/release-candidate-20260327T193107Z/go-live-check/prod-env.sha256`
- `.runtime/release-candidate-20260327T193107Z/go-live-check/final-iptables-filter.txt`
- `.runtime/release-candidate-20260327T193107Z/go-live-check/final-iptables-raw.txt`
- `.runtime/release-candidate-20260327T193107Z/go-live-check/final-nft-ruleset.txt`
- `.runtime/release-candidate-20260327T193107Z/go-live-check/final-wg-show.txt`
- `.runtime/release-candidate-20260327T193107Z/go-live-check/final-compose-ps.txt`

Expected evidence from the nested attack-map load validation:

- `.runtime/release-candidate-20260327T193107Z/go-live-check/attack-map-load/README.txt`
- `.runtime/release-candidate-20260327T193107Z/go-live-check/attack-map-load/attack-map-load.env`
- `.runtime/release-candidate-20260327T193107Z/go-live-check/attack-map-load/attack-map-load-report.json`
- `.runtime/release-candidate-20260327T193107Z/go-live-check/attack-map-load/operations-status.json`

Release note:

- create the release tag from commit `166b0804c2dbb28ed48bac01153e22d849fbd188` after committing this readiness note update
