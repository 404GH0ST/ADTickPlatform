"use client";

import {
  createContext,
  startTransition,
  useContext,
  useLayoutEffect,
  useMemo,
  useState,
  type ReactNode,
} from "react";

import {
  persistThemePreference,
  readStoredThemePreference,
  SYSTEM_COLOR_SCHEME_QUERY,
  type ResolvedTheme,
  type ThemePreference,
} from "@/lib/appearance";

const ColorModeContext = createContext<{
  preference: ThemePreference;
  selectMode: (mode: ThemePreference) => void;
} | null>(null);

export function useColorMode() {
  const colorMode = useContext(ColorModeContext);
  if (!colorMode) throw new Error("Color mode controls require DocumentAppearance");
  return colorMode;
}

export function DocumentAppearance({
  children,
  className,
  themeId,
  defaultMode,
  initialPreference,
}: {
  children: ReactNode;
  className: string;
  themeId: string;
  defaultMode: ThemePreference;
  initialPreference: ThemePreference | null;
}) {
  // The cookie seeds the first render. A later route response can contain an
  // older cookie snapshot, so the live choice stays in this mounted root.
  const [selectedMode, setSelectedMode] = useState(initialPreference);
  const [systemMode, setSystemMode] = useState<ResolvedTheme>();

  useLayoutEffect(() => {
    const storedMode = readStoredThemePreference();
    // The inline head script has already applied this mode. An urgent update
    // here would make React discard server HTML that hasn't hydrated yet,
    // including anything typed into it.
    if (storedMode) startTransition(() => setSelectedMode(storedMode));

    const media = window.matchMedia(SYSTEM_COLOR_SCHEME_QUERY);
    const updateSystemMode = () => setSystemMode(media.matches ? "dark" : "light");
    updateSystemMode();
    media.addEventListener("change", updateSystemMode);
    return () => media.removeEventListener("change", updateSystemMode);
  }, []);

  const preference = selectedMode ?? defaultMode;
  // Stable across renders: a new context value reaches every Suspense boundary
  // still waiting to hydrate, and React then replaces its server HTML.
  const colorMode = useMemo(
    () => ({
      preference,
      selectMode(mode: ThemePreference) {
        persistThemePreference(mode);
        setSelectedMode(mode);
      },
    }),
    [preference],
  );

  return (
    <ColorModeContext.Provider value={colorMode}>
      <html
        lang="en"
        className={className}
        data-scheme={themeId}
        data-theme={preference === "system" ? systemMode : preference}
        data-theme-preference={preference}
        suppressHydrationWarning
      >
        {children}
      </html>
    </ColorModeContext.Provider>
  );
}
