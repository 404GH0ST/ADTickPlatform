export const THEME_STORAGE_KEY = "ad-platform-theme";
export const SCHEME_STORAGE_KEY = "ad-platform-scheme";

export type ThemePreference = "light" | "dark" | "system";
export type ResolvedTheme = "light" | "dark";
export type ColorScheme = "graphite" | "ink" | "paper" | "moss";

export const DEFAULT_SCHEME: ColorScheme = "graphite";
export const DEFAULT_THEME_PREFERENCE: ThemePreference = "system";

export const COLOR_SCHEMES: readonly {
  id: ColorScheme;
  label: string;
  description: string;
  /** Preview chips: light surface, dark surface, accent */
  swatch: { light: string; dark: string; accent: string };
}[] = [
  {
    id: "graphite",
    label: "Graphite",
    description: "Strict neutral minimal",
    swatch: {
      light: "oklch(0.985 0.002 260)",
      dark: "oklch(0.14 0.004 260)",
      accent: "oklch(0.28 0.01 260)",
    },
  },
  {
    id: "ink",
    label: "Ink",
    description: "Cool slate accent",
    swatch: {
      light: "oklch(0.985 0.006 250)",
      dark: "oklch(0.14 0.012 250)",
      accent: "oklch(0.42 0.09 250)",
    },
  },
  {
    id: "paper",
    label: "Paper",
    description: "Warm workroom desk",
    swatch: {
      light: "oklch(0.975 0.012 75)",
      dark: "oklch(0.15 0.015 60)",
      accent: "oklch(0.42 0.08 55)",
    },
  },
  {
    id: "moss",
    label: "Moss",
    description: "Calm sage technical",
    swatch: {
      light: "oklch(0.98 0.01 145)",
      dark: "oklch(0.15 0.014 150)",
      accent: "oklch(0.40 0.07 150)",
    },
  },
] as const;

export const THEME_CYCLE: ThemePreference[] = ["light", "dark", "system"];

export function isThemePreference(value: string | null | undefined): value is ThemePreference {
  return value === "light" || value === "dark" || value === "system";
}

export function isColorScheme(value: string | null | undefined): value is ColorScheme {
  return value === "graphite" || value === "ink" || value === "paper" || value === "moss";
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

export function applyAppearance(scheme: ColorScheme, preference: ThemePreference) {
  if (typeof document === "undefined") {
    return;
  }
  const root = document.documentElement;
  const resolved = resolveTheme(preference);
  root.dataset.scheme = scheme;
  root.dataset.theme = resolved;
  root.dataset.themePreference = preference;
}

export function persistScheme(scheme: ColorScheme) {
  try {
    window.localStorage.setItem(SCHEME_STORAGE_KEY, scheme);
  } catch {}
  document.cookie = `${SCHEME_STORAGE_KEY}=${scheme}; path=/; max-age=31536000; samesite=lax`;
}

export function persistThemePreference(preference: ThemePreference) {
  try {
    window.localStorage.setItem(THEME_STORAGE_KEY, preference);
  } catch {}
  document.cookie = `${THEME_STORAGE_KEY}=${preference}; path=/; max-age=31536000; samesite=lax`;
}

export function readSchemeFromDom(): ColorScheme {
  const fromDom = document.documentElement.dataset.scheme;
  if (isColorScheme(fromDom)) {
    return fromDom;
  }
  try {
    const stored = window.localStorage.getItem(SCHEME_STORAGE_KEY);
    if (isColorScheme(stored)) {
      return stored;
    }
  } catch {}
  const match = document.cookie.match(/(?:^|; )ad-platform-scheme=(graphite|ink|paper|moss)/);
  if (match && isColorScheme(match[1])) {
    return match[1];
  }
  return DEFAULT_SCHEME;
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
