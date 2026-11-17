// @ts-expect-error Bun exposes its test module at runtime without bundled TypeScript types.
import { describe, expect, test } from "bun:test";
import { existsSync, readdirSync, readFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

// @ts-expect-error: plain .mjs build script without type declarations
import { discoverThemes, renderCss, renderRegistry } from "../scripts/generate-themes.mjs";
import { THEME_ID_PATTERN } from "./theme-contract";
import { GENERATED_THEMES } from "../themes/registry.generated";
import { DEFAULT_THEME_ID, getTheme, resolveThemeId } from "./themes";

// Checks cover the shipped registry; e2e-only fixtures (themes/_fixtures) are
// intentionally partial and live in a separate, environment-dependent file.
const THEMES = GENERATED_THEMES;

const themesDir = join(dirname(fileURLToPath(import.meta.url)), "..", "themes");

describe("theme registry", () => {
  test("generated files are in sync with theme folders", () => {
    const ids: string[] = discoverThemes(themesDir);
    expect(readFileSync(join(themesDir, "registry.generated.ts"), "utf8")).toBe(renderRegistry(ids));
    expect(readFileSync(join(themesDir, "themes.generated.css"), "utf8")).toBe(renderCss(ids, themesDir));
  });

  test("default theme is registered first", () => {
    expect(THEMES[0]?.id).toBe(DEFAULT_THEME_ID);
  });

  test("ids are unique slugs matching their folder", () => {
    const ids: string[] = discoverThemes(themesDir);
    expect(THEMES.map((theme) => theme.id)).toEqual(ids);
    for (const theme of THEMES) {
      expect(theme.id).toMatch(THEME_ID_PATTERN);
      expect(theme.label.trim()).not.toBe("");
    }
  });

  for (const theme of THEMES) {
    test(`${theme.id}: theme.css scopes light and dark tokens to its own id`, () => {
      const cssPath = join(themesDir, theme.id, "theme.css");
      expect(existsSync(cssPath)).toBe(true);
      const css = readFileSync(cssPath, "utf8");
      expect(css).toContain(`:root[data-scheme="${theme.id}"] {`);
      expect(css).toContain(`:root[data-scheme="${theme.id}"][data-theme="dark"]`);
      // A theme must not restyle other themes.
      const foreign = [...css.matchAll(/data-scheme="([^"]+)"/g)].map((match) => match[1]);
      expect(foreign.filter((id) => id !== theme.id)).toEqual([]);
    });
  }

  // The root layout (a server component) imports the registry, so theme view
  // files must opt into the client; without it the build fails with an
  // unhelpful "useEffect in a Server Component" error.
  for (const theme of THEMES) {
    test(`${theme.id}: component files start with "use client"`, () => {
      const dir = join(themesDir, theme.id, "components");
      if (!existsSync(dir)) return;
      const files = readdirSync(dir, { recursive: true, encoding: "utf8" }).filter((file: string) => file.endsWith(".tsx"));
      const missing = files.filter((file: string) => !/^\s*["']use client["']/.test(readFileSync(join(dir, file), "utf8")));
      expect(missing).toEqual([]);
    });
  }

  test("unknown theme ids fall back to the default", () => {
    expect(resolveThemeId("does-not-exist")).toBe(DEFAULT_THEME_ID);
    expect(resolveThemeId(undefined)).toBe(DEFAULT_THEME_ID);
    expect(getTheme("paper").id).toBe("paper");
  });
});
