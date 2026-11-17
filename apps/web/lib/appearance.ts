export const THEME_STORAGE_KEY = "ad-platform-theme";

export type ThemePreference = "light" | "dark" | "system";
export type ResolvedTheme = "light" | "dark";

export const DEFAULT_THEME_PREFERENCE: ThemePreference = "system";

export const THEME_CYCLE: ThemePreference[] = ["light", "dark", "system"];

export function isThemePreference(value: string | null | undefined): value is ThemePreference {
  return value === "light" || value === "dark" || value === "system";
}

export function resolveTheme(preference: ThemePreference): ResolvedTheme {
  if (preference === "system") {
    if (typeof window === "undefined") {
      return "light";
    }
    return window.matchMedia("(prefers-color-scheme: dark)").matches ? "dark" : "light";
  }
  return preference;
}

export function nextThemePreference(current: ThemePreference): ThemePreference {
  const index = THEME_CYCLE.indexOf(current);
  return THEME_CYCLE[(index + 1) % THEME_CYCLE.length];
}

/**
 * Applies the participant's light/dark preference. The theme itself
 * (html[data-scheme]) is set by the organizer and rendered by the server.
 */
export function applyThemePreference(preference: ThemePreference) {
  if (typeof document === "undefined") {
    return;
  }
  const root = document.documentElement;
  const resolved = resolveTheme(preference);
  root.dataset.theme = resolved;
  root.dataset.themePreference = preference;
}

export function persistThemePreference(preference: ThemePreference) {
  try {
    window.localStorage.setItem(THEME_STORAGE_KEY, preference);
  } catch {}
  document.cookie = `${THEME_STORAGE_KEY}=${preference}; path=/; max-age=31536000; samesite=lax`;
}

export function readThemePreferenceFromDom(): ThemePreference {
  const fromDom = document.documentElement.dataset.themePreference;
  if (isThemePreference(fromDom)) {
    return fromDom;
  }
  try {
    const stored = window.localStorage.getItem(THEME_STORAGE_KEY);
    if (isThemePreference(stored)) {
      return stored;
    }
  } catch {}
  const match = document.cookie.match(/(?:^|; )ad-platform-theme=(light|dark|system)/);
  if (match && isThemePreference(match[1])) {
    return match[1];
  }
  // Do not infer preference from resolved data-theme (light|dark): that would
  // collapse System into a locked mode after OS resolution.
  return DEFAULT_THEME_PREFERENCE;
}
