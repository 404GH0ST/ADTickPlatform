"use client";

import { useLinkStatus } from "next/link";
import { useEffect } from "react";

let pendingLinks = 0;

function syncPendingNavigation() {
  const busy = pendingLinks > 0;
  const root = document.documentElement;
  if (busy) root.dataset.navPending = "true";
  else delete root.dataset.navPending;
  const main = document.querySelector("main");
  if (busy) main?.setAttribute("aria-busy", "true");
  else main?.removeAttribute("aria-busy");
}

/**
 * Put inside a <Link>. While its page loads, the link gets a pending style and
 * the top of the window shows a progress bar (see globals.css).
 */
export function LinkPendingSignal() {
  const { pending } = useLinkStatus();
  useEffect(() => {
    if (!pending) return;
    pendingLinks += 1;
    syncPendingNavigation();
    return () => {
      pendingLinks -= 1;
      syncPendingNavigation();
    };
  }, [pending]);
  return <span hidden data-link-pending={pending ? "true" : undefined} />;
}
