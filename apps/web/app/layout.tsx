import type { Metadata } from 'next';
import { IBM_Plex_Sans, Lexend } from 'next/font/google';
import { cookies } from 'next/headers';

import { ActiveThemeProvider } from '@/components/theme/active-theme-provider';
import { getActiveThemeId } from '@/lib/platform-appearance';
import { getTheme } from '@/lib/themes';
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
  // The theme (data-scheme) is the organizer's choice, read from the gateway.
  // SSR also paints an explicit light/dark so router.refresh() does not flash;
  // System mode and first visit still resolve in the inline FOUC script.
  const [cookieStore, themeId] = await Promise.all([cookies(), getActiveThemeId()]);
  const storedTheme = cookieStore.get('ad-platform-theme')?.value;
  // A theme may pick the mode used until the participant chooses one.
  const themeDefaultMode = getTheme(themeId).defaultMode ?? 'system';

  const preference =
    storedTheme === 'dark' || storedTheme === 'light' || storedTheme === 'system'
      ? storedTheme
      : themeDefaultMode;
  // Only bake an explicit light/dark into SSR; system resolves in FOUC script.
  const theme = preference === 'dark' || preference === 'light' ? preference : undefined;

  return (
    <html
      lang="en"
      className={`${ibmPlexSans.variable} ${lexend.variable}`}
      data-scheme={themeId}
      data-theme={theme}
      data-theme-preference={preference}
      suppressHydrationWarning
    >
      <head>
        <script
          dangerouslySetInnerHTML={{
            __html: `(() => {
  const themeKey = "ad-platform-theme";
  const themeDefaultMode = ${JSON.stringify(themeDefaultMode)};
  const root = document.documentElement;

  function readCookie(name) {
    const match = document.cookie.match(new RegExp("(?:^|; )" + name + "=(dark|light|system)"));
    return match ? match[1] : null;
  }

  let themeStored = readCookie(themeKey);
  if (!themeStored) {
    try { themeStored = window.localStorage.getItem(themeKey); } catch {}
  }

  const hasTheme = themeStored === "dark" || themeStored === "light" || themeStored === "system";
  const preference = hasTheme ? themeStored : themeDefaultMode;
  const resolved =
    preference === "system"
      ? (window.matchMedia("(prefers-color-scheme: dark)").matches ? "dark" : "light")
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
        <ActiveThemeProvider themeId={themeId}>{children}</ActiveThemeProvider>
      </body>
    </html>
  );
}
