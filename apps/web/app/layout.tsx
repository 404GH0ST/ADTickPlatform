import type { Metadata } from 'next';
import './globals.css';

export const metadata: Metadata = {
  title: 'AD Platform',
  description: 'Attack-Defense Tick platform control surface',
};

export default function RootLayout({ children }: Readonly<{ children: React.ReactNode }>) {
  return (
    <html lang="en" suppressHydrationWarning>
      <head>
        <script
          dangerouslySetInnerHTML={{
            __html: `(() => {
  const storageKey = "ad-platform-theme";
  const root = document.documentElement;
  let stored = null;
  try {
    stored = window.localStorage.getItem(storageKey);
  } catch {}
  const theme = stored === "dark" || stored === "light"
    ? stored
    : (window.matchMedia("(prefers-color-scheme: dark)").matches ? "dark" : "light");
  root.dataset.theme = theme;
})();`,
          }}
        />
      </head>
      <body>{children}</body>
    </html>
  );
}
