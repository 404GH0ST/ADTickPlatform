import { OrganizerDashboard } from '@/components/admin/organizer-dashboard';
import { OrganizerShell } from '@/components/admin/organizer-shell';
import { loadAdminDashboardData } from '@/lib/admin-dashboard-data';

export const dynamic = 'force-dynamic';

export default async function AdminChallengesPage() {
  const dashboard = await loadAdminDashboardData();

  return (
    <OrganizerShell
      activePath="/admin/challenges"
      overview={dashboard.overview}
      title="Challenges"
      description="Draft, validate, and deploy challenge packages."
    >
      <OrganizerDashboard
        attackPage={dashboard.attackPage}
        challenges={dashboard.challenges}
        checkerRunPage={dashboard.checkerRunPage}
        deployments={dashboard.deployments}
        gameStatus={dashboard.gameStatus}
        players={dashboard.players}
        schedulerEventPage={dashboard.schedulerEventPage}
        scoreboard={dashboard.scoreboard}
        teams={dashboard.teams}
        initialTab="challenges"
      />
    </OrganizerShell>
  );
}
