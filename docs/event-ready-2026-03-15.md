# Event Ready 2026-03-15

Known-good candidate tag:

- `event-ready-2026-03-15`

Validated host commands:

```bash
make smoke-prod-short-match
make smoke-prod-host-recovery
make capture-prod-host-baseline
make go-live-check
```

Recent release commits:

- `61068ea` `docs(ops): add operator cheat sheet`
- `7141790` `feat(ops): add one-shot go-live check`
- `827be0d` `feat(ops): speed up extended match rehearsals`
- `1a8a78b` `feat(ops): automate host rehearsal workflows`
- `3bd2b8b` `feat(admin): add scoreboard filter and sort controls`
- `297c6d6` `feat(game-core): reconcile scheduled match windows`

Event-day references:

- `docs/operator-cheatsheet.md`
- `docs/final-rehearsal-checklist.md`
- `docs/deployment-host.md`

Expected artifacts after final validation:

- `.runtime/go-live-check-*/README.txt`
- `.runtime/go-live-check-*/short-match.env`
- `.runtime/go-live-check-*/git-revision.txt`
- `.runtime/go-live-check-*/prod-env.sha256`
- `.runtime/go-live-check-*/final-iptables-filter.txt`
- `.runtime/go-live-check-*/final-iptables-raw.txt`
- `.runtime/go-live-check-*/final-nft-ruleset.txt`
- `.runtime/go-live-check-*/final-wg-show.txt`
- `.runtime/go-live-check-*/final-compose-ps.txt`
