export const THEME_STORAGE_KEY = "ad-platform-theme";
export const THEME_COOKIE_PATTERN = /(?:^|;\s*)ad-platform-theme=(dark|light|system)(?:;|$)/;
export const SYSTEM_COLOR_SCHEME_QUERY = "(prefers-color-scheme: dark)";

export type ThemePreference = "light" | "dark" | "system";
export type ResolvedTheme = "light" | "dark";

export const DEFAULT_THEME_PREFERENCE: ThemePreference = "system";

export const THEME_CYCLE: ThemePreference[] = ["light", "dark", "system"];

export function isThemePreference(value: string | null | undefined): value is ThemePreference {
  return value === "light" || value === "dark" || value === "system";
}

export function nextThemePreference(current: ThemePreference): ThemePreference {
  const index = THEME_CYCLE.indexOf(current);
  return THEME_CYCLE[(index + 1) % THEME_CYCLE.length];
}

export function persistThemePreference(preference: ThemePreference) {
  try {
    window.localStorage.setItem(THEME_STORAGE_KEY, preference);
  } catch {}
  document.cookie = `${THEME_STORAGE_KEY}=${preference}; path=/; max-age=31536000; samesite=lax`;
}

export function readStoredThemePreference(): ThemePreference | null {
  const cookie = document.cookie.match(THEME_COOKIE_PATTERN)?.[1];
  if (isThemePreference(cookie)) return cookie;

  try {
    const stored = window.localStorage.getItem(THEME_STORAGE_KEY);
    if (isThemePreference(stored)) {
      return stored;
    }
  } catch {}
  return null;
}
