import { AnnouncementsPanel } from '@/components/admin/announcements-panel';
import { OrganizerOverview } from '@/components/admin/organizer-overview';
import { OrganizerShell } from '@/components/admin/organizer-shell';
import { TrustedStatePanel } from '@/components/admin/trusted-state-panel';
import { loadAdminDashboardData } from '@/lib/admin-dashboard-data';

export const dynamic = 'force-dynamic';

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
          { id: 'teams', href: '/admin/teams', title: 'Teams', description: 'Create teams and review the current registry.', meta: `${dashboard.teams.length} teams` },
          { id: 'players', href: '/admin/players', title: 'Players', description: 'Manage accounts, WireGuard peers, and access sync.', meta: `${dashboard.players.length} players` },
          { id: 'challenges', href: '/admin/challenges', title: 'Challenges', description: 'Validate, publish, and deploy challenge packages.', meta: `${dashboard.challenges.length} challenges` },
          { id: 'deployments', href: '/admin/deployments', title: 'Deployments', description: 'Track rollout jobs and reconcile stale work.', meta: `${dashboard.deployments.length} jobs` },
          { id: 'game', href: '/admin/game', title: 'Game', description: 'Run the match window, scheduler, and checker flow.', meta: `${dashboard.gameStatus.total_ticks} persisted ticks` },
          { id: 'scoreboard', href: '/admin/scoreboard', title: 'Scoreboard', description: 'Inspect authoritative scores from game-core.', meta: `${dashboard.scoreboard.length} rows` },
          { id: 'audit', href: '/admin/audit', title: 'Audit', description: 'Review privileged actions across teams and organizers.', meta: 'persistent control trail' },
        ]}
      />
    </OrganizerShell>
  );
}
