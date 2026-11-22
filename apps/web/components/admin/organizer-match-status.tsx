'use client';

import { Snowflake } from 'lucide-react';

import { useLiveMatchStatus } from '@/components/admin/organizer-live-status';

import { formatMatchStateLabel } from '@/lib/match-state';
import type { AdminOverview } from '@/lib/admin-dashboard-types';

const STATE_TONE: Record<string, string> = {
  running: 'tone-success',
  paused: 'tone-warning',
  stopped: 'tone-danger',
};

/**
 * Glanceable match state for every organizer page: lifecycle, current tick,
 * and whether participants are looking at a frozen board.
 */
export function OrganizerMatchStatus({ overview }: { overview: AdminOverview }) {
  // The dashboard's live state is newer than this server-rendered overview.
  const live = useLiveMatchStatus();
  const matchState = live ? live.matchState : overview.matchState;
  const currentTick = live ? live.currentTick : overview.currentTick;
  const state = matchState?.toLowerCase();
  return (
    <div
      role="group"
      aria-label="Match status"
      data-slot="match-status"
      data-testid="organizer-match-status"
      className="flex flex-wrap items-center gap-1.5"
    >
      <span
        data-slot="badge"
        data-state={state ?? 'unknown'}
        className={`inline-flex items-center rounded-sm border px-2 py-0.5 text-xs font-semibold ${STATE_TONE[state ?? ''] ?? 'tone-neutral'}`}
      >
        {!live && !matchState && overview.unavailable?.gameStatus
          ? 'Status unavailable'
          : formatMatchStateLabel(matchState)}
      </span>
      {typeof currentTick === 'number' ? (
        <span
          data-slot="badge"
          className="tone-neutral inline-flex items-center rounded-sm border px-2 py-0.5 text-xs font-semibold tabular-nums"
        >
          Tick #{currentTick}
        </span>
      ) : null}
      {overview.scoreboardFrozen ? (
        <span
          data-slot="badge"
          data-state="frozen"
          data-testid="organizer-frozen-chip"
          className="tone-info inline-flex items-center gap-1 rounded-sm border px-2 py-0.5 text-xs font-semibold"
        >
          <Snowflake className="h-3 w-3" aria-hidden />
          Board frozen
        </span>
      ) : null}
    </div>
  );
}
