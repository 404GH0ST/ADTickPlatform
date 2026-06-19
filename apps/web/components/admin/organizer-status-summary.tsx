'use client';

import { useEffect, useState } from 'react';
import type { ReactNode } from 'react';

import { StatusBanner } from '@/components/ui/status-banner';
import type {
  AdminOperationsStatus,
  AdminOverview,
} from '@/lib/admin-dashboard-types';
import { processApiResponse } from '@/lib/api-utils';

type Props = {
  overview: AdminOverview;
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
      className="border-b px-3 py-2.5 last:border-b-0 sm:[&:nth-last-child(-n+2)]:border-b-0 xl:border-b-0 xl:border-r xl:last:border-r-0"
      data-testid={testId}
    >
      <dt className="text-xs font-medium text-muted-foreground">{label}</dt>
      <dd className={mono ? 'mt-1 break-all font-mono text-sm' : 'mt-1 text-sm font-semibold'}>
        {value}
      </dd>
    </div>
  );
}

export function OrganizerStatusSummary({ overview }: Props) {
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
      <section className="surface-workroom rounded-sm border" data-testid="organizer-summary">
        <dl className="grid gap-0 sm:grid-cols-2 xl:grid-cols-9">
          <SummaryItem label="Teams" value={String(overview.teamCount)} />
          <SummaryItem label="Players" value={String(overview.playerCount)} />
          <SummaryItem
            label="Challenges"
            value={String(overview.challengeCount)}
          />
          <SummaryItem
            label="Published Challenges"
            value={String(overview.publishedChallengeCount)}
          />
          <SummaryItem
            label="Deployments"
            value={String(overview.deploymentCount)}
          />
          <SummaryItem
            label="Pending Deployments"
            value={
              overview.pendingDeploymentCount > 0 ? (
                <span className="text-highlight">{overview.pendingDeploymentCount}</span>
              ) : (
                String(overview.pendingDeploymentCount)
              )
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
          <SummaryItem label="API Base" value={overview.apiBaseUrl} mono />
        </dl>
      </section>

      {overview.message ? (
        <StatusBanner message={overview.message} variant="warning" />
      ) : null}
      {operationsStatus?.alerts.map((alert) => (
        <StatusBanner
          key={alert.id}
          message={alert.detail ? `${alert.summary} ${alert.detail}` : alert.summary}
          variant={alert.severity === 'critical' ? 'error' : 'warning'}
        />
      )) ?? null}
    </>
  );
}
