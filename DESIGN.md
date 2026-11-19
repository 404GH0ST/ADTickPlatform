# AD Platform Design System

AD Platform uses a tactical workroom design system for Attack-Defense CTF operations. It adapts the parent reference's editorial grid, hard borders, and tactile surfaces into a dense event-control interface for organizers and participants.

## Design Context

### Users
Organizers run live events, watch service state, recover failures, and manage teams under time pressure. Participants monitor rankings, service status, and accepted attacks while actively playing. Both groups are technical and need fast scanning more than decorative presentation.

### Brand Personality
Calm, tactical, trustworthy.

The interface should feel like a prepared event desk: structured, legible, resilient, and clear when the room is busy.

### Visual Direction

Name: Tactical Workroom

Core traits:
- Modern minimal product chrome: hard 1px borders, compact controls, no gradients/glass/neon.
- **Default scheme: Graphite** — strict cool neutrals, near-monochrome primary.
- Additional schemes (Paper, Neubrutalism), each with independent light/dark.
- Soft field grid only on the **Paper** scheme outer canvas gutter; content panels stay solid. Graphite has no grid. Neubrutalism is the one exception with a body-wide graph-paper grid (see Neubrutalism exemptions).
- Dense operational tables and summaries that stay readable.

Anti-patterns:
- No purple or blue AI gradients.
- No glassmorphism.
- No hero sections inside the product.
- No oversized metric-card grids as the main layout.
- No gradient text.
- No colored side-stripe borders.
- No decorative cybersecurity theater.

## Tokens

### Typography

Current family: `IBM Plex Sans`.

Use one family across product UI. It is readable for tables, labels, code-adjacent content, and controls. Keep hierarchy tight:
- Page title: `text-2xl`, semibold.
- Section title: `text-base`, semibold.
- Body and controls: `text-sm`.
- Metadata: `text-xs`, medium or semibold.

### Color

Use OKLCH for custom values. Themes live in `apps/web/themes/<id>/theme.css` under `data-scheme` (see `apps/web/themes/README.md`).

| Scheme | Feel | Primary |
|--------|------|---------|
| **Graphite** (default) | Strict neutral minimal | Near-black / near-white ink |
| **Paper** | Warm workroom desk | Deep brass |
| **Neubrutalism** | Full redesign: ink outlines, hard offset shadows, pop fills (light default) | Electric blue chrome; sunflower yellow for primary keys and the title banner |

Semantics (success / warning / danger / info) stay meaning-stable across schemes. Medals and signals use dedicated tokens; prefer `text-positive` / `tone-*` over raw Tailwind palette utilities.

Registered Tailwind signal colors (via `@theme`): `positive`, `negative`, `highlight`, `info` map to `--signal-*`. Use them for text, status dots, and light tints (`bg-positive/5`, `border-negative/30`). Do not hardcode Tailwind palette greens/ambers for scoreboard or SLA.

Rules:
- Accent marks selection, action, or state only — not decoration.
- State colors always pair with text or labels.
- No pure black/white, no neon, no gradients, no glassmorphism.
- **Highlight as text:** `--signal-highlight` must stay ≥4.5:1 on `--background` and `--card` in every scheme × mode. Prefer `text-highlight` for attention (paused, pending, SLA soft-fail), not body copy.
- **Medals:** `medal-gold|silver|bronze` chips only. Number color ≥4.5:1 on medal background; border ≥3:1 on card. Paper scheme uses warmer brass/copper medal ramps; Graphite shares the default medal ramp; Neubrutalism uses flat pop fills with ink numbers.

### Neubrutalism exemptions

Neubrutalism is a deliberate, loud exception to the Tactical Workroom bans. It is allowed:
- A body-wide 44px graph-paper grid.
- A yellow title banner on every page. It stays compact (`text-3xl`/`text-4xl`, one row shared with the header keys), never a marketing hero.
- Hard, unblurred offset shadows and 3px ink outlines on cards, keys and inputs.
- In dark mode, big colour blocks become slate panels with a 10px coloured edge stripe (and an 8px yellow stripe on the banner).
- Display face Lexend instead of IBM Plex Sans.

Even here, these rules hold:
- **Status colours keep one meaning:** aqua = healthy, yellow = primary action or attention, pink = failing or danger (alert counts included), orange = paused (also used for the Pause key), lilac = held (locked SSH, frozen board) or "you".
- **Chrome colour is blue (rail, icon keys) and yellow (banner, logo).** Home destination blocks are white sheets with a blue icon key, never a status fill.
- **Selection is neutral:** the current nav key is white on the rail. Active tabs and chosen toggles are ink (cream in dark mode) via `.nb-selected`.
- **Score data has its own hues:** A violet, D cobalt, SLA slate-teal (`--nb-score-*`), as outlined stickers (`.nb-stat-a|d|sla`). The "new" rank sticker is plain `.nb-stat`.
- **The frozen scoreboard is an inverted ink bar** (`.nb-frozen`, a cream bar with a lilac edge in dark mode), unlike any chrome or status fill. The scoreboard page shows only this bar, with no second shell alert. Organizer pages show a "Board frozen" chip beside the title.
- **Podium fills follow the medal ramp** (yellow, lilac, orange) with ink numbers. The DOM order is rank order.
- **Contrast:** text on any fill is fully opaque and ≥4.5:1. Dark-mode `--nb-blue` stays dark enough for cream text.
- **Focus rings stay visible on every surface.** On the blue rail the ring is yellow.
- **Destructive keys** (`.button-danger-subtle`) show a pink wash at rest, not only on hover. Ghost destructive icons keep an ink outline, and disabled ones go dashed and grey.
- **The bounce easing is reserved for the first-blood banner.** Everyday hovers use `--nb-snap`, an ease-out curve with no overshoot.

### Appearance (theme × mode)

Two independent axes, owned by different people:

1. **Theme**: `graphite | paper | neubrutalism | …`, one folder per theme in `apps/web/themes/`. The **organizer** picks it platform-wide (Organizer → Game → Theme card). It is stored in `platform_settings.theme_id` and rendered server-side, so there is no per-user cookie and no flash.
2. **Mode**: light / dark / **system** (`ad-platform-theme`), chosen by each **participant**.

**Default mode is System.** Neubrutalism defaults to Light until the participant chooses a mode. A first visit leaves the mode preference unset. Choosing Light, Dark, or System saves the preference. System follows changes to `prefers-color-scheme` during the session.

DOM:
- `data-scheme`: active theme id (server-rendered from the organizer setting)
- `data-theme`: resolved `light|dark` (never store the resolved value as preference)
- `data-theme-preference`: `light | dark | system`

Controls: `AppearanceControls` in the shell header (mode only). A theme may replace the shell and any participant view (home, scoreboard, services, flag form, attacks, SSH/reset dialogs, login) through `components` in its `theme.ts`; views get the same state and callbacks as the stock ones, so features are unchanged. See `apps/web/themes/README.md`.

### Shape

The system is intentionally sharp:
- Default card radius: `rounded-sm`.
- Buttons and inputs: `rounded-sm`.
- Badges may use `rounded-sm`.
- Avoid pill shapes unless the component is an established tiny status chip.

### Layout

- Keep app chrome inside a practical max width.
- Use a top navigation rail because both participant and organizer surfaces are task tools.
- Use summary strips for operational counts instead of oversized cards.
- Tables remain dense and horizontally scrollable.
- Forms and filters use visible labels and compact inputs.

### Motion

- Use 150 to 200 ms transitions.
- Animate color and opacity for state feedback.
- Avoid layout choreography and decorative page-load animation.
- Base layer honors `prefers-reduced-motion: reduce` (scroll + transitions/animations).

## Components

### Shell

The shell owns the workroom feel: hard bottom rules, compact navigation, and a clear title row (the canvas grid is per scheme, see Color). Page descriptions should stay factual and short.

### Summary Strips

Summary strips expose counts and connection details as a single bordered grid. Labels are small and steady. Values should be strong enough to scan without becoming hero metrics.

### Cards

Cards are work slips:
- Full border on all sides.
- Warm card background.
- Compact header.
- No nested card-on-card compositions.
- Use separators, tables, or background shifts for internal structure.

### Buttons

- Primary is used for decisive actions.
- Outline is for navigation and utility.
- Ghost is for quiet controls.
- Destructive styling is reserved for destructive actions.
- Active state can move down 1px.
- Focus: `ring-2 ring-ring ring-offset-2 ring-offset-background` (full ring, not diluted opacity).

### Tables

Tables are the core operational surface:
- Strong header row.
- Compact cells.
- Subtle row hover.
- No decorative zebra striping unless data density requires it.

### Status

Use tone classes for success, warning, danger, and neutral. Pair color with readable copy. Do not use colored side bars.

## Accessibility

- Maintain visible focus rings on all interactive elements.
- Keep hit targets practical for event laptops and mobile checks.
- Preserve contrast in both themes.
- Do not hide controls on mobile; wrap or allow horizontal scroll.
