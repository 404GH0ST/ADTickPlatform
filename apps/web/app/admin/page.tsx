import type { Metadata } from 'next';
import { AnnouncementsPanel } from '@/components/admin/announcements-panel';
import { OrganizerOverview } from '@/components/admin/organizer-overview';
import { OrganizerShell } from '@/components/admin/organizer-shell';
import { TrustedStatePanel } from '@/components/admin/trusted-state-panel';
import { loadAdminDashboardData } from '@/lib/admin-dashboard-data';

export const metadata: Metadata = { title: { absolute: 'Overview | Organizer | AD Platform' } };

export const dynamic = 'force-dynamic';

function plural(count: number, noun: string) {
  return `${count} ${noun}${count === 1 ? '' : 's'}`;
}

export default async function AdminPage() {
  const dashboard = await loadAdminDashboardData();
  const { unavailable } = dashboard;
  const meta = (failed: boolean | undefined, count: number, noun: string) =>
    failed ? 'unavailable' : plural(count, noun);

  return (
    <OrganizerShell
      activePath="/admin"
      overview={dashboard.overview}
      title="Organizer Overview"
    >
      <OrganizerOverview
        trustedState={<TrustedStatePanel />}
        announcements={<AnnouncementsPanel />}
        links={[
          { id: 'teams', href: '/admin/teams', title: 'Teams', description: 'Create teams and review the current registry.', meta: meta(unavailable.teams, dashboard.teams.length, 'team') },
          { id: 'players', href: '/admin/players', title: 'Players', description: 'Manage accounts, WireGuard peers, and access sync.', meta: meta(unavailable.players, dashboard.players.length, 'player') },
          { id: 'challenges', href: '/admin/challenges', title: 'Challenges', description: 'Validate, publish, and deploy challenge packages.', meta: meta(unavailable.challenges, dashboard.challenges.length, 'challenge') },
          { id: 'deployments', href: '/admin/deployments', title: 'Deployments', description: 'Track rollout jobs and reconcile stale work.', meta: meta(unavailable.deployments, dashboard.deployments.length, 'job') },
          { id: 'game', href: '/admin/game', title: 'Game', description: 'Run the match window, scheduler, and checker flow.', meta: meta(unavailable.gameStatus, dashboard.gameStatus.total_ticks, 'persisted tick') },
          { id: 'scoreboard', href: '/admin/scoreboard', title: 'Scoreboard', description: 'Inspect authoritative scores from game-core.', meta: meta(unavailable.scoreboard, dashboard.scoreboard.length, 'row') },
          { id: 'audit', href: '/admin/audit', title: 'Audit', description: 'Review privileged actions across teams and organizers.', meta: 'persistent control trail' },
        ]}
      />
    </OrganizerShell>
  );
}
