import type { Metadata } from 'next';
import { IBM_Plex_Sans, Lexend } from 'next/font/google';
import { cookies } from 'next/headers';

import { ActiveThemeProvider } from '@/components/theme/active-theme-provider';
import { DocumentAppearance } from '@/components/theme/document-appearance';
import {
  DEFAULT_THEME_PREFERENCE,
  isThemePreference,
  SYSTEM_COLOR_SCHEME_QUERY,
  THEME_COOKIE_PATTERN,
  THEME_STORAGE_KEY,
} from '@/lib/appearance';
import { getActiveThemeId } from '@/lib/platform-appearance';
import { getTheme } from '@/lib/themes';
import { ActiveThemeStyles } from '@/themes/styles.generated';
import './globals.css';

const ibmPlexSans = IBM_Plex_Sans({
  subsets: ['latin'],
  weight: ['400', '500', '600', '700'],
  variable: '--font-plex',
  display: 'swap',
  // Let the active scheme's CSS request Plex when it needs it.
  preload: false,
});

// Face for the Neubrutalism theme. Not preloaded: only pages whose theme CSS
// references --font-lexend ever download it.
const lexend = Lexend({
  subsets: ['latin'],
  variable: '--font-lexend',
  display: 'swap',
  preload: false,
});

export const metadata: Metadata = {
  title: 'AD Platform',
  description: 'Attack-Defense Tick platform control surface',
};

export default async function RootLayout({ children }: Readonly<{ children: React.ReactNode }>) {
  // The organizer sets the theme. The browser owns the visitor's color mode
  // after the initial server render.
  const [cookieStore, themeId] = await Promise.all([cookies(), getActiveThemeId()]);
  const storedTheme = cookieStore.get(THEME_STORAGE_KEY)?.value;
  const themeDefaultMode = getTheme(themeId).defaultMode ?? DEFAULT_THEME_PREFERENCE;

  return (
    <DocumentAppearance
      className={`${ibmPlexSans.variable} ${lexend.variable}`}
      themeId={themeId}
      defaultMode={themeDefaultMode}
      initialPreference={isThemePreference(storedTheme) ? storedTheme : null}
    >
      <head>
        <script
          dangerouslySetInnerHTML={{
            __html: `(() => {
  const themeKey = ${JSON.stringify(THEME_STORAGE_KEY)};
  const themeDefaultMode = ${JSON.stringify(themeDefaultMode)};
  const root = document.documentElement;

  let themeStored = document.cookie.match(${THEME_COOKIE_PATTERN})?.[1];
  if (!themeStored) {
    try { themeStored = window.localStorage.getItem(themeKey); } catch {}
  }

  const hasTheme = themeStored === "dark" || themeStored === "light" || themeStored === "system";
  const preference = hasTheme ? themeStored : themeDefaultMode;
  const resolved =
    preference === "system"
      ? (window.matchMedia(${JSON.stringify(SYSTEM_COLOR_SCHEME_QUERY)}).matches ? "dark" : "light")
      : preference;

  root.dataset.theme = resolved;
  root.dataset.themePreference = preference;
  // Shells with a collapsible sidebar read this before first paint, so
  // navigating never flashes the expanded rail.
  try {
    if (window.localStorage.getItem("ad-platform-rail") === "collapsed") root.dataset.rail = "collapsed";
  } catch {}
  // First visit writes nothing: absence means the theme's default mode (System
  // unless the theme says otherwise). Only explicit clicks persist.
})();`,
          }}
        />
      </head>
      <body className={ibmPlexSans.className}>
        <ActiveThemeStyles themeId={themeId} />
        <ActiveThemeProvider themeId={themeId}>{children}</ActiveThemeProvider>
      </body>
    </DocumentAppearance>
  );
}
