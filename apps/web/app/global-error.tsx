"use client";

import { useEffect, useLayoutEffect } from "react";

import { readStoredThemePreference, SYSTEM_COLOR_SCHEME_QUERY } from "@/lib/appearance";

// This page replaces the root layout, so no theme CSS is loaded. System colors
// stay readable in either mode without depending on anything that may have failed.
const styles = `
:root { color-scheme: light dark; }
body { margin: 0; min-height: 100dvh; display: grid; place-items: center; padding: 1rem; box-sizing: border-box; background: Canvas; color: CanvasText; font-family: system-ui, sans-serif; }
main { width: 100%; max-width: 28rem; box-sizing: border-box; border: 2px solid CanvasText; padding: 1.5rem; }
h1 { margin: 0 0 .5rem; font-size: 1.5rem; line-height: 1.2; text-wrap: balance; }
p { margin: 0 0 1.25rem; line-height: 1.5; }
code { overflow-wrap: anywhere; }
.actions { display: flex; flex-wrap: wrap; gap: .5rem; }
button, a { display: inline-flex; align-items: center; min-height: 2.75rem; box-sizing: border-box; padding: 0 1rem; border: 2px solid CanvasText; font: inherit; font-weight: 600; text-decoration: none; cursor: pointer; }
button { background: CanvasText; color: Canvas; }
a { background: Canvas; color: CanvasText; }
:focus-visible { outline: 3px solid Highlight; outline-offset: 2px; }
`;

export default function GlobalError({ error, retry }: { error: unknown; retry: () => void }) {
  useEffect(() => {
    console.error(error);
  }, [error]);

  // Honor the visitor's explicit light or dark choice.
  useLayoutEffect(() => {
    const stored = readStoredThemePreference();
    const dark =
      stored === "dark" || (stored !== "light" && window.matchMedia(SYSTEM_COLOR_SCHEME_QUERY).matches);
    document.documentElement.style.colorScheme = dark ? "dark" : "light";
  }, []);

  const digest =
    error instanceof Error && "digest" in error && typeof error.digest === "string"
      ? error.digest
      : "";

  return (
    <html lang="en">
      <head>
        <title>Something Went Wrong | AD Platform</title>
        <style dangerouslySetInnerHTML={{ __html: styles }} />
      </head>
      <body>
        <main>
          <h1>The platform couldn&apos;t load</h1>
          <p>
            Something went wrong on our side. Try again, and if it keeps happening, tell the organizers
            {digest ? (
              <>
                {" "}and quote reference <code>{digest}</code>
              </>
            ) : null}
            .
          </p>
          <div className="actions">
            <button type="button" onClick={() => retry()}>
              Try again
            </button>
            <a href="/">Go to Overview</a>
          </div>
        </main>
      </body>
    </html>
  );
}
