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
- Additional curated schemes (Ink, Paper, Moss), each with independent light/dark.
- Soft field grid only on the **Paper** scheme outer canvas gutter; content panels and other schemes stay solid (no body-wide notebook grid).
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

Use OKLCH for custom values. Schemes live in `apps/web/app/globals.css` under `data-scheme`.

| Scheme | Feel | Primary |
|--------|------|---------|
| **Graphite** (default) | Strict neutral minimal | Near-black / near-white ink |
| **Ink** | Cool modern ops | Restrained slate blue |
| **Paper** | Warm workroom desk | Deep brass |
| **Moss** | Calm technical | Muted sage |

Semantics (success / warning / danger / info) stay meaning-stable across schemes. Medals and signals use dedicated tokens; prefer `text-positive` / `tone-*` over raw Tailwind palette utilities.

Registered Tailwind signal colors (via `@theme`): `positive`, `negative`, `highlight`, `info` map to `--signal-*`. Use them for text, status dots, and light tints (`bg-positive/5`, `border-negative/30`). Do not hardcode Tailwind palette greens/ambers for scoreboard or SLA.

Rules:
- Accent marks selection, action, or state only — not decoration.
- State colors always pair with text or labels.
- No pure black/white, no neon, no gradients, no glassmorphism.
- **Highlight as text:** `--signal-highlight` must stay ≥4.5:1 on `--background` and `--card` in every scheme × mode. Prefer `text-highlight` for attention (paused, pending, SLA soft-fail), not body copy.
- **Medals:** `medal-gold|silver|bronze` chips only. Number color ≥4.5:1 on medal background; border ≥3:1 on card. Paper scheme uses warmer brass/copper medal ramps; Graphite/Ink/Moss share the default medal ramp.

### Appearance (scheme × mode)

Two independent axes:

1. **Scheme** — `graphite | ink | paper | moss` (`ad-platform-scheme` cookie + localStorage).
2. **Mode** — light / dark / **system** (`ad-platform-theme`).

**Default mode is System.** First visit does not persist a mode cookie; absence means follow `prefers-color-scheme`. Explicit Light, Dark, or System clicks write storage. Multi-day CTFs can track day/night without re-picking.

DOM:
- `data-scheme` — active scheme
- `data-theme` — resolved `light|dark` (never store the resolved value as preference)
- `data-theme-preference` — `light | dark | system`

Controls: `AppearanceControls` in the shell header (expand pickers for scheme + mode).

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

The shell owns the workroom feel: grid-paper background, hard bottom rules, compact navigation, and a clear title row. Page descriptions should stay factual and short.

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
