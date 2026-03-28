import { OrganizerDashboard } from '@/components/admin/organizer-dashboard';
import { OrganizerShell } from '@/components/admin/organizer-shell';
import { loadAdminDashboardData } from '@/lib/admin-dashboard-data';

export const dynamic = 'force-dynamic';

const MAP_ATTACK_LIMIT = 1000;

export default async function AdminAttacksPage() {
  const dashboard = await loadAdminDashboardData({
    attackQuery: { limit: MAP_ATTACK_LIMIT },
  });

  return (
    <OrganizerShell
      activePath="/admin/attacks"
      overview={dashboard.overview}
      title="Attacks"
      description="Dedicated live attack monitoring with the broad accepted-attack feed rendered as a map-first surface."
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
        initialTab="attacks"
      />
    </OrganizerShell>
  );
}
