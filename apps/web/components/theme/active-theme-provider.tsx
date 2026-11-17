"use client";

import { createContext, useContext, type ReactNode } from "react";

import { DEFAULT_THEME_ID, type ThemeDefinition } from "@/lib/theme-contract";
// Only called at render time: themes import the kit, which imports this
// module, so reading lib/themes during module evaluation would be circular.
import { getTheme } from "@/lib/themes";

const ActiveThemeContext = createContext<string>(DEFAULT_THEME_ID);

/** Carries the organizer-selected theme id from the root layout to client components. */
export function ActiveThemeProvider({ themeId, children }: { themeId: string; children: ReactNode }) {
  return <ActiveThemeContext.Provider value={themeId}>{children}</ActiveThemeContext.Provider>;
}

export function useActiveTheme(): ThemeDefinition {
  return getTheme(useContext(ActiveThemeContext));
}
