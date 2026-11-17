# Themes

The organizer picks one theme for the whole platform (Organizer → Game →
**Theme**). Participants only choose light / dark / system on top of it.

Each folder here is one theme. A theme can be anything from a few color
changes to a complete redesign of every participant page. To add a theme,
add a folder, then rebuild and redeploy the web app. The new theme then shows
up in the organizer picker.

```
themes/
  my-theme/
    theme.ts        required: metadata, default mode, view overrides
    theme.css       required: color tokens for light and dark, extra CSS
    components/     optional: your own views
    assets/         optional: images, fonts, referenced from theme.css
```

`registry.generated.ts` and `themes.generated.css` are written by
`scripts/generate-themes.mjs`. It runs automatically before `dev`, `build` and
`typecheck`, or by hand with `bun run themes`. Don't edit those files yourself.
Folders whose names start with `_` are ignored, so `_draft/` is a safe place to
work on a theme.

`themes/neubrutalism/` is a complete worked example. It replaces the shell
(participant and organizer, with a collapsible rail and a title banner on
every page), home, scoreboard,
services, flag form, login and the organizer scoreboard, restyles every other
stock view through the primitives' `data-slot` hooks, and defaults to light
mode. Its organizer scoreboard shows partial reuse: it draws its own standings
and keeps the stock freeze, audit and award panels from the kit, and its match
clock reuses the stock countdown logic through `useMatchClock`.

## 1. `theme.ts`

```ts
import { defineTheme } from "@/components/theme/kit";

export default defineTheme({
  id: "my-theme",                 // must equal the folder name: a-z, 0-9, -
  label: "My Theme",
  description: "One line shown in the organizer picker",
  swatch: { light: "oklch(...)", dark: "oklch(...)", accent: "oklch(...)" },
  defaultMode: "dark",            // optional: "light" | "dark" | "system" (default)
  components: {},                 // optional: view overrides, see section 3
});
```

`defaultMode` only applies until a participant picks a mode; their own choice
always wins.

## 2. `theme.css`

Scope every rule to your id. Light goes on the bare selector and dark on the
`[data-theme="dark"]` variant:

```css
:root[data-scheme="my-theme"] {
  color-scheme: light;
  --background: oklch(0.97 0.01 200);
  --primary: oklch(0.4 0.1 200);
}

:root[data-scheme="my-theme"][data-theme="dark"] {
  color-scheme: dark;
  --background: oklch(0.14 0.01 200);
  --primary: oklch(0.8 0.1 200);
}
```

Any token you leave out is inherited from Graphite (`themes/graphite/theme.css`
lists them all: surfaces, `--tone-*`, `--signal-*`, `--medal-*`,
`--attack-map-*`). If you set a token in the light block, set it in the dark
block too, or it carries into dark mode. Keep the contrast rules in
`DESIGN.md`.

Extra CSS (classes for your views, backgrounds, per-component tweaks) is
allowed. Put it under `@layer components { :root[data-scheme="my-theme"] ... }`
so utilities can still override it. `lib/themes.test.ts` fails if a theme's
CSS mentions another theme's `data-scheme`.

**Assets and fonts:** reference files next to `theme.css` with relative URLs.
The build fingerprints and bundles them:

```css
@font-face {
  font-family: "My Font";
  src: url("./assets/my-font.woff2") format("woff2");
  font-display: swap;
}
:root[data-scheme="my-theme"] body { font-family: "My Font", ui-sans-serif, sans-serif; }
:root[data-scheme="my-theme"] .my-canvas { background-image: url("./assets/pattern.svg"); }
```

## 3. Replacing views

Every participant page is split into **logic** (shared, never themed) and a
**view** (replaceable). A view override receives exactly the state and
callbacks the stock view gets, so a theme can redesign the page while live
updates, flag submission, SSH sessions, restart, factory reset, cooldowns and
filters keep working unchanged.

```ts
import { NbScoreboard } from "./components/nb-scoreboard";

export default defineTheme({
  // ...
  components: { Scoreboard: NbScoreboard },
});
```

| Slot | Where | Props (from `@/components/theme/kit`) |
|------|-------|----------------------------------------|
| `AppShell` | Chrome on every page: title, nav, header actions | `AppShellProps` (`surface` tells participant from organizer pages) |
| `Overview` | Participant home `/` | `OverviewViewProps` |
| `Scoreboard` | `/scoreboard` body | `ScoreboardViewProps` |
| `Services` | `/services` body | `ServicesViewProps` (includes the ready-made `flagSubmit` element) |
| `FlagSubmit` | Flag form, wherever it is placed | `FlagSubmitState` |
| `Attacks` | `/attacks` body: map, feed, filters | `AttacksViewProps` |
| `SSHSessionDialog` | SSH access dialog | `SSHSessionDialogProps` |
| `FactoryResetDialog` | Factory reset confirmation | `FactoryResetDialogProps` |
| `Login` | `/login` form | `ParticipantLoginState` |
| `ParticipantManual` | `/docs/participant` body | `ParticipantManualViewProps` |
| `ApiDocs` | `/docs/platform-api` body (`explorer` is the Swagger UI) | `ApiDocsViewProps` |
| `OrganizerOverview` | `/admin` home: section links plus the two panels below | `OrganizerOverviewViewProps` |
| `OrganizerTrustedState` | Trusted runtime state panel (`/admin`, `/admin/deployments`) | `TrustedState` |
| `OrganizerAnnouncements` | Match announcements panel | `AnnouncementsState` |
| `OrganizerTeams` | `/admin/teams` body | props of `DefaultOrganizerTeams` |
| `OrganizerPlayers` | `/admin/players` body | props of `DefaultOrganizerPlayers` |
| `OrganizerChallenges` | `/admin/challenges` body | props of `DefaultOrganizerChallenges` |
| `OrganizerDeployments` | `/admin/deployments` body | props of `DefaultOrganizerDeployments` |
| `OrganizerGame` | `/admin/game` body: match, scheduler, runtime health | props of `DefaultOrganizerGame` |
| `OrganizerScoreboard` | `/admin/scoreboard` body | props of `DefaultOrganizerScoreboard` |
| `OrganizerAttacks` | `/admin/attacks` body | props of `DefaultOrganizerAttacks` |
| `OrganizerSubmissions` | `/admin/submissions` table with delete | props of `DefaultOrganizerSubmissions` |
| `OrganizerAudit` | `/admin/audit` body | `OrganizerAuditViewProps` |

Every page in the app is covered: each participant page, each organizer page,
and the docs pages. (`/admin/api` returns 404 and the legacy `.md` URL is a
redirect, so neither has UI to theme.) Any slot you leave out uses the stock
view, restyled by your colors.

You can replace a whole page or just part of one. `ThemeSlotProps["OrganizerGame"]`
etc. give the exact prop types for the organizer tabs. The stock tabs carry
many controls, so you can either redraw everything from the props or render
the stock view (`DefaultOrganizerGame`) inside your own layout. Your
`AppShell` also receives `surface`: return `<DefaultAppShell {...props} />`
when `props.surface === "organizer"` if you want the organizer console
untouched.

### Rules for views

- Import from `@/components/theme/kit`. It holds the stock views (wrap them or
  reuse parts), helpers such as `describeService()`, UI primitives (`Button`,
  `Card`, `AppDialog`, `StatusBanner`, `ScoreboardTable`, `AttackMapPanel`, …),
  and all prop and data types. Deep imports may break when internals move.
- Views are client components: start every file in `components/` with
  `"use client";` (a unit test checks this). Don't fetch data or import
  server-only modules. Everything comes in through props.
- Use the provided callbacks for every action (`onRestart`, `onSelectReset`,
  `onSubmit`, …), and take enabled/disabled states from the props or from
  `describeService()`. That keeps cooldowns, locks and pending states
  identical across themes.
- Keep `AppearanceControls` somewhere in your `AppShell` so participants can
  still switch modes.
- Keep accessible names: label the flag textarea "Flags", and keep the button
  names ("SSH Access", "Restart", "Factory Reset", "Submit").
- If an override throws while rendering, the stock view is shown instead and
  the error is logged as `[theme:<id>] <Slot> override failed`. A throw
  during server rendering doesn't fail the page: React retries that view in
  the browser, which then falls back. A broken theme degrades to
  the default view instead of taking the page down.
- Shared behavior such as "request SSH credentials when the dialog opens"
  lives outside the views, so a themed view only has to render.

### Test fixtures

`themes/_fixtures/<id>/` holds test-only themes (for example `e2e-crash`, whose
views throw on purpose). They are registered only when the generator runs
with `AD_PLATFORM_THEME_FIXTURES=1`, which the Playwright build sets. They are
written to the gitignored `themes/fixtures.generated.ts` and never appear in a
normal build.

### Adding a new slot

Add the props to `ThemeSlotProps` in `lib/theme-contract.ts`, then render the
place through `<ThemeSlot name="…" fallback={StockView} props={…} />`
(`components/theme/theme-slot.tsx`). Move any logic the view used into a hook
first, so the props carry all the state.
