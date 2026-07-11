import { OrganizerDashboard } from '@/components/admin/organizer-dashboard';
import { BulkImportPanel } from '@/components/admin/bulk-import-panel';
import { OrganizerShell } from '@/components/admin/organizer-shell';
import { loadAdminDashboardData } from '@/lib/admin-dashboard-data';

export const dynamic = 'force-dynamic';

export default async function AdminTeamsPage() {
  const dashboard = await loadAdminDashboardData();

  return (
    <OrganizerShell
      activePath="/admin/teams"
      overview={dashboard.overview}
      title="Teams"
      description="Team creation and registry state."
    >
      <div className="grid gap-4">
        <BulkImportPanel />
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
          initialTab="teams"
        />
      </div>
    </OrganizerShell>
  );
}
