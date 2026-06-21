import type { Metadata } from 'next';
import { cookies } from 'next/headers';
import './globals.css';

export const metadata: Metadata = {
  title: 'AD Platform',
  description: 'Attack-Defense Tick platform control surface',
};

export default async function RootLayout({ children }: Readonly<{ children: React.ReactNode }>) {
  // Render data-theme from the cookie so React owns the attribute. Without this
  // a router.refresh() re-renders <html> without data-theme and the theme snaps
  // back to the CSS default. The inline script below still covers the first
  // visit (no cookie yet) to avoid a flash.
  const storedTheme = (await cookies()).get('ad-platform-theme')?.value;
  const theme = storedTheme === 'dark' || storedTheme === 'light' ? storedTheme : undefined;

  return (
    <html lang="en" data-theme={theme} suppressHydrationWarning>
      <head>
        <script
          dangerouslySetInnerHTML={{
            __html: `(() => {
  const storageKey = "ad-platform-theme";
  const root = document.documentElement;
  const match = document.cookie.match(/(?:^|; )ad-platform-theme=(dark|light)/);
  let stored = match ? match[1] : null;
  if (!stored) {
    try {
      stored = window.localStorage.getItem(storageKey);
    } catch {}
  }
  const hasStored = stored === "dark" || stored === "light";
  const theme = hasStored
    ? stored
    : (window.matchMedia("(prefers-color-scheme: dark)").matches ? "dark" : "light");
  root.dataset.theme = theme;
  // Persist on first visit so the server can render data-theme on the next
  // request and later refreshes don't flip the theme on their own.
  if (!hasStored) {
    try {
      window.localStorage.setItem(storageKey, theme);
    } catch {}
    document.cookie = "ad-platform-theme=" + theme + "; path=/; max-age=31536000; samesite=lax";
  }
})();`,
          }}
        />
      </head>
      <body>{children}</body>
    </html>
  );
}
