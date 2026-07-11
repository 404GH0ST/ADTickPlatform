import type { AdminOverview } from '@/lib/admin-dashboard-types';
import type { ReactNode } from 'react';

import { OrganizerStatusSummary } from '@/components/admin/organizer-status-summary';
import { AppShell } from '@/components/ui/app-shell';
import { Button } from '@/components/ui/button';

const navItems = [
  { href: '/admin', label: 'Overview' },
  { href: '/admin/teams', label: 'Teams' },
  { href: '/admin/players', label: 'Players' },
  { href: '/admin/challenges', label: 'Challenges' },
  { href: '/admin/deployments', label: 'Deployments' },
  { href: '/admin/game', label: 'Game' },
  { href: '/admin/scoreboard', label: 'Scoreboard' },
  { href: '/admin/attacks', label: 'Attacks' },
  { href: '/admin/audit', label: 'Audit' },
] as const;

type Props = {
  activePath: string;
  overview: AdminOverview;
  children: ReactNode;
  title?: string;
};

export function OrganizerShell({
  activePath,
  overview,
  children,
  title = 'Organizer',
}: Props) {
  return (
    <AppShell
      activePath={activePath}
      navItems={navItems}
      title={title}
      headerActions={
        <Button asChild variant="outline">
          <a href="/">Participant</a>
        </Button>
      }
      summarySection={<OrganizerStatusSummary overview={overview} />}
    >
      {children}
    </AppShell>
  );
}
