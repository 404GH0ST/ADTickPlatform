import { AnnouncementsPanel } from '@/components/admin/announcements-panel';
import { OrganizerOverview } from '@/components/admin/organizer-overview';
import { OrganizerShell } from '@/components/admin/organizer-shell';
import { TrustedStatePanel } from '@/components/admin/trusted-state-panel';
import { loadAdminDashboardData } from '@/lib/admin-dashboard-data';

export const dynamic = 'force-dynamic';

function plural(count: number, noun: string) {
  return `${count} ${noun}${count === 1 ? '' : 's'}`;
}

export default async function AdminPage() {
  const dashboard = await loadAdminDashboardData();

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
          { id: 'teams', href: '/admin/teams', title: 'Teams', description: 'Create teams and review the current registry.', meta: plural(dashboard.teams.length, 'team') },
          { id: 'players', href: '/admin/players', title: 'Players', description: 'Manage accounts, WireGuard peers, and access sync.', meta: plural(dashboard.players.length, 'player') },
          { id: 'challenges', href: '/admin/challenges', title: 'Challenges', description: 'Validate, publish, and deploy challenge packages.', meta: plural(dashboard.challenges.length, 'challenge') },
          { id: 'deployments', href: '/admin/deployments', title: 'Deployments', description: 'Track rollout jobs and reconcile stale work.', meta: plural(dashboard.deployments.length, 'job') },
          { id: 'game', href: '/admin/game', title: 'Game', description: 'Run the match window, scheduler, and checker flow.', meta: plural(dashboard.gameStatus.total_ticks, 'persisted tick') },
          { id: 'scoreboard', href: '/admin/scoreboard', title: 'Scoreboard', description: 'Inspect authoritative scores from game-core.', meta: plural(dashboard.scoreboard.length, 'row') },
          { id: 'audit', href: '/admin/audit', title: 'Audit', description: 'Review privileged actions across teams and organizers.', meta: 'persistent control trail' },
        ]}
      />
    </OrganizerShell>
  );
}
