import { OrganizerDashboard } from '@/components/admin/organizer-dashboard';
import { OrganizerShell } from '@/components/admin/organizer-shell';
import { loadAdminDashboardData } from '@/lib/admin-dashboard-data';

export const dynamic = 'force-dynamic';

export default async function AdminScoreboardPage() {
  const dashboard = await loadAdminDashboardData();

  return (
    <OrganizerShell
      activePath="/admin/scoreboard"
      overview={dashboard.overview}
      title="Scoreboard"
      description="Authoritative rankings from game-core. Use the Game route for match and scheduler operations."
    >
      <OrganizerDashboard
        attackPage={dashboard.attackPage}
        challenges={dashboard.challenges}
        checkerRunPage={dashboard.checkerRunPage}
        deployments={dashboard.deployments}
        gameStatus={dashboard.gameStatus}
        players={dashboard.players}
        serviceMetrics={dashboard.serviceMetrics}
        schedulerEventPage={dashboard.schedulerEventPage}
        scoreboard={dashboard.scoreboard}
        teams={dashboard.teams}
        initialTab="scoreboard"
      />
    </OrganizerShell>
  );
}
