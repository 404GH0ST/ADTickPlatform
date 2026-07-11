import { OrganizerDashboard } from '@/components/admin/organizer-dashboard';
import { ExportDataPanel } from '@/components/admin/export-data-panel';
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
    >
      <div className="grid gap-4">
        <ExportDataPanel
          scoreboard={dashboard.scoreboard}
          attacks={dashboard.attackPage.items}
        />
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
      </div>
    </OrganizerShell>
  );
}
