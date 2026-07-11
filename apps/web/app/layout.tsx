import type { Metadata } from 'next';
import { IBM_Plex_Sans } from 'next/font/google';
import { cookies } from 'next/headers';
import './globals.css';

const ibmPlexSans = IBM_Plex_Sans({
  subsets: ['latin'],
  weight: ['400', '500', '600', '700'],
  variable: '--font-plex',
  display: 'swap',
});

export const metadata: Metadata = {
  title: 'AD Platform',
  description: 'Attack-Defense Tick platform control surface',
};

export default async function RootLayout({ children }: Readonly<{ children: React.ReactNode }>) {
  // SSR paints explicit light/dark + scheme so router.refresh() does not flash.
  // System mode and first visit still resolve in the inline FOUC script.
  const cookieStore = await cookies();
  const storedTheme = cookieStore.get('ad-platform-theme')?.value;
  const storedScheme = cookieStore.get('ad-platform-scheme')?.value;

  const preference =
    storedTheme === 'dark' || storedTheme === 'light' || storedTheme === 'system'
      ? storedTheme
      : 'system';
  const scheme =
    storedScheme === 'graphite' ||
    storedScheme === 'ink' ||
    storedScheme === 'paper' ||
    storedScheme === 'moss'
      ? storedScheme
      : 'graphite';
  // Only bake an explicit light/dark into SSR; system resolves in FOUC script.
  const theme = preference === 'dark' || preference === 'light' ? preference : undefined;

  return (
    <html
      lang="en"
      className={ibmPlexSans.variable}
      data-scheme={scheme}
      data-theme={theme}
      data-theme-preference={preference}
      suppressHydrationWarning
    >
      <head>
        <script
          dangerouslySetInnerHTML={{
            __html: `(() => {
  const themeKey = "ad-platform-theme";
  const schemeKey = "ad-platform-scheme";
  const root = document.documentElement;
  const schemes = { graphite: 1, ink: 1, paper: 1, moss: 1 };

  function readCookie(name) {
    const match = document.cookie.match(new RegExp("(?:^|; )" + name + "=(graphite|ink|paper|moss|dark|light|system)"));
    return match ? match[1] : null;
  }

  let themeStored = readCookie(themeKey);
  if (!themeStored) {
    try { themeStored = window.localStorage.getItem(themeKey); } catch {}
  }
  let schemeStored = readCookie(schemeKey);
  if (!schemeStored) {
    try { schemeStored = window.localStorage.getItem(schemeKey); } catch {}
  }

  const hasTheme = themeStored === "dark" || themeStored === "light" || themeStored === "system";
  const hasScheme = schemeStored && schemes[schemeStored];
  const preference = hasTheme ? themeStored : "system";
  const scheme = hasScheme ? schemeStored : "graphite";
  const resolved =
    preference === "system"
      ? (window.matchMedia("(prefers-color-scheme: dark)").matches ? "dark" : "light")
      : preference;

  root.dataset.scheme = scheme;
  root.dataset.theme = resolved;
  root.dataset.themePreference = preference;

  /*
   * First visit: do not persist theme. Absence means System and keeps
   * following OS day/night. Only explicit Light/Dark/System clicks write
   * storage (see AppearanceControls). Scheme still defaults to Graphite
   * without locking mode.
   */
  if (!hasScheme) {
    try { window.localStorage.setItem(schemeKey, scheme); } catch {}
    document.cookie = schemeKey + "=" + scheme + "; path=/; max-age=31536000; samesite=lax";
  }
})();`,
          }}
        />
      </head>
      <body className={ibmPlexSans.className}>{children}</body>
    </html>
  );
}
