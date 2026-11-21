'use client';

import { useEffect, useState, useSyncExternalStore, type ReactElement } from 'react';

type StatusBannerVariant = 'error' | 'success' | 'warning' | 'info';

/**
 * How the banner reaches screen readers:
 * - mount: announce when it appears (results of an action). The default.
 * - change: only when its text changes (page state that returns on every navigation).
 * - inherit: same as mount, but a parent aria-live region does the announcing.
 */
type StatusBannerAnnounce = 'mount' | 'change' | 'inherit';

type StatusBannerProps = {
  message: string;
  variant: StatusBannerVariant;
  announce?: StatusBannerAnnounce;
};

const bannerTone: Record<StatusBannerVariant, string> = {
  error: 'tone-danger',
  success: 'tone-success',
  warning: 'tone-warning',
  info: 'tone-info',
};

// Long enough for the browser to expose the empty live region first.
const REVEAL_DELAY_MS = 60;

const subscribeToNothing = () => () => {};

export function StatusBanner({
  message,
  variant,
  announce = 'mount',
}: StatusBannerProps): ReactElement {
  // Server-rendered banners are part of the page and must show without JS.
  const hydrating = useSyncExternalStore(subscribeToNothing, () => false, () => true);
  // A live region that appears with its text is often skipped. The text keeps
  // its space but enters the accessibility tree just after the region does.
  const [revealed, setRevealed] = useState(hydrating || announce === 'change');

  useEffect(() => {
    if (revealed) return;
    const timer = window.setTimeout(() => setRevealed(true), REVEAL_DELAY_MS);
    return () => window.clearTimeout(timer);
  }, [revealed]);

  return (
    <div
      role={announce === 'inherit' ? undefined : 'status'}
      className={`rounded-sm border px-3.5 py-2.5 text-sm leading-5 ${bannerTone[variant]}`}
    >
      <span className={revealed ? undefined : 'invisible'}>{message}</span>
    </div>
  );
}
