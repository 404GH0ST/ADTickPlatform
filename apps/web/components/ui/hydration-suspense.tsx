"use client";

import { Suspense, useCallback, useEffect, useMemo, useState, type ReactNode } from "react";

/**
 * Suspense for a lazy view that is server-rendered. A parent update reaching
 * the boundary before it hydrates makes React discard the server HTML and show
 * the fallback, so the view blinks out and the page jumps. This holds the
 * server-rendered children until they have hydrated, then follows live props.
 */
export function HydrationSuspense({ fallback, children }: { fallback: ReactNode; children: ReactNode }) {
  const [serverTree] = useState({ fallback, children });
  const [hydrated, setHydrated] = useState(false);
  const markHydrated = useCallback(() => setHydrated(true), []);
  const tree = hydrated ? { fallback, children } : serverTree;
  return useMemo(
    () => (
      <Suspense fallback={tree.fallback}>
        {tree.children}
        <HydrationMark onHydrated={markHydrated} />
      </Suspense>
    ),
    [tree.fallback, tree.children, markHydrated],
  );
}

/** Sits inside the boundary, so its effect runs once the content is live. */
function HydrationMark({ onHydrated }: { onHydrated: () => void }) {
  useEffect(onHydrated, [onHydrated]);
  return null;
}
