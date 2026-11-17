import { DEFAULT_THEME_ID, type ThemeDefinition } from "@/lib/theme-contract";
import { FIXTURE_THEMES } from "@/themes/fixtures.generated";
import { GENERATED_THEMES } from "@/themes/registry.generated";

export { DEFAULT_THEME_ID };

/*
 * Theme files import the kit, and the kit's stock views reach back here
 * through the slot machinery, so this module sits inside an import cycle.
 * Nothing reads the registry at module-evaluation time; it is resolved on
 * first use, which makes the cycle harmless regardless of import order.
 */
let cache: { list: readonly ThemeDefinition[]; byId: Map<string, ThemeDefinition> } | null = null;

function registry() {
  if (!cache) {
    const list = [...GENERATED_THEMES, ...FIXTURE_THEMES];
    cache = { list, byId: new Map(list.map((theme) => [theme.id, theme])) };
  }
  return cache;
}

/** Every registered theme. Test fixtures are only present in e2e builds. */
export function getThemes(): readonly ThemeDefinition[] {
  return registry().list;
}

export function isKnownThemeId(id: string | null | undefined): id is string {
  return typeof id === "string" && registry().byId.has(id);
}

/** Unknown ids (e.g. a theme folder removed after the organizer picked it) fall back to the default. */
export function resolveThemeId(id: string | null | undefined): string {
  return isKnownThemeId(id) ? id : DEFAULT_THEME_ID;
}

export function getTheme(id: string | null | undefined): ThemeDefinition {
  return registry().byId.get(resolveThemeId(id)) as ThemeDefinition;
}
