import Link from 'next/link';
import type { ReactNode } from 'react';
import { Download } from 'lucide-react';

import { AppShell } from '@/components/ui/app-shell';
import { ParticipantJoinTeamForm } from '@/components/dashboard/participant-join-team-form';
import { ParticipantLogoutButton } from '@/components/dashboard/participant-session-button';
import { Button } from '@/components/ui/button';
import { StatusBanner } from '@/components/ui/status-banner';
import type { PlatformOverview } from '@/lib/dashboard-types';

const navItems = [
  { href: '/', label: 'Overview' },
  { href: '/scoreboard', label: 'Scoreboard' },
  { href: '/services', label: 'Services' },
  { href: '/attacks', label: 'Attacks' },
] as const;

type Props = {
  activePath: string;
  overview: PlatformOverview;
  children: ReactNode;
  title?: string;
  description?: string;
};

export function ParticipantShell({
  activePath,
  overview,
  children,
  title = 'Participant',
  description = 'Live standings, owned service controls, and accepted attacks.',
}: Props) {
  const gameAlertMessage = getParticipantGameAlertMessage(overview);
  const hasParticipantTeam =
    overview.authenticated &&
    overview.role !== "organizer" &&
    (overview.teamID ?? 0) > 0;
  const needsTeamJoin =
    overview.authenticated &&
    overview.role !== "organizer" &&
    (overview.teamID ?? 0) <= 0;

  return (
    <AppShell
      activePath={activePath}
      navItems={navItems}
      title={title}
      description={description}
      headerActions={
        <>
          {overview.role === "organizer" && (
            <Button asChild variant="outline">
              <a href="/admin">Organizer</a>
            </Button>
          )}
          {hasParticipantTeam ? (
            <Button
              asChild
              data-testid="participant-vpn-config"
              variant="outline"
            >
              <a href="/api/platform/me/wireguard" download>
                <Download className="h-4 w-4" />
                VPN Config
              </a>
            </Button>
          ) : null}
          <Button asChild variant="outline">
            <Link href="/docs/participant">Manual</Link>
          </Button>
          {overview.authenticated ? (
            <ParticipantLogoutButton />
          ) : (
            <Button asChild>
              <Link href="/login">Sign In</Link>
            </Button>
          )}
        </>
      }
      summarySection={
        <section className="surface-workroom rounded-sm border">
          <dl className="grid gap-0 sm:grid-cols-2 xl:grid-cols-7">
            <SummaryItem label="Challenge Catalog" value={String(overview.challengeCount)} />
            <SummaryItem label="Owned Services" value={String(overview.ownServiceCount)} />
            <SummaryItem label="Enemy Targets" value={String(overview.enemyTargetCount)} />
            <SummaryItem
              label="Signed In As"
              value={
                overview.authenticated
                  ? overview.displayName || overview.teamName || `Team ${overview.teamID ?? ''}`
                  : 'Not Signed In'
              }
            />
            <SummaryItem
              label="API Status"
              value={
                overview.source === "live" ? (
                  <span className="text-positive">Live</span>
                ) : (
                  <span className="text-negative">Degraded</span>
                )
              }
            />
            <SummaryItem label="API Base" value={overview.apiBaseUrl} mono />
            <SummaryItem label="Realtime" value={overview.realtimeBaseUrl} mono />
          </dl>
        </section>
      }
      alertSection={
        <>
          {overview.message ? <StatusBanner message={overview.message} variant="warning" /> : null}
          {needsTeamJoin ? <ParticipantJoinTeamForm /> : null}
          {gameAlertMessage ? (
            <StatusBanner message={gameAlertMessage} variant="warning" />
          ) : null}
        </>
      }
    >
      {children}
    </AppShell>
  );
}

function getParticipantGameAlertMessage(overview: PlatformOverview): string | null {
  if (overview.matchState === 'finished') {
    const currentTickText =
      typeof overview.currentTick === 'number' ? ` at tick #${overview.currentTick}` : '';
    return `The match has finished${currentTickText}. Participant submissions are closed.`;
  }

  if (overview.matchState === 'stopped') {
    const schedulerText =
      overview.schedulerState === 'running'
        ? ' The scheduler is still reporting as running and should be checked by the organizer.'
        : '';
    return `The match is currently stopped. Participant submissions are closed.${schedulerText}`;
  }

  if (overview.matchState === 'paused' || overview.schedulerState === 'paused') {
    return 'The game scheduler is currently paused. Tick progression and checker runs are temporarily suspended.';
  }

  return null;
}

function SummaryItem({
  label,
  value,
  mono = false,
}: {
  label: string;
  value: ReactNode;
  mono?: boolean;
}) {
  return (
    <div className="border-b px-3 py-2.5 last:border-b-0 sm:[&:nth-last-child(-n+2)]:border-b-0 xl:border-b-0 xl:border-r xl:last:border-r-0">
      <dt className="text-xs font-medium text-muted-foreground">{label}</dt>
      <dd
        className={
          mono
            ? "mt-1 break-all font-mono text-sm"
            : "mt-1 text-sm font-semibold"
        }
      >
        {value}
      </dd>
    </div>
  );
}
