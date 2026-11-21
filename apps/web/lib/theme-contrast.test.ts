// @ts-expect-error Bun exposes its test module at runtime without bundled TypeScript types.
import { describe, expect, test } from "bun:test";
import { readFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

import { GENERATED_THEMES } from "../themes/registry.generated";

// WCAG contrast for each theme's token pairs, computed from theme.css itself,
// so a token edit that drops text below 4.5:1 (or an outline below 3:1) fails
// here instead of in review. The resolver understands what theme tokens use:
// oklch(L C H [/ a]), var(--x) and color-mix(in oklab, A p%, B).

const themesDir = join(dirname(fileURLToPath(import.meta.url)), "..", "themes");

type Rgb = [number, number, number];
type Color = { rgb: Rgb; alpha: number };
type Tokens = Map<string, string>;

const clamp = (v: number) => Math.min(1, Math.max(0, v));

function oklabToLinear([L, a, b]: Rgb): Rgb {
  const l = (L + 0.3963377774 * a + 0.2158037573 * b) ** 3;
  const m = (L - 0.1055613458 * a - 0.0638541728 * b) ** 3;
  const s = (L - 0.0894841775 * a - 1.291485548 * b) ** 3;
  return [
    clamp(4.0767416621 * l - 3.3077115913 * m + 0.2309699292 * s),
    clamp(-1.2684380046 * l + 2.6097574011 * m - 0.3413193965 * s),
    clamp(-0.0041960863 * l - 0.7034186147 * m + 1.707614701 * s),
  ];
}

function linearToOklab([r, g, b]: Rgb): Rgb {
  const l = Math.cbrt(0.4122214708 * r + 0.5363325363 * g + 0.0514459929 * b);
  const m = Math.cbrt(0.2119034982 * r + 0.6806995451 * g + 0.1073969566 * b);
  const s = Math.cbrt(0.0883024619 * r + 0.2817188376 * g + 0.6299787005 * b);
  return [
    0.2104542553 * l + 0.793617785 * m - 0.0040720468 * s,
    1.9779984951 * l - 2.428592205 * m + 0.4505937099 * s,
    0.0259040371 * l + 0.7827717662 * m - 0.808675766 * s,
  ];
}

const encode = (v: number) => (v <= 0.0031308 ? 12.92 * v : 1.055 * v ** (1 / 2.4) - 0.055);
const decode = (v: number) => (v <= 0.04045 ? v / 12.92 : ((v + 0.055) / 1.055) ** 2.4);

/** Paint `top` over an opaque `bottom` in sRGB, as the browser composites. */
function over(top: Color, bottom: Color): Color {
  const rgb = top.rgb.map((v, i) => decode(encode(v) * top.alpha + encode(bottom.rgb[i]) * (1 - top.alpha))) as Rgb;
  return { rgb, alpha: 1 };
}

function mixOklab(a: Color, pa: number, b: Color | null): Color {
  if (!b) return { rgb: a.rgb, alpha: a.alpha * pa }; // mixing with transparent fades alpha
  const A = linearToOklab(a.rgb);
  const B = linearToOklab(b.rgb);
  return { rgb: oklabToLinear(A.map((v, i) => v * pa + B[i] * (1 - pa)) as Rgb), alpha: a.alpha * pa + b.alpha * (1 - pa) };
}

function splitTopLevel(input: string): string[] {
  const parts: string[] = [];
  let depth = 0;
  let start = 0;
  for (let i = 0; i < input.length; i += 1) {
    if (input[i] === "(") depth += 1;
    else if (input[i] === ")") depth -= 1;
    else if (input[i] === "," && depth === 0) {
      parts.push(input.slice(start, i).trim());
      start = i + 1;
    }
  }
  parts.push(input.slice(start).trim());
  return parts;
}

function resolve(value: string, tokens: Tokens, seen: string[] = []): Color | null {
  const v = value.trim();
  if (v === "transparent") return null;
  const varMatch = /^var\((--[\w-]+)\)$/.exec(v);
  if (varMatch) {
    const name = varMatch[1];
    const next = tokens.get(name);
    if (next === undefined || seen.includes(name)) throw new Error(`unresolved ${name}`);
    return resolve(next, tokens, [...seen, name]);
  }
  const ok = /^oklch\(\s*([\d.]+)\s+([\d.]+)\s+([\d.]+)\s*(?:\/\s*([\d.]+))?\s*\)$/.exec(v);
  if (ok) {
    const [L, C, H] = [Number(ok[1]), Number(ok[2]), Number(ok[3])];
    const h = (H * Math.PI) / 180;
    return { rgb: oklabToLinear([L, C * Math.cos(h), C * Math.sin(h)]), alpha: ok[4] ? Number(ok[4]) : 1 };
  }
  const mix = /^color-mix\(in oklab,(.*)\)$/s.exec(v);
  if (mix) {
    const [first, second] = splitTopLevel(mix[1]);
    const share = /^(.*?)\s+([\d.]+)%$/s.exec(first);
    if (!share) throw new Error(`color-mix without a share: ${v}`);
    const a = resolve(share[1], tokens, seen);
    if (!a) throw new Error(`transparent first color: ${v}`);
    return mixOklab(a, Number(share[2]) / 100, resolve(second, tokens, seen));
  }
  throw new Error(`unsupported color: ${v}`);
}

function luminance({ rgb: [r, g, b] }: Color) {
  return 0.2126 * r + 0.7152 * g + 0.0722 * b;
}

function contrast(fg: Color, bg: Color) {
  const flat = fg.alpha < 1 ? over(fg, bg) : fg;
  const [x, y] = [luminance(flat) + 0.05, luminance(bg) + 0.05];
  return Math.max(x, y) / Math.min(x, y);
}

/** The custom properties declared in the first rule whose selector is exactly `selector`. */
function block(css: string, selector: string): Tokens {
  const start = css.indexOf(`${selector} {`);
  if (start < 0) return new Map();
  const body = css.slice(css.indexOf("{", start) + 1, css.indexOf("\n}", start));
  const tokens: Tokens = new Map();
  for (const match of body.replace(/\/\*[\s\S]*?\*\//g, "").matchAll(/(--[\w-]+)\s*:\s*([^;]+);/g)) {
    tokens.set(match[1], match[2].trim());
  }
  return tokens;
}

function themeTokens(id: string, mode: "light" | "dark"): Tokens {
  const css = readFileSync(join(themesDir, id, "theme.css"), "utf8");
  const light = block(css, `:root[data-scheme="${id}"]`);
  if (mode === "light") return light;
  return new Map([...light, ...block(css, `:root[data-scheme="${id}"][data-theme="dark"]`)]);
}

type Pair = { fg: string; bg: string; min: number; label?: string };

const text = (fg: string, bg: string): Pair => ({ fg, bg, min: 4.5 });

// Pairs every theme ships. A pair whose tokens a theme leaves to the global
// defaults is skipped for that theme.
const SHARED: Pair[] = [
  text("var(--foreground)", "var(--background)"),
  text("var(--foreground)", "var(--card)"),
  text("var(--card-foreground)", "var(--card)"),
  text("var(--popover-foreground)", "var(--popover)"),
  text("var(--muted-foreground)", "var(--card)"),
  text("var(--muted-foreground)", "var(--background)"),
  text("var(--muted-foreground)", "var(--muted)"),
  text("var(--primary-foreground)", "var(--primary)"),
  text("var(--primary-ink)", "var(--card)"),
  text("var(--primary-ink)", "var(--background)"),
  text("var(--secondary-foreground)", "var(--secondary)"),
  text("var(--accent-foreground)", "var(--accent)"),
  ...["success", "warning", "danger", "info", "neutral"].map((tone) =>
    text(`var(--tone-${tone}-foreground)`, `var(--tone-${tone}-background)`),
  ),
  ...["gold", "silver", "bronze"].map((medal) => text(`var(--medal-${medal}-foreground)`, `var(--medal-${medal}-background)`)),
  ...["positive", "negative", "info", "highlight"].flatMap((signal) => [
    text(`var(--signal-${signal})`, "var(--card)"),
    text(`var(--signal-${signal})`, "var(--background)"),
  ]),
];

const NB_FILLS = ["yellow", "aqua", "pink", "orange", "lilac", "silver"];
const NB_SCORES = ["a", "d", "sla"];
const NB_TEXT_SIGNALS = ["positive", "negative", "info", "highlight"];
// Surfaces a row or well can sit on besides the plain card: the zebra shift,
// the row hover wash, and the "you" row (a full lilac fill by day, a 34%
// lilac wash over slate by night).
const NB_ROW_SURFACES = {
  light: ["var(--nb-zebra)", "var(--nb-hover-wash)", "var(--nb-lilac)"],
  dark: ["var(--nb-zebra)", "var(--nb-hover-wash)", "color-mix(in oklab, var(--nb-lilac) 34%, var(--nb-sheet))"],
};

// Neubrutalism's own surfaces: pop fills, score stickers, washes and shadows.
const NEUBRUTALISM: Record<"light" | "dark", Pair[]> = {
  light: [
    ...NB_FILLS.map((fill) => text("var(--nb-ink)", `var(--nb-${fill})`)),
    text("var(--nb-on-blue)", "var(--nb-blue)"),
    text("var(--nb-blue)", "var(--nb-paper)"),
    text("var(--muted-foreground)", "var(--nb-zebra)"),
    text("var(--muted-foreground)", "var(--nb-lilac)"),
    text("var(--nb-ink)", "var(--nb-hover-fill)"),
    text("var(--nb-ink)", "color-mix(in oklab, var(--nb-pink) 24%, var(--nb-sheet))"),
    ...NB_SCORES.map((s) => text(`var(--nb-score-${s})`, `color-mix(in oklab, var(--nb-score-${s}) 12%, var(--nb-sheet))`)),
    // text-primary-ink (current-team names, panel icons) reads in --signal-info,
    // and status text sits on zebra wells and the lilac "you" row as well as on cards.
    ...NB_ROW_SURFACES.light.flatMap((bg) => NB_TEXT_SIGNALS.map((signal) => text(`var(--signal-${signal})`, bg))),
    { fg: "var(--border)", bg: "var(--card)", min: 3, label: "outline on card" },
    { fg: "var(--ring)", bg: "var(--nb-paper)", min: 3, label: "focus ring on paper" },
    { fg: "var(--nb-yellow)", bg: "var(--nb-blue)", min: 3, label: "focus ring on rail" },
  ],
  dark: [
    ...NB_FILLS.map((fill) => text("var(--nb-ink)", `var(--nb-${fill})`)),
    text("var(--nb-on-blue)", "var(--nb-blue)"),
    text("var(--nb-chalk)", "var(--nb-zebra)"),
    text("var(--muted-foreground)", "var(--nb-zebra)"),
    text("var(--nb-ink)", "var(--nb-selected-fill)"),
    text("var(--nb-ink)", "color-mix(in oklab, var(--nb-yellow) 80%, var(--nb-chalk))"),
    text("var(--nb-ink)", "color-mix(in oklab, var(--nb-orange) 80%, var(--nb-white))"),
    // Night colour blocks inside a panel are a 34% wash of the fill over slate.
    ...NB_FILLS.flatMap((fill) => [
      text("var(--nb-chalk)", `color-mix(in oklab, var(--nb-${fill}) 34%, var(--nb-sheet))`),
      text("var(--muted-foreground)", `color-mix(in oklab, var(--nb-${fill}) 34%, var(--nb-sheet))`),
    ]),
    ...NB_ROW_SURFACES.dark.flatMap((bg) => NB_TEXT_SIGNALS.map((signal) => text(`var(--signal-${signal})`, bg))),
    text("var(--nb-yellow)", "var(--nb-zebra)"),
    text("var(--nb-pink)", "var(--nb-sheet)"),
    ...NB_SCORES.map((s) => text(`var(--nb-score-${s})`, `color-mix(in oklab, var(--nb-score-${s}) 12%, var(--nb-sheet))`)),
    { fg: "var(--ring)", bg: "var(--nb-paper)", min: 3, label: "focus ring on paper" },
    { fg: "var(--nb-chalk)", bg: "var(--nb-sheet)", min: 3, label: "outline on slate" },
    // Not a WCAG pair: a hard offset shadow must be visibly darker or lighter
    // than the paper it falls on, or the depth system disappears.
    { fg: "var(--nb-shadow)", bg: "var(--nb-paper)", min: 1.5, label: "shadow visible on paper" },
    { fg: "var(--nb-shadow)", bg: "var(--nb-sheet)", min: 1.5, label: "shadow visible on slate" },
  ],
};

for (const theme of GENERATED_THEMES) {
  for (const mode of ["light", "dark"] as const) {
    describe(`${theme.id} ${mode}: contrast`, () => {
      const tokens = themeTokens(theme.id, mode);
      const pairs = [...SHARED, ...(theme.id === "neubrutalism" ? NEUBRUTALISM[mode] : [])];
      for (const pair of pairs) {
        let fg: Color | null;
        let bg: Color | null;
        const own = !SHARED.includes(pair);
        try {
          fg = resolve(pair.fg, tokens);
          bg = resolve(pair.bg, tokens);
        } catch (error) {
          // A shared token may be left to the global defaults; a theme's own
          // pair must resolve, so a renamed token cannot silently drop a check.
          if (!own) continue;
          test(`${pair.fg} on ${pair.bg} resolves`, () => {
            throw error;
          });
          continue;
        }
        if (!fg || !bg || bg.alpha < 1) continue;
        const name = `${pair.label ?? "text"}: ${pair.fg} on ${pair.bg} ≥ ${pair.min}`;
        test(name, () => {
          expect(Number(contrast(fg, bg).toFixed(2))).toBeGreaterThanOrEqual(pair.min);
        });
      }
    });
  }
}
