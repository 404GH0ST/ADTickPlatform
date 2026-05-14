# Operations Polish Todo

These items track the post-merge hardening pass for the attack map, scoring audit flow, and release checks.

- [x] Add an operator-triggered attack sound preview and keep the attack SFX preference visible in the attack map header.
- [x] Add presentation-mode controls for pausing, resuming, and stepping the featured route showcase.
- [x] Audit scoring automatically after completed ticks and after manual tick advancement.
- [x] Surface scoring audit status in the organizer Game quick actions card.
- [x] Persist scoring-worker audit status and export audit mismatch metrics.
- [x] Add a `make premerge` target that runs backend CI, frontend typecheck/build, and Playwright E2E.
- [x] Cover the new scoring audit and presentation controls in targeted tests.
- [x] Document operational response for scoring mismatches, stale realtime, and backup/restore confidence.
