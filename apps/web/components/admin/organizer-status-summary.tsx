'use client';

import { useEffect, useState } from 'react';
import type { ReactNode } from 'react';

import { StatusBanner } from '@/components/ui/status-banner';
import type {
  AdminOperationsStatus,
  AdminOverview,
} from '@/lib/admin-dashboard-types';
import { processApiResponse } from '@/lib/api-utils';
import { formatTimestampsInText } from '@/lib/date-format';

type Props = {
  overview: AdminOverview;
  /** Show the count strip. Alerts always render; the counts only belong on the overview. */
  showStats?: boolean;
};

function SummaryItem({
  label,
  value,
  mono = false,
  testId,
}: {
  label: string;
  value: ReactNode;
  mono?: boolean;
  testId?: string;
}) {
  return (
    <div
      className="min-w-0 wrap-anywhere border-b border-r border-border px-3 py-2.5 xl:border-b-0 xl:last:border-r-0"
      data-testid={testId}
    >
      <dt className="text-xs font-medium text-muted-foreground">{label}</dt>
      <dd className={mono ? 'mt-1 truncate font-mono text-sm' : 'mt-1 text-sm font-semibold'}>
        {value}
      </dd>
    </div>
  );
}

export function OrganizerStatusSummary({ overview, showStats = true }: Props) {
  const [operationsStatus, setOperationsStatus] = useState<
    AdminOperationsStatus | undefined
  >(overview.operationsStatus);

  useEffect(() => {
    let cancelled = false;

    async function refreshOperationsStatus() {
      try {
        const response = await fetch('/api/admin/operations/status', {
          cache: 'no-store',
        });
        if (!response.ok) {
          return;
        }
        const payload = await processApiResponse<AdminOperationsStatus>(
          response,
          '/api/admin/operations/status',
        );
        if (!cancelled) {
          setOperationsStatus(payload);
        }
      } catch {
        // Keep the last known operations snapshot if one refresh fails.
      }
    }

    const interval = window.setInterval(refreshOperationsStatus, 30000);
    const handleFocus = () => {
      void refreshOperationsStatus();
    };

    window.addEventListener('focus', handleFocus);

    return () => {
      cancelled = true;
      window.clearInterval(interval);
      window.removeEventListener('focus', handleFocus);
    };
  }, []);

  const operationsAlertCount =
    operationsStatus?.alerts.length ?? overview.operationsAlertCount;

  return (
    <>
      {showStats ? (
        <section className="surface-inset overflow-hidden rounded-sm border" data-slot="summary-strip" data-testid="organizer-summary">
          <dl className="grid grid-cols-2 gap-0 sm:grid-cols-3 xl:grid-cols-6">
            <SummaryItem label="Teams" value={String(overview.teamCount)} />
            <SummaryItem label="Players" value={String(overview.playerCount)} />
            <SummaryItem
              label="Challenges"
              value={
                <>
                  {overview.publishedChallengeCount}
                  <span className="font-normal text-muted-foreground"> / {overview.challengeCount} published</span>
                </>
              }
            />
            <SummaryItem
              label="Deployments"
              value={
                <>
                  {overview.deploymentCount}
                  {overview.pendingDeploymentCount > 0 ? (
                    <span className="font-normal text-highlight"> · {overview.pendingDeploymentCount} pending</span>
                  ) : null}
                </>
              }
            />
            <SummaryItem
              label="Ops Alerts"
              value={
                operationsAlertCount > 0 ? (
                  <span className="text-negative">{operationsAlertCount}</span>
                ) : (
                  String(operationsAlertCount)
                )
              }
            />
            <SummaryItem label="Persisted Ticks" value={String(overview.totalTicks)} />
          </dl>
          <p className="border-t border-border px-3 py-1.5 text-xs text-muted-foreground">
            API base <span className="wrap-anywhere font-mono text-foreground">{overview.apiBaseUrl}</span>
          </p>
        </section>
      ) : null}

      {overview.message ? (
        <StatusBanner message={overview.message} variant="warning" announce="change" />
      ) : null}
      {operationsStatus?.alerts.map((alert) => (
        <StatusBanner
          key={alert.id}
          message={alert.detail ? `${alert.summary} ${formatTimestampsInText(alert.detail)}` : alert.summary}
          variant={alert.severity === 'critical' ? 'error' : 'warning'}
          announce="change"
        />
      )) ?? null}
    </>
  );
}
