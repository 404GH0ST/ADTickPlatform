import { Snowflake } from 'lucide-react';

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
  const state = overview.matchState?.toLowerCase();
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
        {formatMatchStateLabel(overview.matchState)}
      </span>
      {typeof overview.currentTick === 'number' ? (
        <span
          data-slot="badge"
          className="tone-neutral inline-flex items-center rounded-sm border px-2 py-0.5 text-xs font-semibold tabular-nums"
        >
          Tick #{overview.currentTick}
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
