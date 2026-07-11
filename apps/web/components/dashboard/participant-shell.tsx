"use client";

import Link from 'next/link';
import type { ReactNode } from 'react';

import { AppShell } from '@/components/ui/app-shell';
import { AnnouncementBanner } from '@/components/dashboard/announcement-banner';
import {
  MatchReadinessNavButton,
  MatchReadinessPanel,
  useMatchReadiness,
} from '@/components/dashboard/match-readiness';
import { ParticipantAccountSettings } from '@/components/dashboard/participant-account-settings';
import { ParticipantJoinTeamForm } from '@/components/dashboard/participant-join-team-form';
import { ParticipantLogoutButton } from '@/components/dashboard/participant-session-button';
import { VpnConfigDownloadButton } from '@/components/dashboard/vpn-config-download-button';
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
};

export function ParticipantShell({
  activePath,
  overview,
  children,
  title = 'Participant',
}: Props) {
  const gameAlertMessage = getParticipantGameAlertMessage(overview);
  const readiness = useMatchReadiness(overview);
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
      navActions={
        readiness.visible ? (
          <MatchReadinessNavButton
            remaining={readiness.remaining}
            open={readiness.open}
            onToggle={readiness.toggle}
          />
        ) : null
      }
      belowNav={
        readiness.open ? (
          <MatchReadinessPanel
            items={readiness.items}
            remaining={readiness.remaining}
          />
        ) : null
      }
      headerActions={
        <>
          {overview.role === "organizer" && (
            <Button asChild variant="outline">
              <a href="/admin">Organizer</a>
            </Button>
          )}
          {hasParticipantTeam ? <VpnConfigDownloadButton /> : null}
          <Button asChild variant="outline">
            <Link href="/docs/participant">Manual</Link>
          </Button>
          {overview.authenticated ? (
            <>
              {overview.role !== "organizer" ? (
                <ParticipantAccountSettings overview={overview} />
              ) : null}
              <ParticipantLogoutButton />
            </>
          ) : (
            <Button asChild>
              <Link href="/login">Sign In</Link>
            </Button>
          )}
        </>
      }
      summarySection={
        <section className="surface-inset overflow-hidden rounded-sm border" data-testid="participant-summary">
          <dl className="grid gap-0 sm:grid-cols-2 xl:grid-cols-6">
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
          </dl>
        </section>
      }
      alertSection={
        <>
          <AnnouncementBanner />
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
    return 'The match is currently stopped. Participant submissions are closed.';
  }

  if (overview.matchState === 'paused') {
    return 'The match is currently paused. Tick progression and checker runs are temporarily suspended. Submissions are blocked until organizers resume. VPN and SSH to unlocked services remain available.';
  }

  if (overview.scoreboardFrozen) {
    return 'The public scoreboard is frozen. Attack submissions and service controls still follow match state, but standings will not move until organizers unfreeze.';
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
    <div className="border-b border-border px-3 py-2.5 last:border-b-0 sm:[&:nth-last-child(-n+2)]:border-b-0 xl:border-b-0 xl:border-r xl:last:border-r-0">
      <dt className="text-[11px] font-medium uppercase tracking-wide text-muted-foreground">
        {label}
      </dt>
      <dd
        className={
          mono
            ? "mt-1 break-all font-mono text-xs leading-5 text-foreground"
            : "mt-1 text-sm font-semibold tabular-nums tracking-tight text-foreground"
        }
      >
        {value}
      </dd>
    </div>
  );
}
