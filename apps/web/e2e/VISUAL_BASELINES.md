# Visual Baselines

These Playwright snapshots protect deliberate UI surfaces, not incidental page
height. Treat a snapshot update as part of the design change that caused it.

## Baseline Groups

- Participant baselines cover the player-facing service and attack-map panels.
- Presentation baselines cover the dense globe state used on large displays.
- Admin baselines cover organizer control cards where layout density matters.

## Update Rules

Update snapshots when the intended UI changes in spacing, hierarchy, color,
copy size, or component structure. Do not update snapshots to hide animation
flakiness, loading instability, clipped content, or unexpected overflow.

When a snapshot changes, the PR should mention the reason and the affected
group. If the change is caused by copy or selector churn rather than visual
design, prefer a stable `data-testid` or ARIA assertion instead of refreshing a
PNG.

## Local Commands

Run the affected visual baselines:

```sh
cd apps/web
PLAYWRIGHT_BROWSERS_PATH=$(pwd)/.cache/ms-playwright bun run playwright test e2e/visual-regression.spec.ts
```

Update intentional baselines:

```sh
cd apps/web
PLAYWRIGHT_BROWSERS_PATH=$(pwd)/.cache/ms-playwright bun run playwright test e2e/visual-regression.spec.ts --update-snapshots
```
