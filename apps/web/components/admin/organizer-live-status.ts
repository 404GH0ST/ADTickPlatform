'use client';

import { useSyncExternalStore } from 'react';

export type LiveMatchStatus = {
  matchState?: string;
  currentTick?: number;
};

let current: LiveMatchStatus | null = null;
const listeners = new Set<() => void>();

/**
 * Lets the server-rendered organizer header follow the dashboard's live match
 * state. Publish null when the dashboard doesn't know the state.
 */
export function publishLiveMatchStatus(next: LiveMatchStatus | null): void {
  if (next?.matchState === current?.matchState && next?.currentTick === current?.currentTick && !next === !current) {
    return;
  }
  current = next;
  listeners.forEach((listener) => listener());
}

function subscribe(listener: () => void): () => void {
  listeners.add(listener);
  return () => listeners.delete(listener);
}

export function useLiveMatchStatus(): LiveMatchStatus | null {
  return useSyncExternalStore(
    subscribe,
    () => current,
    () => null,
  );
}
