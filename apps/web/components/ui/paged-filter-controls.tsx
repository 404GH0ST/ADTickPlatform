'use client';

import type { ReactElement } from 'react';

import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';

type PageDirection = 'prev' | 'next';

type LiveModeBadgeProps = {
  filteredLabel?: string;
  liveLabel?: string;
  liveMode: boolean;
};

type PagedFilterActionsProps = {
  applyLabel: string;
  canPageNext: boolean;
  canPagePrev: boolean;
  disabled: boolean;
  filteredLabel?: string;
  liveLabel?: string;
  liveMode: boolean;
  onApply: () => void;
  onPage: (direction: PageDirection) => void;
  onReset: () => void;
  resetLabel: string;
  showLiveModeBadge?: boolean;
};

type SliceCountBadgeProps = {
  suffix?: string;
  totalCount: number;
  visibleCount: number;
  offset?: number;
};

export function LiveModeBadge({
  filteredLabel = 'filtered view',
  liveLabel = 'live head',
  liveMode,
}: LiveModeBadgeProps): ReactElement {
  return (
    <Badge variant={liveMode ? 'default' : 'secondary'}>
      {liveMode ? liveLabel : filteredLabel}
    </Badge>
  );
}

export function PagedFilterActions({
  applyLabel,
  canPageNext,
  canPagePrev,
  disabled,
  filteredLabel,
  liveLabel,
  liveMode,
  onApply,
  onPage,
  onReset,
  resetLabel,
  showLiveModeBadge = true,
}: PagedFilterActionsProps): ReactElement {
  return (
    <div className="flex flex-wrap gap-2">
      <Button disabled={disabled} variant="outline" size="sm" onClick={onApply}>
        {applyLabel}
      </Button>
      <Button disabled={disabled} variant="outline" size="sm" onClick={onReset}>
        {resetLabel}
      </Button>
      <Button
        disabled={disabled || !canPagePrev}
        variant="outline"
        size="sm"
        onClick={() => onPage('prev')}
      >
        Previous
      </Button>
      <Button
        disabled={disabled || !canPageNext}
        variant="outline"
        size="sm"
        onClick={() => onPage('next')}
      >
        Next
      </Button>
      {showLiveModeBadge ? (
        <LiveModeBadge
          filteredLabel={filteredLabel}
          liveLabel={liveLabel}
          liveMode={liveMode}
        />
      ) : null}
    </div>
  );
}

export function SliceCountBadge({
  suffix,
  totalCount,
  visibleCount,
  offset = 0,
}: SliceCountBadgeProps): ReactElement {
  const start = visibleCount > 0 ? offset + 1 : 0;
  const end = offset + visibleCount;
  const range = `${start}-${end}`;

  const label = suffix
    ? `Showing ${range} of ${totalCount} ${suffix}`
    : `Showing ${range} of ${totalCount}`;

  return <Badge variant="secondary">{label}</Badge>;
}
