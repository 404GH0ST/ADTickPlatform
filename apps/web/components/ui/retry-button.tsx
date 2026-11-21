"use client";

import { useRouter } from "next/navigation";
import { useEffect, useRef, useTransition, type ComponentProps } from "react";

import { Button } from "@/components/ui/button";
import { focusNeighborWhenRemoved } from "@/lib/focus-after-removal";

type RetryButtonProps = Pick<ComponentProps<typeof Button>, "variant" | "size">;

/** A full reload, so the session and every server check run again. */
export function RetryButton({ variant, size }: RetryButtonProps) {
  return (
    <Button type="button" variant={variant} size={size} onClick={() => window.location.reload()}>
      Try again
    </Button>
  );
}

/** Re-runs the server checks in place, so unsent text and last good data stay on screen. */
export function RefreshButton({ variant, size }: RetryButtonProps) {
  const router = useRouter();
  const [pending, startRefresh] = useTransition();
  // Recovery removes this button. Focus then moves to the page heading rather
  // than dropping to the body. If the platform is still down, the button stays.
  const stopWatchingFocus = useRef<(() => void) | null>(null);
  useEffect(() => {
    if (pending) return;
    stopWatchingFocus.current?.();
    stopWatchingFocus.current = null;
  }, [pending]);
  return (
    <Button
      type="button"
      variant={variant}
      size={size}
      disabled={pending}
      holdFocus={pending}
      onClick={(event) => {
        stopWatchingFocus.current?.();
        stopWatchingFocus.current = focusNeighborWhenRemoved(event.currentTarget);
        startRefresh(() => router.refresh());
      }}
    >
      {pending ? "Trying…" : "Try again"}
    </Button>
  );
}
