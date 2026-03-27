'use client';

import type { ReactElement } from 'react';

import { Badge } from '@/components/ui/badge';
import { EmptyStateText } from '@/components/ui/empty-state';

type AttackSliceRow = {
  attacker: string;
  victim: string;
  service: string;
};

export function AttackSliceSummaryGrid<T extends AttackSliceRow>({
  rows,
}: {
  rows: T[];
}): ReactElement {
  const leaders = buildAttackSliceLeaders(rows);

  return (
    <div className="grid gap-3 xl:grid-cols-3">
      <AttackLeaderboardPanel
        emptyMessage="No accepted attackers in the current slice."
        items={leaders.attackers}
        title="Top attackers"
      />
      <AttackLeaderboardPanel
        emptyMessage="No targeted victims in the current slice."
        items={leaders.victims}
        title="Top victims"
      />
      <AttackLeaderboardPanel
        emptyMessage="No services in the current slice."
        items={leaders.services}
        title="Hottest services"
      />
    </div>
  );
}

function AttackLeaderboardPanel({
  emptyMessage,
  items,
  title,
}: {
  emptyMessage: string;
  items: Array<{ count: number; label: string }>;
  title: string;
}): ReactElement {
  return (
    <div className="rounded-md border border-border/70 bg-muted/20 p-3">
      <div className="mb-3 flex items-center justify-between gap-3">
        <p className="text-sm font-medium text-foreground">{title}</p>
        <Badge variant="outline">{items.length}</Badge>
      </div>
      {items.length === 0 ? (
        <EmptyStateText message={emptyMessage} />
      ) : (
        <ol className="space-y-2 text-sm">
          {items.map((item, index) => (
            <li key={`${title}:${item.label}`} className="flex items-center justify-between gap-3">
              <span className="truncate text-muted-foreground">
                {index + 1}. {item.label}
              </span>
              <span className="font-mono text-foreground">{item.count}</span>
            </li>
          ))}
        </ol>
      )}
    </div>
  );
}

function buildAttackSliceLeaders<T extends AttackSliceRow>(
  attackRows: T[],
): {
  attackers: Array<{ count: number; label: string }>;
  services: Array<{ count: number; label: string }>;
  victims: Array<{ count: number; label: string }>;
} {
  return {
    attackers: summarizeAttackField(attackRows, 'attacker'),
    victims: summarizeAttackField(attackRows, 'victim'),
    services: summarizeAttackField(attackRows, 'service'),
  };
}

function summarizeAttackField<T extends AttackSliceRow>(
  attackRows: T[],
  field: 'attacker' | 'victim' | 'service',
): Array<{ count: number; label: string }> {
  const counts = new Map<string, number>();

  for (const row of attackRows) {
    const label = row[field];
    counts.set(label, (counts.get(label) ?? 0) + 1);
  }

  return Array.from(counts.entries())
    .map(([label, count]) => ({ label, count }))
    .sort((left, right) => {
      if (right.count !== left.count) {
        return right.count - left.count;
      }
      return left.label.localeCompare(right.label);
    })
    .slice(0, 3);
}
